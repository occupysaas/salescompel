package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func updateAs(t *testing.T, f writeFixture, user, object, id string, data map[string]any) error {
	t.Helper()
	return as(t, f, user, func(s *Store, p Principal) error { return s.recordTx(p).Update(context.Background(), object, id, data) })
}

func deleteAs(t *testing.T, f writeFixture, user, object, id string) error {
	t.Helper()
	return as(t, f, user, func(s *Store, p Principal) error { return s.recordTx(p).Delete(context.Background(), object, id) })
}

func manualShare(t *testing.T, f writeFixture, object, record, grantee, level string) {
	t.Helper()
	if _, e := f.s.pool.Exec(context.Background(), `INSERT INTO sc_sharing.shares(tenant_id,id,object_id,record_id,user_or_group_id,access_level)
		SELECT $1,'00r'||substr(md5(random()::text),1,12)||'AAA',id,$3,$4,$5 FROM sc_catalog.objects WHERE tenant_id=$1 AND api_name=$2`, f.tenant, object, record, grantee, level); e != nil {
		t.Fatal(e)
	}
}

// shr-11: hidden is unknown, readable-not-editable is
// INSUFFICIENT_ACCESS_OR_READONLY, delete needs owner-level access.
func TestSharingUpdateDelete(t *testing.T) {
	f := newWriteFixture(t)
	boss, _, rep, peer := hierarchy(t, f)
	acct := mustCreate(t, f, rep, "Account", map[string]any{"Name": "Rep acct"})
	// Public Read/Write: a peer edits but can't delete.
	if e := updateAs(t, f, peer, "Account", acct, map[string]any{"Name": "Edited"}); e != nil {
		t.Fatal(e)
	}
	wantCode(t, "peer delete under Public Read/Write", deleteAs(t, f, peer, "Account", acct), "INSUFFICIENT_ACCESS_OR_READONLY")
	setOWD(t, f, "Account", "Read")
	wantCode(t, "peer edit under Public Read Only", updateAs(t, f, peer, "Account", acct, map[string]any{"Name": "x"}), "INSUFFICIENT_ACCESS_OR_READONLY")
	setOWD(t, f, "Account", "None")
	wantCode(t, "peer edit under Private", updateAs(t, f, peer, "Account", acct, map[string]any{"Name": "x"}), "NOT_FOUND")
	wantCode(t, "peer delete under Private", deleteAs(t, f, peer, "Account", acct), "NOT_FOUND")
	manualShare(t, f, "Account", acct, peer, "Edit")
	if e := updateAs(t, f, peer, "Account", acct, map[string]any{"Name": "Shared edit"}); e != nil {
		t.Fatalf("Edit share: %v", e)
	}
	wantCode(t, "delete through an Edit share", deleteAs(t, f, peer, "Account", acct), "INSUFFICIENT_ACCESS_OR_READONLY")
	if e := deleteAs(t, f, boss, "Account", acct); e != nil {
		t.Fatalf("manager delete: %v", e)
	}
	if n := f.setupCount("SELECT count(*) FROM sc_sharing.shares WHERE tenant_id=$1 AND record_id=$2", acct); n != 0 {
		t.Fatalf("shares of a deleted record kept: %d", n)
	}
}

// D34-09, D34-10, D34-11: who may transfer, and what a transfer removes and
// moves.
func TestOwnerTransfer(t *testing.T) {
	f := newWriteFixture(t)
	_, mid, rep, peer := hierarchy(t, f)
	acct := mustCreate(t, f, rep, "Account", map[string]any{"Name": "Transfer acct"})
	own := mustCreate(t, f, rep, "Contact", map[string]any{"LastName": "Rep's", "AccountId": acct})
	other := mustCreate(t, f, peer, "Contact", map[string]any{"LastName": "Peer's", "AccountId": acct})
	// Edit access isn't enough to transfer.
	wantCode(t, "peer transfer", updateAs(t, f, peer, "Account", acct, map[string]any{"OwnerId": peer}), "INSUFFICIENT_ACCESS_OR_READONLY")
	manualShare(t, f, "Account", acct, peer, "Read")
	manualShare(t, f, "Contact", own, peer, "Read")
	if e := updateAs(t, f, mid, "Account", acct, map[string]any{"OwnerId": mid}); e != nil {
		t.Fatalf("manager transfer: %v", e)
	}
	if got := ownerOf(t, f, "Account", acct); got != mid {
		t.Fatalf("account owner %s", got)
	}
	if ownerOf(t, f, "Contact", own) != mid || ownerOf(t, f, "Contact", other) != peer {
		t.Fatal("contacts didn't follow the old owner's rule")
	}
	if n := f.setupCount("SELECT count(*) FROM sc_sharing.shares WHERE tenant_id=$1 AND record_id IN ($2,$3)", acct, own); n != 0 {
		t.Fatalf("manual shares survived the transfer: %d", n)
	}
	// Naming the current owner is not a transfer.
	if e := updateAs(t, f, peer, "Account", acct, map[string]any{"OwnerId": mid, "Name": "Same owner"}); e != nil {
		t.Fatalf("same owner: %v", e)
	}
}

// shr-11: a lookup to a hidden record fails as an unknown ID; upsert never
// updates a hidden match (D34-29).
func TestSharingLookupAndUpsert(t *testing.T) {
	f := newWriteFixture(t)
	a, b := sharingUser(t, f, "look-a", "Standard User"), sharingUser(t, f, "look-b", "Standard User")
	setOWD(t, f, "Account", "None")
	setOWD(t, f, "Contact", "None")
	hidden := mustCreate(t, f, a, "Account", map[string]any{"Name": "Hidden", "Compat_Key__c": "k-hidden"})
	_, eHidden := createAs(t, f, b, "Contact", map[string]any{"LastName": "x", "AccountId": hidden})
	_, eMissing := createAs(t, f, b, "Contact", map[string]any{"LastName": "x", "AccountId": "001000000000000AAA"})
	if eHidden == nil || eMissing == nil || eHidden.Error() != eMissing.Error() {
		t.Fatalf("hidden lookup %v, missing lookup %v", eHidden, eMissing)
	}
	wantCode(t, "hidden lookup", eHidden, "INVALID_CROSS_REFERENCE_KEY")
	e := as(t, f, b, func(s *Store, p Principal) error {
		_, _, e := s.recordTx(p).UpsertAccount(context.Background(), "k-hidden", map[string]any{"Name": "Hijack"})
		return e
	})
	wantCode(t, "upsert onto hidden", e, "DUPLICATE_VALUE")
	if res := queryAs(t, f, a, "SELECT Name FROM Account WHERE Id = '"+hidden+"'"); res.Records[0]["Name"] != "Hidden" {
		t.Fatalf("hidden record changed: %v", res.Records)
	}
}

// Hidden records are indistinguishable from unknown IDs over REST,
// Composite and Collections (anti-goal: existence leak).
func TestSharingHiddenEqualsMissing(t *testing.T) {
	f := newWriteFixture(t)
	setOWD(t, f, "Account", "None")
	b := f.newUser("leak-b", "Standard User")
	hidden := f.account // owned by the admin
	unknown := "001" + strings.Repeat("0", 12) + "AAA"
	same := func(what string, call func(id string) (int, any)) {
		t.Helper()
		c1, b1 := call(hidden)
		c2, b2 := call(unknown)
		j1, _ := json.Marshal(b1)
		j2, _ := json.Marshal(b2)
		if c1 != c2 || strings.ReplaceAll(string(j1), hidden, "ID") != strings.ReplaceAll(string(j2), unknown, "ID") {
			t.Fatalf("%s differs: %d %s vs %d %s", what, c1, j1, c2, j2)
		}
	}
	same("GET", func(id string) (int, any) { return b.do("GET", "sobjects/Account/"+id, "", false) })
	same("PATCH", func(id string) (int, any) { return b.do("PATCH", "sobjects/Account/"+id, `{"Name":"x"}`, false) })
	same("DELETE", func(id string) (int, any) { return b.do("DELETE", "sobjects/Account/"+id, "", false) })
	same("Composite", func(id string) (int, any) {
		return b.do("POST", "composite", `{"allOrNone":false,"compositeRequest":[{"method":"GET","url":"/services/data/v65.0/sobjects/Account/`+id+`","referenceId":"r"}]}`, false)
	})
	same("Collections retrieve", func(id string) (int, any) {
		return b.do("GET", "composite/sobjects/Account?ids="+id+"&fields=Name", "", false)
	})
	same("Collections delete", func(id string) (int, any) { return b.do("DELETE", "composite/sobjects?ids="+id, "", false) })
	if code, body := b.query("SELECT COUNT() FROM Account WHERE Id = '" + hidden + "'"); code != 200 || body.(map[string]any)["totalSize"] != float64(0) {
		t.Fatalf("count of a hidden record: %d %v", code, body)
	}
}
