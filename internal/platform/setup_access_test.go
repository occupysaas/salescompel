package platform

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// setupAccessFixture provisions an admin and a restricted token in one tenant
// and creates a CLI user, whose profile assignment is a setup row.
func setupAccessFixture(t *testing.T) (*Store, Principal, map[string]string, string) {
	t.Helper()
	s := testStore(t)
	a, _ := principals(t, s)
	restricted := a
	restricted.Role, restricted.User = "restricted", "reader"
	if e := s.Provision(context.Background(), "restricted-"+a.Tenant, restricted); e != nil {
		t.Fatal(e)
	}
	user, e := s.CreateUser(context.Background(), a.Tenant, "setup."+a.Tenant+"@example.com", "long-password", "restricted")
	if e != nil {
		t.Fatal(e)
	}
	return s, a, map[string]string{"admin": "token-" + a.Tenant, "restricted": "restricted-" + a.Tenant}, user
}

func setupRequest(t *testing.T, s *Store, method, path, token string) (int, string) {
	t.Helper()
	r := httptest.NewRequest(method, "/services/data/v65.0/"+path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	NewHandler(s, "").ServeHTTP(w, r)
	return w.Code, w.Body.String()
}

func setupQuery(t *testing.T, s *Store, q, token string) (int, string) {
	t.Helper()
	return setupRequest(t, s, "GET", "query?q="+url.QueryEscape(q), token)
}

func assignmentCount(t *testing.T, s *Store, tenant, user string) int {
	t.Helper()
	ctx := context.Background()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant)
	var n int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM sc_setup.permission_set_assignments WHERE tenant_id=$1 AND assignee_id=$2", tenant, user).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

// A profile's system-maintained assignment can't be deleted by anyone
// (D33-21), restricted users can't read the setup objects other than User,
// and an admin's generic create of a PermissionSet goes through the setup
// write rules.
func TestSetupObjectAccess(t *testing.T) {
	s, a, tokens, user := setupAccessFixture(t)
	code, body := setupQuery(t, s, "SELECT Id FROM PermissionSetAssignment WHERE AssigneeId = '"+user+"'", tokens["admin"])
	if code != 200 || !strings.Contains(body, `"totalSize":1`) {
		t.Fatalf("admin assignment query: %d %s", code, body)
	}
	psa := body[strings.Index(body, `"Id":"`)+6:]
	psa = psa[:18]
	for _, who := range []string{"restricted", "admin"} {
		code, body = setupRequest(t, s, "DELETE", "sobjects/PermissionSetAssignment/"+psa, tokens[who])
		if code == 204 || code == 200 {
			t.Fatalf("%s deleted a setup row: %d %s", who, code, body)
		}
		if who == "admin" && (code != 403 || !strings.Contains(body, "INSUFFICIENT_ACCESS_OR_READONLY")) {
			t.Fatalf("admin delete error: %d %s", code, body)
		}
	}
	if n := assignmentCount(t, s, a.Tenant, user); n != 1 {
		t.Fatalf("assignment rows after refused deletes: %d", n)
	}
	for _, q := range []string{"SELECT COUNT() FROM PermissionSetAssignment", "SELECT Id FROM PermissionSet", "SELECT Id, (SELECT Id FROM PermissionSetAssignments) FROM User", "SELECT Profile.Name FROM User"} {
		if code, body = setupQuery(t, s, q, tokens["restricted"]); code != 400 || !strings.Contains(body, "INVALID_TYPE") {
			t.Fatalf("restricted %q: %d %s", q, code, body)
		}
	}
	if code, body = setupQuery(t, s, "SELECT Id, Name, Label, PermissionsManageUsers FROM PermissionSet", tokens["admin"]); code != 200 || !strings.Contains(body, `"totalSize":4`) {
		t.Fatalf("admin PermissionSet query: %d %s", code, body)
	}
	if code, body = setupQuery(t, s, "SELECT Id, Username FROM User", tokens["restricted"]); code != 200 || !strings.Contains(body, user) {
		t.Fatalf("restricted User query: %d %s", code, body)
	}
	if _, e := s.Create(context.Background(), a, "PermissionSet", map[string]any{"Name": "X", "Label": "X"}); e != nil {
		t.Fatalf("admin PermissionSet create: %v", e)
	}
}

// A token whose tenant has no setup data to load is an invalid session.
func TestTokenWithoutSetupDataIsInvalidSession(t *testing.T) {
	s := testStore(t)
	a, _ := principals(t, s)
	ctx := context.Background()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", a.Tenant)
	if _, e = tx.Exec(ctx, "DELETE FROM sc_setup.revisions WHERE tenant_id=$1", a.Tenant); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	p, e := s.Authenticate(ctx, "token-"+a.Tenant)
	if c, _ := apiCode(e); c != "INVALID_SESSION_ID" || p.Tenant != "" {
		t.Fatalf("got %+v %v", p, e)
	}
}
