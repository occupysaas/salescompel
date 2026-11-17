package platform

import (
	"sort"
	"strings"
	"testing"
)

func rowsOf(t *testing.T, f readFixture, q string, fields ...string) string {
	t.Helper()
	code, body := f.query(q)
	if code != 200 {
		t.Fatalf("%s: %d %v", q, code, body)
	}
	var out []string
	for _, r := range body.(map[string]any)["records"].([]any) {
		m := r.(map[string]any)
		var parts []string
		for _, fl := range fields {
			s, _ := m[fl].(string)
			parts = append(parts, s)
		}
		out = append(out, strings.Join(parts, ":"))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// D34-08: a share object exists only while its record object is Private or
// Public Read Only.
func TestShareObjectAvailability(t *testing.T) {
	f := newWriteFixture(t)
	code, body := f.query("SELECT Id FROM AccountShare")
	wantREST(t, "AccountShare under Public Read/Write", code, body, 400, "INVALID_TYPE")
	code, body = f.do("GET", "sobjects/ContactShare/describe", "", false)
	wantREST(t, "ContactShare under Controlled by Parent", code, body, 404, "NOT_FOUND")
	setOWD(t, f, "Account", "Read")
	if code, body = f.do("GET", "sobjects/AccountShare/describe", "", false); code != 200 {
		t.Fatalf("AccountShare under Public Read Only: %d %v", code, body)
	}
}

// D34-04..07: who shares, what a share may say, what it grants, and the
// rows a share query returns.
func TestManualShares(t *testing.T) {
	f := newWriteFixture(t)
	owner, peer, other := f.newUser("sh-owner", "Standard User"), f.newUser("sh-peer", "Standard User"), f.newUser("sh-other", "Standard User")
	setOWD(t, f, "Account", "Read")
	acct := owner.created("Account", `{"Name":"Shared"}`)
	share := func(as readFixture, body string) (int, any) {
		return as.do("POST", "sobjects/AccountShare", body, false)
	}
	code, body := share(peer, `{"AccountId":"`+acct+`","UserOrGroupId":"`+other.user+`","AccountAccessLevel":"Edit"}`)
	wantREST(t, "non-owner shares", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = share(owner, `{"AccountId":"`+acct+`","UserOrGroupId":"`+peer.user+`","AccountAccessLevel":"Read"}`)
	wantREST(t, "Read under Public Read Only", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	code, body = share(owner, `{"AccountId":"`+acct+`","UserOrGroupId":"`+peer.user+`","AccountAccessLevel":"All"}`)
	if code != 400 {
		t.Fatalf("All accepted: %d %v", code, body)
	}
	code, body = share(owner, `{"AccountId":"`+acct+`","UserOrGroupId":"`+peer.user+`","AccountAccessLevel":"Edit","RowCause":"Rule"}`)
	wantREST(t, "Rule row cause", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	if code, _ = peer.do("PATCH", "sobjects/Account/"+acct, `{"Name":"Before share"}`, false); code != 403 {
		t.Fatalf("peer edited before the share: %d", code)
	}
	manual := owner.created("AccountShare", `{"AccountId":"`+acct+`","UserOrGroupId":"`+peer.user+`","AccountAccessLevel":"Edit","ContactAccessLevel":"Read"}`)
	if code, body = peer.do("PATCH", "sobjects/Account/"+acct, `{"Name":"After share"}`, false); code != 204 {
		t.Fatalf("Edit share didn't grant edit: %d %v", code, body)
	}
	// Sharing again with the same grantee updates the existing row.
	if again := owner.created("AccountShare", `{"AccountId":"`+acct+`","UserOrGroupId":"`+peer.user+`","AccountAccessLevel":"Edit","ContactAccessLevel":"Edit"}`); again != manual {
		t.Fatalf("re-share made %s, want %s", again, manual)
	}
	parts := []string{owner.user + ":All:Owner", peer.user + ":Edit:Manual"}
	sort.Strings(parts)
	want := strings.Join(parts, ",")
	if got := rowsOf(t, peer, "SELECT UserOrGroupId, AccountAccessLevel, RowCause FROM AccountShare WHERE AccountId = '"+acct+"'", "UserOrGroupId", "AccountAccessLevel", "RowCause"); got != want {
		t.Fatalf("share rows %q, want %q", got, want)
	}
	ownerRow := rowsOf(t, owner, "SELECT Id FROM AccountShare WHERE AccountId = '"+acct+"' AND RowCause = 'Owner'", "Id")
	code, body = owner.do("DELETE", "sobjects/AccountShare/"+ownerRow, "", false)
	wantREST(t, "delete Owner row", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	if code, body = owner.do("DELETE", "sobjects/AccountShare/"+manual, "", false); code != 204 {
		t.Fatalf("delete manual: %d %v", code, body)
	}
	if code, _ = peer.do("PATCH", "sobjects/Account/"+acct, `{"Name":"Unshared"}`, false); code != 403 {
		t.Fatalf("peer still edits after unshare: %d", code)
	}
}

// Shares to groups: a public group, and a role's RoleAndSubordinates group
// reaching the role, its subordinates and, through the hierarchy, managers
// of a grantee.
func TestShareToGroups(t *testing.T) {
	f := newWriteFixture(t)
	setOWD(t, f, "Account", "None")
	owner := f.newUser("grp-owner", "Standard User")
	member, outsider := f.newUser("grp-member", "Standard User"), f.newUser("grp-outsider", "Standard User")
	group := f.created("Group", `{"Name":"Deal desk","DeveloperName":"Deal_desk"}`)
	f.created("GroupMember", `{"GroupId":"`+group+`","UserOrGroupId":"`+member.user+`"}`)
	acct := owner.created("Account", `{"Name":"Group shared"}`)
	owner.created("AccountShare", `{"AccountId":"`+acct+`","UserOrGroupId":"`+group+`","AccountAccessLevel":"Read"}`)
	if got := rowsOf(t, member, "SELECT Name FROM Account", "Name"); got != "Group shared" {
		t.Fatalf("group member sees %q", got)
	}
	if got := rowsOf(t, outsider, "SELECT Name FROM Account", "Name"); got != "" {
		t.Fatalf("outsider sees %q", got)
	}
	// Role and subordinates: share to the VP role's group; the rep below
	// sees it, the CEO above sees it through the hierarchy.
	ceo := f.created("UserRole", `{"Name":"CEO","DeveloperName":"CEO"}`)
	vp := f.created("UserRole", `{"Name":"VP","DeveloperName":"VP","ParentRoleId":"`+ceo+`"}`)
	rep := f.created("UserRole", `{"Name":"Rep","DeveloperName":"Rep","ParentRoleId":"`+vp+`"}`)
	boss, low := f.newUser("grp-boss", "Standard User"), f.newUser("grp-low", "Standard User")
	f.do("PATCH", "sobjects/User/"+boss.user, `{"UserRoleId":"`+ceo+`"}`, false)
	f.do("PATCH", "sobjects/User/"+low.user, `{"UserRoleId":"`+rep+`"}`, false)
	ras := f.one("SELECT Id FROM Group WHERE Type = 'RoleAndSubordinates' AND RelatedId = '"+vp+"'", "Id")
	acct2 := owner.created("Account", `{"Name":"Role shared"}`)
	owner.created("AccountShare", `{"AccountId":"`+acct2+`","UserOrGroupId":"`+ras+`","AccountAccessLevel":"Read"}`)
	for who, fx := range map[string]readFixture{"rep below the role": low, "CEO above the role": boss} {
		if got := rowsOf(t, fx, "SELECT Name FROM Account", "Name"); got != "Role shared" {
			t.Errorf("%s sees %q", who, got)
		}
	}
}
