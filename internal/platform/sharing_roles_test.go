package platform

import (
	"context"
	"sort"
	"strings"
	"testing"
)

func wantREST(t *testing.T, what string, code int, body any, status int, errCode string) {
	t.Helper()
	got, _ := failure(body)
	if code != status || got != errCode {
		t.Fatalf("%s: %d %v, want %d %s", what, code, body, status, errCode)
	}
}

// groupsOf lists the closure's groups for user by type and name, sorted.
func groupsOf(t *testing.T, f writeFixture, user string) []string {
	t.Helper()
	rows, e := f.s.pool.Query(context.Background(), `SELECT g.type||':'||g.name FROM sc_sharing.user_groups ug
		JOIN sc_setup.groups g ON g.tenant_id=ug.tenant_id AND g.id=ug.group_id WHERE ug.tenant_id=$1 AND ug.user_id=$2`, f.tenant, user)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func principalsOf(t *testing.T, f writeFixture, user string) map[string]bool {
	t.Helper()
	rows, e := f.s.pool.Query(context.Background(), "SELECT principal_id FROM sc_sharing.user_principals WHERE tenant_id=$1 AND user_id=$2", f.tenant, user)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		rows.Scan(&s)
		out[s] = true
	}
	return out
}

// D34-15: Manage Users writes roles; each role owns three system groups;
// cycles and deleting a role in use are refused.
func TestUserRoleCRUD(t *testing.T) {
	f := newWriteFixture(t)
	ceo := f.created("UserRole", `{"Name":"CEO","DeveloperName":"CEO"}`)
	vp := f.created("UserRole", `{"Name":"VP Sales","DeveloperName":"VP_Sales","ParentRoleId":"`+ceo+`","ContactAccessForAccountOwner":"Edit"}`)
	if n := f.setupCount("SELECT count(*) FROM sc_setup.groups WHERE tenant_id=$1 AND related_id=$2", vp); n != 3 {
		t.Fatalf("role groups %d", n)
	}
	code, body := f.do("POST", "sobjects/UserRole", `{"Name":"Dup","DeveloperName":"ceo"}`, false)
	wantREST(t, "duplicate developer name", code, body, 400, "DUPLICATE_VALUE")
	code, body = f.do("PATCH", "sobjects/UserRole/"+ceo, `{"ParentRoleId":"`+vp+`"}`, false)
	wantREST(t, "cycle", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	std := f.newUser("role-std", "Standard User")
	code, body = std.do("POST", "sobjects/UserRole", `{"Name":"Nope","DeveloperName":"Nope"}`, false)
	wantREST(t, "standard user create", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	if code, _ := std.query("SELECT Id, Name, ParentRole.Name FROM UserRole"); code != 200 {
		t.Fatalf("standard user can't read roles: %d", code)
	}
	if code, body = f.do("PATCH", "sobjects/User/"+std.user, `{"UserRoleId":"`+vp+`"}`, false); code != 204 {
		t.Fatal(code, body)
	}
	code, body = f.do("DELETE", "sobjects/UserRole/"+vp, "", false)
	wantREST(t, "role with users", code, body, 400, "DELETE_FAILED")
	code, body = f.do("DELETE", "sobjects/UserRole/"+ceo, "", false)
	wantREST(t, "role with children", code, body, 400, "DELETE_FAILED")
	if code, body = f.do("PATCH", "sobjects/User/"+std.user, `{"UserRoleId":null}`, false); code != 204 {
		t.Fatal(code, body)
	}
	if code, body = f.do("DELETE", "sobjects/UserRole/"+vp, "", false); code != 204 {
		t.Fatal(code, body)
	}
	if n := f.setupCount("SELECT count(*) FROM sc_setup.groups WHERE tenant_id=$1 AND related_id=$2", vp); n != 0 {
		t.Fatalf("role groups left %d", n)
	}
}

// D34-12, D34-13: only Regular groups are writable; members nest; cycles
// are refused.
func TestGroupsAndMembers(t *testing.T) {
	f := newWriteFixture(t)
	g1 := f.created("Group", `{"Name":"Outer","DeveloperName":"Outer"}`)
	g2 := f.created("Group", `{"Name":"Inner","DeveloperName":"Inner","Type":"Regular"}`)
	code, body := f.do("POST", "sobjects/Group", `{"Name":"Role","Type":"Role"}`, false)
	wantREST(t, "system type", code, body, 400, "INVALID_OPERATION")
	org := f.one("SELECT Id FROM Group WHERE Type = 'Organization'", "Id")
	code, body = f.do("PATCH", "sobjects/Group/"+org, `{"Name":"Renamed"}`, false)
	wantREST(t, "organization group", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	f.created("GroupMember", `{"GroupId":"`+g1+`","UserOrGroupId":"`+g2+`"}`)
	std := f.newUser("member-std", "Standard User")
	f.created("GroupMember", `{"GroupId":"`+g2+`","UserOrGroupId":"`+std.user+`"}`)
	code, body = f.do("POST", "sobjects/GroupMember", `{"GroupId":"`+g2+`","UserOrGroupId":"`+g1+`"}`, false)
	wantREST(t, "cycle", code, body, 400, "FIELD_INTEGRITY_EXCEPTION")
	code, body = f.do("POST", "sobjects/GroupMember", `{"GroupId":"`+g2+`","UserOrGroupId":"`+std.user+`"}`, false)
	wantREST(t, "duplicate", code, body, 400, "DUPLICATE_VALUE")
	code, body = f.do("POST", "sobjects/GroupMember", `{"GroupId":"`+org+`","UserOrGroupId":"`+std.user+`"}`, false)
	wantREST(t, "member of system group", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	if got := strings.Join(groupsOf(t, f, std.user), ","); got != "Organization:All Internal Users,Regular:Inner,Regular:Outer" {
		t.Fatalf("closure %s", got)
	}
}

// shr-05, D34-25: role groups, the hierarchy's principals, and
// deactivation.
func TestClosureRebuild(t *testing.T) {
	f := newWriteFixture(t)
	ceo := f.created("UserRole", `{"Name":"CEO","DeveloperName":"CEO"}`)
	vp := f.created("UserRole", `{"Name":"VP","DeveloperName":"VP","ParentRoleId":"`+ceo+`"}`)
	rep := f.created("UserRole", `{"Name":"Rep","DeveloperName":"Rep","ParentRoleId":"`+vp+`"}`)
	boss, mid, low := f.newUser("boss", "Standard User"), f.newUser("mid", "Standard User"), f.newUser("low", "Standard User")
	for user, role := range map[string]string{boss.user: ceo, mid.user: vp, low.user: rep} {
		if code, body := f.do("PATCH", "sobjects/User/"+user, `{"UserRoleId":"`+role+`"}`, false); code != 204 {
			t.Fatal(code, body)
		}
	}
	want := "Organization:All Internal Users,Role:Rep,RoleAndSubordinates:CEO,RoleAndSubordinates:Rep,RoleAndSubordinates:VP,RoleAndSubordinatesInternal:CEO,RoleAndSubordinatesInternal:Rep,RoleAndSubordinatesInternal:VP"
	if got := strings.Join(groupsOf(t, f, low.user), ","); got != want {
		t.Fatalf("rep closure %s", got)
	}
	bp, mp := principalsOf(t, f, boss.user), principalsOf(t, f, mid.user)
	if !bp[mid.user] || !bp[low.user] || !mp[low.user] || mp[boss.user] || principalsOf(t, f, low.user)[mid.user] {
		t.Fatalf("hierarchy principals boss=%v mid=%v", bp, mp)
	}
	ctx := context.Background()
	if _, e := f.s.pool.Exec(ctx, `INSERT INTO sc_sharing.shares(tenant_id,id,object_id,record_id,user_or_group_id,access_level)
		SELECT $1,'00r000000000001AAA',id,$2,$3,'Read' FROM sc_catalog.objects WHERE tenant_id=$1 AND api_name='Account'`, f.tenant, f.account, low.user); e != nil {
		t.Fatal(e)
	}
	if code, body := f.do("PATCH", "sobjects/User/"+low.user, `{"IsActive":false}`, false); code != 204 {
		t.Fatal(code, body)
	}
	if g := groupsOf(t, f, low.user); len(g) != 0 {
		t.Fatalf("inactive user still in %v", g)
	}
	if n := f.setupCount("SELECT count(*) FROM sc_sharing.shares WHERE tenant_id=$1 AND user_or_group_id=$2", low.user); n != 0 {
		t.Fatalf("shares to inactive user kept: %d", n)
	}
	// Managers keep the inactive subordinate as a principal: their records
	// stay reachable through the hierarchy.
	if !principalsOf(t, f, mid.user)[low.user] {
		t.Fatal("manager lost the inactive subordinate")
	}
	if n := f.setupCount("SELECT count(*) FROM sc_setup.users WHERE tenant_id=$1 AND id=$2 AND user_role_id=$3", low.user, rep); n != 1 {
		t.Fatal("deactivation dropped the role")
	}
}
