package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func oauthFixture(t *testing.T) (*Store, string, string) {
	t.Helper()
	s := testStore(t)
	tenant := fmt.Sprintf("oauth-%d", time.Now().UnixNano())
	t.Cleanup(func() { s.pool.Exec(context.Background(), "DELETE FROM tenants WHERE id=$1", tenant) })
	return s, tenant, "Alice" + tenant + "@example.com"
}
func TestCreateUserRules(t *testing.T) {
	s, tenant, name := oauthFixture(t)
	ctx := context.Background()
	for _, v := range []struct{ n, p, r string }{{name, "12345678901", "admin"}, {name, "éééééé", "admin"}, {name, strings.Repeat("x", 1025), "admin"}, {"missing", "long-password", "admin"}, {"a@@b", "long-password", "admin"}, {"é@b", "long-password", "admin"}, {strings.Repeat("a", 79) + "@b", "long-password", "admin"}, {name, "long-password", "other"}} {
		if _, e := s.CreateUser(ctx, tenant, v.n, v.p, v.r); e == nil {
			t.Fatal("accepted invalid user")
		}
	}
	id, e := s.CreateUser(ctx, tenant, name, "long-password", "admin")
	if e != nil || !strings.HasPrefix(id, "005") {
		t.Fatal(id, e)
	}
	if _, e = s.CreateUser(ctx, tenant+"2", strings.ToLower(name), "long-password", "admin"); e == nil {
		t.Fatal("duplicate")
	}
	var h string
	if e = s.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", id).Scan(&h); e != nil || !strings.HasPrefix(h, "$argon2id$") {
		t.Fatal(h, e)
	}
	var raw string
	s.pool.QueryRow(ctx, "SELECT row_to_json(u)::text FROM users u WHERE id=$1", id).Scan(&raw)
	if strings.Contains(raw, "long-password") {
		t.Fatal("stored password")
	}
}
func TestVerifyLogin(t *testing.T) {
	s, tenant, name := oauthFixture(t)
	ctx := context.Background()
	s.CreateUser(ctx, tenant, name, "long-password", "admin")
	u, e := s.VerifyLogin(ctx, strings.ToUpper(name), "long-password", time.Now())
	if e != nil || u.Username != name {
		t.Fatal(u, e)
	}
	for _, n := range []string{name, "unknown@example.com"} {
		if _, e = s.VerifyLogin(ctx, n, "wrong", time.Now()); !errors.Is(e, errBadCredentials) {
			t.Fatal(e)
		}
	}
	s.DeactivateUser(ctx, name)
	if _, e = s.VerifyLogin(ctx, name, "long-password", time.Now()); !errors.Is(e, errBadCredentials) {
		t.Fatal(e)
	}
}
func TestLoginLockout(t *testing.T) {
	s, tenant, name := oauthFixture(t)
	ctx := context.Background()
	s.CreateUser(ctx, tenant, name, "long-password", "admin")
	now := time.Now()
	bad := func(n int) {
		for range n {
			if _, e := s.VerifyLogin(ctx, name, "bad", now); !errors.Is(e, errBadCredentials) {
				t.Fatal(e)
			}
		}
	}
	bad(9)
	if _, e := s.VerifyLogin(ctx, name, "long-password", now); e != nil {
		t.Fatal(e)
	}
	bad(10)
	if _, e := s.VerifyLogin(ctx, name, "long-password", now.Add(15*time.Minute-time.Second)); e == nil {
		t.Fatal("unlocked early")
	}
	if _, e := s.VerifyLogin(ctx, name, "long-password", now.Add(15*time.Minute)); e != nil {
		t.Fatal(e)
	}
	bad(10)
	if e := s.SetPassword(ctx, name, "new-password"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.VerifyLogin(ctx, name, "new-password", now); e != nil {
		t.Fatal(e)
	}
}
func TestCreateAppRules(t *testing.T) {
	s, tenant, name := oauthFixture(t)
	ctx := context.Background()

	if _, e := s.CreateUser(ctx, tenant, name, "long-password", "admin"); e != nil {
		t.Fatal(e)
	}
	other := name + "other"
	if _, e := s.CreateUser(ctx, tenant+"-other", other, "long-password", "admin"); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.pool.Exec(ctx, "DELETE FROM tenants WHERE id=$1", tenant+"-other") })
	for _, input := range []AppInput{{RunAs: other}, {JWTUsers: []string{other}}} {
		input.Tenant = tenant
		input.Name = "foreign user"
		if _, _, e := s.CreateApp(ctx, input, time.Now()); e == nil {
			t.Fatal("accepted user from another tenant")
		}
	}
	for _, uri := range []string{"http://example.com/cb", "myapp://cb"} {
		if _, _, e := s.CreateApp(ctx, AppInput{Tenant: tenant, Name: "app", RedirectURIs: []string{uri}}, time.Now()); e == nil {
			t.Fatal(uri)
		}
	}
	for _, input := range []AppInput{{Grants: []string{"bad"}}, {Scopes: []string{"openid"}}, {Scopes: []string{"bad"}}, {Public: true, Grants: []string{"client_credentials"}}, {Grants: []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"}}, {RunAs: "other@example.com"}, {JWTUsers: []string{"other@example.com"}}} {
		input.Tenant = tenant
		input.Name = "app"
		if _, _, e := s.CreateApp(ctx, input, time.Now()); e == nil {
			t.Fatal(input)
		}
	}
	for _, uri := range []string{"http://localhost:9/cb", "http://127.0.0.1:9/cb", "http://[::1]:9/cb", "https://example.com/cb"} {
		id, secret, e := s.CreateApp(ctx, AppInput{Tenant: tenant, Name: "app", RedirectURIs: []string{uri}}, time.Now())
		if e != nil || len(secret) != 64 || id == PlatformCLIClientID {
			t.Fatal(id, secret, e)
		}
		a, e := s.LookupApp(ctx, id)
		if e != nil || a.SecretHash != digest(secret) {
			t.Fatal(a, e)
		}
		next, e := s.RotateAppSecret(ctx, id)
		a, _ = s.LookupApp(ctx, id)
		if e != nil || next == secret || a.SecretHash != digest(next) {
			t.Fatal(e)
		}
	}
	_, secret, e := s.CreateApp(ctx, AppInput{Tenant: tenant, Name: "public", Public: true}, time.Now())
	if e != nil || secret != "" {
		t.Fatal(e)
	}
}
func TestLookupPlatformCLI(t *testing.T) {
	s := testStore(t)
	a, e := s.LookupApp(context.Background(), PlatformCLIClientID)
	if e != nil || !a.Public || !a.Active || strings.Join(a.RedirectURIs, ",") != "http://localhost:1717/OauthRedirect" || strings.Join(a.Grants, " ") != "authorization_code refresh_token" || strings.Join(a.Scopes, " ") != "api web refresh_token" {
		t.Fatal(a, e)
	}
}

func TestPasswordWorkBoundAndCancellation(t *testing.T) {
	s := testStore(t)
	if cap(s.passwordWork) != 4 {
		t.Fatal("password work must be bounded to four")
	}
	for range 4 {
		s.passwordWork <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.passwordHash(ctx, "long-password"); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := s.VerifyLogin(ctx, "unknown@example.com", "long-password", time.Now()); !errors.Is(e, errBadCredentials) {
		t.Fatal(e)
	}
	for range 4 {
		<-s.passwordWork
	}
}

func TestAdminStoreRevokesTokens(t *testing.T) {
	for _, action := range []string{"set-password", "deactivate", "disable"} {
		t.Run(action, func(t *testing.T) {
			s, tenant, name := oauthFixture(t)
			ctx := context.Background()
			id, e := s.CreateUser(ctx, tenant, name, "long-password", "admin")
			if e != nil {
				t.Fatal(e)
			}
			app, _, e := s.CreateApp(ctx, AppInput{Tenant: tenant, Name: "revocation", RunAs: name, JWTUsers: []string{name}}, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.JWTUserAllowed(ctx, app, name); e != nil {
				t.Fatal(e)
			}
			for _, q := range []string{"INSERT INTO oauth_codes(code_hash,client_id,user_id,redirect_uri,scope,expires_at) VALUES('code',$1,$2,'https://example.com','api',now()+interval '1 minute')", "INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,issued_at,code_hash) VALUES('refresh',$1,$2,'api',now(),'code')", "INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,issued_at,expires_at,refresh_token_hash,code_hash) VALUES('access',$1,$2,'api',now(),now()+interval '1 minute','refresh','code')"} {
				if _, e = s.pool.Exec(ctx, q, app, id); e != nil {
					t.Fatal(e)
				}
			}
			switch action {
			case "set-password":
				e = s.SetPassword(ctx, name, "new-password")
			case "deactivate":
				e = s.DeactivateUser(ctx, name)
			case "disable":
				e = s.DisableApp(ctx, app)
			}
			if e != nil {
				t.Fatal(e)
			}
			for _, table := range []string{"oauth_access_tokens", "oauth_refresh_tokens", "oauth_codes"} {
				var count int
				if e = s.pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE user_id=$1", id).Scan(&count); e != nil || count != 0 {
					t.Fatal(table, count, e)
				}
			}
		})
	}
}
