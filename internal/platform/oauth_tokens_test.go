package platform

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func tokenFixture(t *testing.T) (*Store, TokenGrant, time.Time) {
	t.Helper()
	s, tenant, name := oauthFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	if _, e := s.CreateUser(ctx, tenant, name, "long-password", "admin"); e != nil {
		t.Fatal(e)
	}
	u, e := s.VerifyLogin(ctx, name, "long-password", now)
	if e != nil {
		t.Fatal(e)
	}
	app, _, e := s.CreateApp(ctx, AppInput{Tenant: tenant, Name: "tokens", Scopes: []string{"api", "id", "web", "refresh_token"}}, now)
	if e != nil {
		t.Fatal(e)
	}
	return s, TokenGrant{ClientID: app, User: u, Scope: "api refresh_token", Refresh: true}, now
}
func issueTest(t *testing.T, s *Store, g TokenGrant, now time.Time) IssuedTokens {
	t.Helper()
	v, e := s.IssueTokens(context.Background(), g, now, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestAccessTokenLifetime(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	ctx := context.Background()
	for _, at := range []time.Time{n, n.Add(time.Hour - time.Second)} {
		if _, e := s.AuthenticateAt(ctx, v.Access, at); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := s.AuthenticateAt(ctx, v.Access, n.Add(time.Hour)); e == nil {
		t.Fatal("expired token accepted")
	} else {
		var api *APIError
		if !errors.As(e, &api) || api.Status != 401 || api.Code != "INVALID_SESSION_ID" {
			t.Fatal(e)
		}
	}
	var count int
	if e := s.pool.QueryRow(ctx, "SELECT count(*) FROM oauth_access_tokens WHERE token_hash=$1", v.Access).Scan(&count); e != nil || count != 0 {
		t.Fatal(count, e)
	}
}
func TestPrincipalFromOAuthToken(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	ctx := context.Background()
	p, e := s.AuthenticateAt(ctx, v.Access, n)
	if e != nil || p.Tenant != g.User.Tenant || p.User != g.User.ID || p.Username != g.User.Username || p.Scope != g.Scope || p.Role != "admin" {
		t.Fatal(p, e)
	}
	s.pool.Exec(ctx, "UPDATE users SET role='restricted' WHERE id=$1", g.User.ID)
	p, e = s.AuthenticateAt(ctx, v.Access, n)
	if e != nil || p.Role != "restricted" {
		t.Fatal(p, e)
	}
}
func testInvalidation(t *testing.T, action string) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	ctx := context.Background()
	var e error
	switch action {
	case "user":
		e = s.DeactivateUser(ctx, g.User.Username)
	case "app":
		e = s.DisableApp(ctx, g.ClientID)
	case "password":
		e = s.SetPassword(ctx, g.User.Username, "new-password")
	}
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.AuthenticateAt(ctx, v.Access, n); e == nil {
		t.Fatal("access survived")
	}
	if _, e = s.RefreshAccess(ctx, v.Refresh, g.ClientID, n, time.Hour); e == nil {
		t.Fatal("refresh survived")
	}
	if _, e = s.IssueTokens(ctx, g, n, time.Hour); e == nil {
		t.Fatal("stale grant accepted")
	}
}
func TestDeactivatedUserLosesAccess(t *testing.T) { testInvalidation(t, "user") }
func TestDisabledAppLosesAccess(t *testing.T)     { testInvalidation(t, "app") }
func TestSetPasswordRevokes(t *testing.T)         { testInvalidation(t, "password") }
func codeTest(t *testing.T, s *Store, g TokenGrant, n time.Time, challenge string) string {
	t.Helper()
	c, e := s.IssueCode(context.Background(), CodeInput{ClientID: g.ClientID, UserID: g.User.ID, RedirectURI: "https://example.com/cb", Scope: g.Scope, Challenge: challenge}, n)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestRedeemCode(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	verifier := strings.Repeat("a", 43)
	challenge := strings.Repeat("x", 43)
	_ = verifier
	for _, tt := range []struct {
		name, client, redirect, verifier, challenge string
		after                                       time.Duration
		want                                        string
	}{{"client", "wrong", "https://example.com/cb", "", "", 0, "invalid_grant"}, {"redirect", "", "https://wrong.example/cb", "", "", 0, "redirect_uri_mismatch"}, {"expired", "", "https://example.com/cb", "", "", 10 * time.Minute, "invalid_grant"}, {"unexpected verifier", "", "https://example.com/cb", verifier, "", 0, "invalid_grant"}, {"missing verifier", "", "https://example.com/cb", "", challenge, 0, "invalid_grant"}, {"wrong verifier", "", "https://example.com/cb", verifier, challenge, 0, "invalid_grant"}} {
		t.Run(tt.name, func(t *testing.T) {
			c := codeTest(t, s, g, n, tt.challenge)
			client := tt.client
			if client == "" {
				client = g.ClientID
			}
			_, e := s.RedeemCode(ctx, c, client, tt.redirect, tt.verifier, n.Add(tt.after), time.Hour)
			var oe *oauthError
			if !errors.As(e, &oe) || oe.Code != tt.want {
				t.Fatalf("%v", e)
			}
		})
	}
	c := codeTest(t, s, g, n, "")
	v, e := s.RedeemCode(ctx, c, g.ClientID, "https://example.com/cb", "", n, time.Hour)
	if e != nil || v.Access == "" || v.Refresh == "" {
		t.Fatal(v, e)
	}
}
func TestCodeReplayRevokes(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	c := codeTest(t, s, g, n, "")
	v, e := s.RedeemCode(ctx, c, g.ClientID, "https://example.com/cb", "", n, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.RedeemCode(ctx, c, g.ClientID, "https://example.com/cb", "", n, time.Hour); e == nil {
		t.Fatal("replay accepted")
	}
	if _, e = s.AuthenticateAt(ctx, v.Access, n); e == nil {
		t.Fatal("replayed access survived")
	}
	if _, e = s.RefreshAccess(ctx, v.Refresh, g.ClientID, n, time.Hour); e == nil {
		t.Fatal("replayed refresh survived")
	}
}
func TestCodeRedeemConcurrent(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	c := codeTest(t, s, g, n, "")
	var wg sync.WaitGroup
	var successes atomic.Int32
	tokens := make(chan IssuedTokens, 8)
	for range 8 {
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()
			v, e := s.RedeemCode(ctx, c, g.ClientID, "https://example.com/cb", "", n, time.Hour)
			if e == nil {
				successes.Add(1)
				tokens <- v
			}
		}(ctx)
	}
	wg.Wait()
	close(tokens)
	if successes.Load() != 1 {
		t.Fatal(successes.Load())
	}
	for v := range tokens {
		if _, e := s.AuthenticateAt(ctx, v.Access, n); e == nil {
			t.Fatal("access survived replay")
		}
		if _, e := s.RefreshAccess(ctx, v.Refresh, g.ClientID, n, time.Hour); e == nil {
			t.Fatal("refresh survived replay")
		}
	}
}
func TestRefresh(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	ctx := context.Background()
	next, e := s.RefreshAccess(ctx, v.Refresh, g.ClientID, n.Add(time.Minute), time.Hour)
	if e != nil || next.Access == v.Access || next.Refresh != "" || next.Scope != v.Scope {
		t.Fatal(next, e)
	}
	if _, e = s.AuthenticateAt(ctx, v.Access, n); e != nil {
		t.Fatal(e)
	}
	if _, e = s.RefreshAccess(ctx, v.Refresh, "wrong", n, time.Hour); e == nil {
		t.Fatal("wrong client")
	}
}
func TestRevoke(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	v := issueTest(t, s, g, n)
	if e := s.RevokeToken(ctx, v.Access); e != nil {
		t.Fatal(e)
	}
	if _, e := s.AuthenticateAt(ctx, v.Access, n); e == nil {
		t.Fatal("revoked access accepted")
	}
	next, e := s.RefreshAccess(ctx, v.Refresh, g.ClientID, n, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeToken(ctx, v.Refresh); e != nil {
		t.Fatal(e)
	}
	if _, e = s.AuthenticateAt(ctx, next.Access, n); e == nil {
		t.Fatal("descendant access survived")
	}
	for _, token := range []string{v.Access, v.Refresh, "garbage"} {
		if !errors.Is(s.RevokeToken(ctx, token), errInvalidToken) {
			t.Fatal("unknown accepted")
		}
	}
	raw := "provisioned-" + g.User.Tenant
	s.Provision(ctx, raw, Principal{Tenant: g.User.Tenant, User: "admin", Role: "admin"})
	if !errors.Is(s.RevokeToken(ctx, raw), errInvalidToken) {
		t.Fatal("provisioned revoked")
	}
	if _, e = s.Authenticate(ctx, raw); e != nil {
		t.Fatal(e)
	}
}
func TestIntrospect(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	ctx := context.Background()
	for _, tok := range []string{v.Access, v.Refresh} {
		i, e := s.Introspect(ctx, tok, g.ClientID, n)
		if e != nil || !i.Active || i.Username != g.User.Username || i.Sub != g.User.ID || i.Scope != g.Scope || i.ClientID != g.ClientID || i.Iat != n.Unix() {
			t.Fatal(i, e)
		}
	}
	for _, tt := range []struct {
		token, client string
		at            time.Time
	}{{v.Access, "wrong", n}, {v.Access, g.ClientID, n.Add(time.Hour)}, {"garbage", g.ClientID, n}} {
		i, e := s.Introspect(ctx, tt.token, tt.client, tt.at)
		if e != nil || i.Active {
			t.Fatal(i, e)
		}
	}
	s.RevokeToken(ctx, v.Refresh)
	i, e := s.Introspect(ctx, v.Access, g.ClientID, n)
	if e != nil || i.Active {
		t.Fatal(i, e)
	}
}
func TestProvisionedTokenUnchanged(t *testing.T) {
	s, g, _ := tokenFixture(t)
	ctx := context.Background()
	raw := "provisioned-" + g.User.Tenant
	p := Principal{Tenant: g.User.Tenant, User: "admin", Role: "admin", Username: "ignored", Scope: "ignored"}
	if e := s.Provision(ctx, raw, p); e != nil {
		t.Fatal(e)
	}
	p, e := s.Authenticate(ctx, raw)
	if e != nil || p.Username != "" || p.Scope != "" || !scopeAllows(p, routeData) || !scopeAllows(p, routeIdentity) {
		t.Fatal(p, e)
	}
}
func TestScopeAllows(t *testing.T) {
	for _, scope := range []string{"api", "full", "id", "profile", "email", "address", "phone", "web", "refresh_token", "offline_access", ""} {
		p := Principal{Username: "a@b", Scope: scope}
		data := scope == "api" || scope == "full"
		identity := data || scope == "id" || scope == "profile" || scope == "email" || scope == "address" || scope == "phone"
		if scopeAllows(p, routeData) != data || scopeAllows(p, routeIdentity) != identity {
			t.Fatal(scope)
		}
	}
}

func TestTenantIsolationWithOAuthToken(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	v := issueTest(t, s, g, n)
	a, e := s.AuthenticateAt(ctx, v.Access, n)
	if e != nil {
		t.Fatal(e)
	}
	id, e := s.Create(ctx, a, "Account", map[string]any{"Name": "private"})
	if e != nil {
		t.Fatal(e)
	}
	tenant := g.User.Tenant + "other"
	name := "other" + g.User.Username
	t.Cleanup(func() { s.pool.Exec(ctx, "DELETE FROM tenants WHERE id=$1", tenant) })
	if _, e = s.CreateUser(ctx, tenant, name, "long-password", "admin"); e != nil {
		t.Fatal(e)
	}
	u, e := s.VerifyLogin(ctx, name, "long-password", n)
	if e != nil {
		t.Fatal(e)
	}
	g.User = u
	g.ClientID = PlatformCLIClientID
	v = issueTest(t, s, g, n)
	b, e := s.AuthenticateAt(ctx, v.Access, n)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Read(ctx, b, "Account", id)
	var api *APIError
	if !errors.As(e, &api) || api.Code != "NOT_FOUND" {
		t.Fatal(e)
	}
	q, _ := ParseQuery("SELECT Id FROM Account")
	rows, e := s.Select(ctx, b, q)
	if e != nil || len(rows) != 0 {
		t.Fatal(rows, e)
	}
}
func TestRestrictedOAuthUser(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	s.pool.Exec(ctx, "UPDATE users SET role='restricted' WHERE id=$1", g.User.ID)
	v := issueTest(t, s, g, n)
	p, e := s.AuthenticateAt(ctx, v.Access, n)
	if e != nil {
		t.Fatal(e)
	}
	if e = Validate(p, "Account", map[string]any{"Name": "private", "Compat_Text__c": "secret"}, true); e == nil {
		t.Fatal("restricted field writable")
	}
	q, _ := ParseQuery("SELECT Compat_Text__c FROM Account")
	if _, e = s.Select(ctx, p, q); e == nil {
		t.Fatal("restricted field readable")
	}
}

func TestConcurrentInvalidation(t *testing.T) {
	for _, action := range []string{"password", "user", "app", "refresh"} {
		t.Run(action, func(t *testing.T) {
			s, g, n := tokenFixture(t)
			ctx := context.Background()
			initial := issueTest(t, s, g, n)
			start := make(chan struct{})
			issued := make(chan IssuedTokens, 16)
			var wg sync.WaitGroup
			for range 12 {
				wg.Add(1)
				go func(ctx context.Context) {
					defer wg.Done()
					<-start
					var v IssuedTokens
					var e error
					if action == "refresh" {
						v, e = s.RefreshAccess(ctx, initial.Refresh, g.ClientID, n, time.Hour)
					} else {
						v, e = s.IssueTokens(ctx, g, n, time.Hour)
					}
					if e == nil {
						issued <- v
					}
				}(ctx)
			}
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				<-start
				var e error
				switch action {
				case "password":
					e = s.SetPassword(ctx, g.User.Username, "new-password")
				case "user":
					e = s.DeactivateUser(ctx, g.User.Username)
				case "app":
					e = s.DisableApp(ctx, g.ClientID)
				case "refresh":
					e = s.RevokeToken(ctx, initial.Refresh)
				}
				if e != nil {
					t.Errorf("invalidation: %v", e)
				}
			}(ctx)
			close(start)
			wg.Wait()
			close(issued)
			for v := range issued {
				if _, e := s.AuthenticateAt(ctx, v.Access, n); e == nil {
					t.Fatal("concurrent token survived invalidation")
				}
				if v.Refresh != "" {
					if _, e := s.RefreshAccess(ctx, v.Refresh, g.ClientID, n, time.Hour); e == nil {
						t.Fatal("concurrent refresh survived invalidation")
					}
				}
			}
		})
	}
}

func TestOAuthTokenPrecedesProvisionedCollision(t *testing.T) {
	s, g, n := tokenFixture(t)
	v := issueTest(t, s, g, n)
	ctx := context.Background()
	if e := s.Provision(ctx, v.Access, Principal{Tenant: g.User.Tenant, User: "different", Role: "restricted"}); e != nil {
		t.Fatal(e)
	}
	p, e := s.AuthenticateAt(ctx, v.Access, n)
	if e != nil || p.User != g.User.ID || p.Username != g.User.Username {
		t.Fatal(p, e)
	}
}

func TestRedeemCodePKCESuccess(t *testing.T) {
	s, g, n := tokenFixture(t)
	verifier := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(verifier))
	c := codeTest(t, s, g, n, base64.RawURLEncoding.EncodeToString(sum[:]))
	v, e := s.RedeemCode(context.Background(), c, g.ClientID, "https://example.com/cb", verifier, n, time.Hour)
	if e != nil || v.Access == "" {
		t.Fatal(v, e)
	}
}
func TestIntrospectProvisionedAndRevoked(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	raw := "introspect-provisioned-" + g.User.Tenant
	if e := s.Provision(ctx, raw, Principal{Tenant: g.User.Tenant, User: "admin", Role: "admin"}); e != nil {
		t.Fatal(e)
	}
	v := issueTest(t, s, g, n)
	if e := s.RevokeToken(ctx, v.Refresh); e != nil {
		t.Fatal(e)
	}
	for _, token := range []string{raw, v.Access, v.Refresh} {
		i, e := s.Introspect(ctx, token, g.ClientID, n)
		if e != nil || i.Active {
			t.Fatal(i, e)
		}
	}
}

func TestIssueCodeRejectsStaleLogin(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	if e := s.SetPassword(ctx, g.User.Username, "new-password"); e != nil {
		t.Fatal(e)
	}
	in := CodeInput{ClientID: g.ClientID, UserID: g.User.ID, RedirectURI: "https://example.com/cb", Scope: g.Scope, loginVersion: &g.User.authVersion}
	if _, e := s.IssueCode(ctx, in, n); e == nil {
		t.Fatal("stale login issued code after password reset")
	}
	u, e := s.VerifyLogin(ctx, g.User.Username, "new-password", n)
	if e != nil {
		t.Fatal(e)
	}
	in.loginVersion = &u.authVersion
	c, e := s.IssueCode(ctx, in, n)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.RedeemCode(ctx, c, g.ClientID, in.RedirectURI, "", n, time.Hour); e != nil {
		t.Fatal(e)
	}
}

func TestExpiredCodeReplayRevokesRefreshedDescendants(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	code := codeTest(t, s, g, n, "")
	first, e := s.RedeemCode(ctx, code, g.ClientID, "https://example.com/cb", "", n, time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	descendant, e := s.RefreshAccess(ctx, first.Refresh, g.ClientID, n.Add(9*time.Minute), time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	at := n.Add(10 * time.Minute)
	_, e = s.RedeemCode(ctx, code, g.ClientID, "https://example.com/cb", "", at, time.Hour)
	var oe *oauthError
	if !errors.As(e, &oe) || oe.Code != "invalid_grant" {
		t.Fatal(e)
	}
	for _, token := range []string{first.Access, descendant.Access} {
		if _, e = s.AuthenticateAt(ctx, token, at); e == nil {
			t.Fatal("access survived expired-code replay")
		}
	}
	if _, e = s.RefreshAccess(ctx, first.Refresh, g.ClientID, at, time.Hour); e == nil {
		t.Fatal("refresh survived expired-code replay")
	}
}

func TestIssueTokensCleansExpiredJWTIDs(t *testing.T) {
	s, g, n := tokenFixture(t)
	ctx := context.Background()
	for _, row := range []struct {
		id string
		at time.Time
	}{{"expired-unrelated", n.Add(-time.Second)}, {"exact-boundary", n}, {"future", n.Add(time.Second)}} {
		if _, e := s.pool.Exec(ctx, "INSERT INTO oauth_jwt_ids(client_id,jti,expires_at) VALUES($1,$2,$3)", g.ClientID, row.id, row.at); e != nil {
			t.Fatal(e)
		}
	}
	issueTest(t, s, g, n)
	var ids []string
	rows, e := s.pool.Query(ctx, "SELECT jti FROM oauth_jwt_ids WHERE client_id=$1 ORDER BY jti", g.ClientID)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	if strings.Join(ids, ",") != "exact-boundary,future" {
		t.Fatal(ids)
	}
}
