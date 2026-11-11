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

func TestDisabledAppLosesAccess(t *testing.T) { testInvalidation(t, "app") }

func TestSetPasswordRevokes(t *testing.T) { testInvalidation(t, "password") }

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
