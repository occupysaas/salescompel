package platform

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"salescompel/internal/metadata"
	soqlpkg "salescompel/internal/soql"
)

// setOWD stores an internal org-wide default, as `salescompel sharing
// set-default` does.
func setOWD(t *testing.T, f writeFixture, object, access string) {
	t.Helper()
	if _, e := f.s.pool.Exec(context.Background(), `INSERT INTO sc_sharing.object_settings(tenant_id,object_id,internal_access)
		SELECT tenant_id,id,$3 FROM sc_catalog.objects WHERE tenant_id=$1 AND api_name=$2
		ON CONFLICT(tenant_id,object_id) DO UPDATE SET internal_access=excluded.internal_access`, f.tenant, object, access); e != nil {
		t.Fatal(e)
	}
}

func queryAs(t *testing.T, f writeFixture, user, q string) soqlpkg.Result {
	t.Helper()
	var res soqlpkg.Result
	if e := as(t, f, user, func(s *Store, p Principal) error {
		var e error
		res, e = s.recordTx(p).Query(context.Background(), q, soqlpkg.Options{})
		return e
	}); e != nil {
		t.Fatalf("%s: %v", q, e)
	}
	return res
}

// names lists a field's values across records, sorted.
func names(res soqlpkg.Result, field string) string {
	var out []string
	for _, r := range res.Records {
		out = append(out, fmt.Sprint(r[field]))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func mustCreate(t *testing.T, f writeFixture, user, object string, data map[string]any) string {
	t.Helper()
	id, e := createAs(t, f, user, object, data)
	if e != nil {
		t.Fatalf("create %s as %s: %v", object, user, e)
	}
	return id
}

// hierarchy creates CEO > VP > Rep roles and a user in each, plus a peer
// rep with no role relation to the first, and returns their IDs.
func hierarchy(t *testing.T, f writeFixture) (boss, mid, rep, peer string) {
	t.Helper()
	ceo := f.created("UserRole", `{"Name":"CEO","DeveloperName":"CEO"}`)
	vp := f.created("UserRole", `{"Name":"VP","DeveloperName":"VP","ParentRoleId":"`+ceo+`"}`)
	r := f.created("UserRole", `{"Name":"Rep","DeveloperName":"Rep","ParentRoleId":"`+vp+`"}`)
	boss, mid, rep, peer = sharingUser(t, f, "boss", "Standard User"), sharingUser(t, f, "mid", "Standard User"), sharingUser(t, f, "rep", "Standard User"), sharingUser(t, f, "peer", "Standard User")
	for user, role := range map[string]string{boss: ceo, mid: vp, rep: r, peer: r} {
		if code, body := f.do("PATCH", "sobjects/User/"+user, `{"UserRoleId":"`+role+`"}`, false); code != 204 {
			t.Fatal(code, body)
		}
	}
	return
}

// shr-10: the root query, counts and the hierarchy under a Private OWD.
func TestSharingSOQLRoot(t *testing.T) {
	f := newWriteFixture(t)
	boss, mid, rep, peer := hierarchy(t, f)
	setOWD(t, f, "Account", "None")
	mustCreate(t, f, rep, "Account", map[string]any{"Name": "Rep acct"})
	mustCreate(t, f, peer, "Account", map[string]any{"Name": "Peer acct"})
	mustCreate(t, f, mid, "Account", map[string]any{"Name": "Mid acct"})
	for user, want := range map[string]string{
		rep:    "Rep acct",
		peer:   "Peer acct",
		mid:    "Mid acct,Peer acct,Rep acct",
		boss:   "Mid acct,Peer acct,Rep acct",
		f.user: "Mid acct,Peer acct,Rep acct,Write fixture",
	} {
		if got := names(queryAs(t, f, user, "SELECT Name FROM Account"), "Name"); got != want {
			t.Errorf("%s sees %q, want %q", user, got, want)
		}
	}
	if got := queryAs(t, f, rep, "SELECT COUNT() FROM Account").TotalSize; got != 1 {
		t.Errorf("rep COUNT() = %d", got)
	}
	agg := queryAs(t, f, peer, "SELECT COUNT(Id) n FROM Account")
	if fmt.Sprint(agg.Records[0]["n"]) != "1" {
		t.Errorf("peer COUNT(Id) = %v", agg.Records)
	}
	setOWD(t, f, "Account", "Read")
	if got := names(queryAs(t, f, rep, "SELECT Name FROM Account"), "Name"); got != "Mid acct,Peer acct,Rep acct,Write fixture" {
		t.Errorf("public read only: rep sees %q", got)
	}
}

// shr-10: child subqueries and semi-joins are filtered like the root.
func TestSharingSOQLChildAndSemiJoin(t *testing.T) {
	f := newWriteFixture(t)
	a, b := sharingUser(t, f, "child-a", "Standard User"), sharingUser(t, f, "child-b", "Standard User")
	setOWD(t, f, "Contact", "None")
	acct := mustCreate(t, f, a, "Account", map[string]any{"Name": "Shared parent"})
	mustCreate(t, f, a, "Contact", map[string]any{"LastName": "A-contact", "AccountId": acct})
	mustCreate(t, f, b, "Contact", map[string]any{"LastName": "B-contact", "AccountId": acct})
	q := "SELECT Name, (SELECT LastName FROM Contacts ORDER BY LastName) FROM Account WHERE Id = '" + acct + "'"
	children := func(user string) string {
		res := queryAs(t, f, user, q)
		sub, _ := res.Records[0]["Contacts"].(map[string]any)
		list, _ := sub["records"].([]map[string]any)
		var out []string
		for _, r := range list {
			out = append(out, fmt.Sprint(r["LastName"]))
		}
		if raw, ok := sub["records"].([]any); ok {
			for _, r := range raw {
				out = append(out, fmt.Sprint(r.(map[string]any)["LastName"]))
			}
		}
		return strings.Join(out, ",")
	}
	// b isn't the account owner: only their own contact. a owns the
	// account: account-owner child access shows both (D34-22).
	if got := children(b); got != "B-contact" {
		t.Errorf("b children %q", got)
	}
	if got := children(a); got != "A-contact,B-contact" {
		t.Errorf("a children %q", got)
	}
	semi := "SELECT Name FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName = 'A-contact')"
	if got := queryAs(t, f, b, semi).TotalSize; got != 0 {
		t.Errorf("b semi-join %d", got)
	}
	if got := queryAs(t, f, a, semi).TotalSize; got != 1 {
		t.Errorf("a semi-join %d", got)
	}
	anti := "SELECT Name FROM Account WHERE Id = '" + acct + "' AND Id NOT IN (SELECT AccountId FROM Contact WHERE LastName = 'A-contact')"
	if got := queryAs(t, f, b, anti).TotalSize; got != 1 {
		t.Errorf("b anti-join %d", got)
	}
}

// D34-26: a lookup parent the user can't read is null in results and
// filters.
func TestSharingHiddenParentIsNull(t *testing.T) {
	f := newWriteFixture(t)
	ctx := context.Background()
	if _, e := metadata.CreateObject(ctx, f.s.pool, f.tenant, metadata.ObjectInput{Name: "Note__c", Label: "Note", PluralLabel: "Notes"}); e != nil {
		t.Fatal(e)
	}
	if _, e := metadata.CreateField(ctx, f.s.pool, f.tenant, metadata.FieldInput{Object: "Note__c", Name: "Account__c", Label: "Account", Type: "Lookup", ReferenceTo: "Account"}); e != nil {
		t.Fatal(e)
	}
	a, b := sharingUser(t, f, "parent-a", "Standard User"), sharingUser(t, f, "parent-b", "Standard User")
	set := f.created("PermissionSet", `{"Name":"Notes","Label":"Notes"}`)
	f.created("ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"Note__c","PermissionsRead":true,"PermissionsCreate":true,"PermissionsEdit":true}`)
	f.created("FieldPermissions", `{"ParentId":"`+set+`","SobjectType":"Note__c","Field":"Note__c.Account__c","PermissionsRead":true,"PermissionsEdit":true}`)
	f.created("PermissionSetAssignment", `{"AssigneeId":"`+b+`","PermissionSetId":"`+set+`"}`)
	acct := mustCreate(t, f, a, "Account", map[string]any{"Name": "Hidden parent"})
	mustCreate(t, f, f.user, "Note__c", map[string]any{"Name": "B note", "Account__c": acct, "OwnerId": b})
	setOWD(t, f, "Account", "None")
	res := queryAs(t, f, b, "SELECT Name, Account__r.Name FROM Note__c")
	if res.TotalSize != 1 || res.Records[0]["Account__r"] != nil {
		t.Fatalf("hidden parent leaked: %v", res.Records)
	}
	if got := queryAs(t, f, b, "SELECT Name FROM Note__c WHERE Account__r.Name = 'Hidden parent'").TotalSize; got != 0 {
		t.Fatalf("filter on hidden parent matched %d", got)
	}
}

// A record that loses visibility between query-locator pages is dropped
// from the later page (fail closed).
func TestSharingLocatorRefilter(t *testing.T) {
	f := newWriteFixture(t)
	a, b := sharingUser(t, f, "loc-a", "Standard User"), sharingUser(t, f, "loc-b", "Standard User")
	setOWD(t, f, "Account", "None")
	ctx := context.Background()
	if _, e := f.s.pool.Exec(ctx, `INSERT INTO accounts(tenant_id,id,name,owner_id)
		SELECT $1,'001'||lpad(to_hex(g),12,'0')||'AAA','Loc '||lpad(g::text,4,'0'),$2 FROM generate_series(1,250) g`, f.tenant, b); e != nil {
		t.Fatal(e)
	}
	var first soqlpkg.Result
	if e := as(t, f, b, func(s *Store, p Principal) error {
		var e error
		first, e = s.recordTx(p).Query(ctx, "SELECT Name FROM Account ORDER BY Name", soqlpkg.Options{BatchSize: 200})
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if first.TotalSize != 250 || len(first.Records) != 200 || first.NextLocator == "" {
		t.Fatalf("first page %d/%d %q", len(first.Records), first.TotalSize, first.NextLocator)
	}
	if _, e := f.s.pool.Exec(ctx, "UPDATE accounts SET owner_id=$2 WHERE tenant_id=$1 AND name>'Loc 0240'", f.tenant, a); e != nil {
		t.Fatal(e)
	}
	var next soqlpkg.Result
	if e := as(t, f, b, func(s *Store, p Principal) error {
		var e error
		next, e = s.recordTx(p).QueryMore(ctx, first.NextLocator, 200)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if len(next.Records) != 40 {
		t.Fatalf("second page kept %d records", len(next.Records))
	}
}
