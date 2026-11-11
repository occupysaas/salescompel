package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"salescompel/locales"
)

func authFixture(t *testing.T) (*Store, User, string, time.Time) {
	t.Helper()
	s, u, id, _, n := httpApp(t, []string{"authorization_code"})
	return s, u, id, n
}
func authQuery(id string) url.Values {
	return url.Values{"client_id": {id}, "redirect_uri": {"https://callback.example.com"}, "response_type": {"code"}, "state": {"a&b=<script>\"'"}}
}
func authGet(s *Store, n time.Time, q url.Values, lang string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/services/oauth2/authorize?"+q.Encode(), nil)
	r.Header.Set("Accept-Language", lang)
	w := httptest.NewRecorder()
	NewHandlerWithOptions(s, "", Options{PublicURL: "https://crm.example.com", Now: func() time.Time { return n }}).ServeHTTP(w, r)
	return w
}
func authID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	m := regexp.MustCompile(`name="request_id" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	return m[1]
}
func authPost(s *Store, n time.Time, id, user, password, decision string, cookie *http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	v := url.Values{"request_id": {id}, "username": {user}, "password": {password}, "decision": {decision}}
	r := httptest.NewRequest("POST", "/services/oauth2/authorize", strings.NewReader(v.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	NewHandlerWithOptions(s, "", Options{PublicURL: "https://crm.example.com", Now: func() time.Time { return n }}).ServeHTTP(w, r)
	return w
}
func location(t *testing.T, w *httptest.ResponseRecorder) url.Values {
	t.Helper()
	u, e := url.Parse(w.Header().Get("Location"))
	if e != nil {
		t.Fatal(e)
	}
	return u.Query()
}
func TestAuthorizeValidation(t *testing.T) {
	s, _, id, n := authFixture(t)
	for _, field := range []string{"client_id", "redirect_uri"} {
		q := authQuery(id)
		q.Set(field, "unknown")
		w := authGet(s, n, q, "")
		if w.Code < 400 || w.Header().Get("Location") != "" {
			t.Fatal(field, w.Code, w.Header())
		}
	}
	for _, tc := range []struct{ k, v, e string }{{"response_type", "token", "unsupported_response_type"}, {"code_challenge_method", "plain", "invalid_request"}, {"scope", "bad", "invalid_scope"}, {"scope", "openid", "invalid_scope"}, {"scope", "web", "invalid_scope"}, {"prompt", "none", "login_required"}, {"client_id", PlatformCLIClientID, "invalid_request"}} {
		q := authQuery(id)
		q.Set(tc.k, tc.v)
		if tc.k == "client_id" {
			q.Set("redirect_uri", "http://localhost:1717/OauthRedirect")
		}
		w := authGet(s, n, q, "")
		v := location(t, w)
		if w.Code != 302 || v.Get("error") != tc.e || v.Get("state") != q.Get("state") {
			t.Fatal(tc, w.Code, v)
		}
	}
	w := authGet(s, n, authQuery(id), "")
	req, e := s.LoadAuthRequest(context.Background(), authID(t, w), w.Result().Cookies()[0].Value, n)
	if e != nil || req.Scope != "api refresh_token" {
		t.Fatal(req, e)
	}
}
func TestAuthorizePage(t *testing.T) {
	s, _, id, n := authFixture(t)
	w := authGet(s, n, authQuery(id), "")
	authID(t, w)
	b := w.Body.String()
	if w.Code != 200 || !strings.Contains(b, "http") || strings.Contains(b, `name="client_id"`) || strings.Contains(b, `name="code_challenge"`) || strings.Contains(b, "<script") || strings.Contains(b, "style=") || w.Header().Get("Referrer-Policy") != "same-origin" || strings.Contains(w.Header().Get("Content-Security-Policy"), "form-action") {
		t.Fatal(w.Code, b, w.Header())
	}
	c := w.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
		t.Fatal(c)
	}
	for _, public := range []string{"http://127.0.0.1:8080", "https://crm.example.com"} {
		r := httptest.NewRequest("GET", "/services/oauth2/authorize?"+authQuery(id).Encode(), nil)
		out := httptest.NewRecorder()
		NewHandlerWithOptions(s, "", Options{PublicURL: public}).ServeHTTP(out, r)
		if out.Result().Cookies()[0].Secure != strings.HasPrefix(public, "https:") {
			t.Fatal(public)
		}
	}
	css := httptest.NewRecorder()
	NewHandlerWithOptions(s, "", Options{PublicURL: "https://crm.example.com"}).ServeHTTP(css, httptest.NewRequest("GET", "/services/oauth2/authorize.css", nil))
	if css.Code != 200 || !strings.HasPrefix(css.Header().Get("Content-Type"), "text/css") || css.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal(css.Code, css.Header())
	}
}
func TestAuthorizeLogin(t *testing.T) {
	s, u, id, n := authFixture(t)
	q := authQuery(id)
	w := authGet(s, n, q, "")
	rid, c := authID(t, w), w.Result().Cookies()[0]
	wrong := authPost(s, n, rid, u.Username, "wrong", "allow", c, nil)
	if wrong.Code != 200 || !strings.Contains(wrong.Body.String(), `role="alert"`) {
		t.Fatal(wrong.Code, wrong.Body.String())
	}
	good := authPost(s, n, rid, u.Username, "long-password", "allow", c, nil)
	v := location(t, good)
	if good.Code != 303 || v.Get("code") == "" || v.Get("state") != q.Get("state") {
		t.Fatal(good.Code, v)
	}
	if _, e := s.RedeemCode(context.Background(), v.Get("code"), id, q.Get("redirect_uri"), "", n, time.Hour); e != nil {
		t.Fatal(e)
	}
	if v := location(t, authPost(s, n, rid, u.Username, "long-password", "allow", c, nil)); v.Get("code") != "" {
		t.Fatal(v)
	}
	w = authGet(s, n, q, "")
	deny := authPost(s, n, authID(t, w), "", "", "deny", w.Result().Cookies()[0], nil)
	if deny.Code != 303 || location(t, deny).Get("error") != "access_denied" {
		t.Fatal(deny.Code, deny.Header())
	}
}
func TestAuthorizeGenericFailure(t *testing.T) {
	s, u, id, n := authFixture(t)
	ctx := context.Background()
	otherTenant := u.Tenant + "-other"
	t.Cleanup(func() { s.pool.Exec(context.Background(), "DELETE FROM tenants WHERE id=$1", otherTenant) })
	var message string
	for _, kind := range []string{"unknown", "wrong", "inactive", "locked", "foreign"} {
		name := u.Username
		password := "long-password"
		s.pool.Exec(ctx, "UPDATE users SET active=true,locked_until=NULL WHERE id=$1", u.ID)
		switch kind {
		case "unknown":
			name = "absent@example.com"
		case "wrong":
			password = "wrong"
		case "inactive":
			s.pool.Exec(ctx, "UPDATE users SET active=false WHERE id=$1", u.ID)
		case "locked":
			s.pool.Exec(ctx, "UPDATE users SET locked_until=$2 WHERE id=$1", u.ID, n.Add(time.Hour))
		case "foreign":
			if _, e := s.pool.Exec(ctx, "INSERT INTO tenants(id) VALUES($1)", otherTenant); e != nil {
				t.Fatal(e)
			}
			s.pool.Exec(ctx, "UPDATE oauth_apps SET tenant_id=$2 WHERE client_id=$1", id, otherTenant)
		}
		w := authGet(s, n, authQuery(id), "")
		out := authPost(s, n, authID(t, w), name, password, "allow", w.Result().Cookies()[0], nil)
		m := regexp.MustCompile(`<p role="alert">([^<]+)</p>`).FindStringSubmatch(out.Body.String())
		if len(m) != 2 || location(t, out).Get("code") != "" {
			t.Fatal(kind, out.Code, out.Body.String())
		}
		if message == "" {
			message = m[1]
		} else if message != m[1] {
			t.Fatal(kind, m[1])
		}
	}
}
func TestAuthorizeForgery(t *testing.T) {
	s, u, id, n := authFixture(t)
	for _, kind := range []string{"missing", "other", "origin", "null", "cross", "fetch-none", "expired", "valid"} {
		w := authGet(s, n, authQuery(id), "")
		rid, c := authID(t, w), w.Result().Cookies()[0]
		headers := map[string]string{}
		at := n
		switch kind {
		case "missing":
			c = nil
		case "other":
			other := authGet(s, n, authQuery(id), "")
			c = other.Result().Cookies()[0]
		case "origin":
			headers["Origin"] = "https://evil.example"
		case "null":
			headers["Origin"] = "null"
		case "cross":
			headers["Sec-Fetch-Site"] = "cross-site"
		case "fetch-none":
			headers["Sec-Fetch-Site"] = "none"
		case "expired":
			at = n.Add(10 * time.Minute)
		}
		out := authPost(s, at, rid, u.Username, "long-password", "allow", c, headers)
		has := location(t, out).Get("code") != ""
		if has != (kind == "valid") {
			t.Fatal(kind, out.Code, out.Header())
		}
	}
}
func TestAuthorizeEscaping(t *testing.T) {
	s, u, id, n := authFixture(t)
	ctx := context.Background()
	redirect := "https://c.example/cb?x=1"
	if _, e := s.pool.Exec(ctx, "UPDATE oauth_apps SET name=$2,redirect_uris=$3 WHERE client_id=$1", id, `<img src=x onerror=alert(1)>`, []string{redirect}); e != nil {
		t.Fatal(e)
	}
	q := authQuery(id)
	q.Set("redirect_uri", redirect)
	w := authGet(s, n, q, "")
	if strings.Contains(w.Body.String(), "<img") || strings.Contains(w.Body.String(), q.Get("state")) {
		t.Fatal(w.Body.String())
	}
	out := authPost(s, n, authID(t, w), u.Username, "long-password", "allow", w.Result().Cookies()[0], nil)
	v := location(t, out)
	if v.Get("x") != "1" || v.Get("state") != q.Get("state") || v.Get("code") == "" {
		t.Fatal(v)
	}
}
func TestAuthorizeLocalization(t *testing.T) {
	s, _, id, n := authFixture(t)
	for _, tc := range []struct{ lang, want string }{{"fr", "Identifiant"}, {"iw", `dir="rtl"`}, {"he", `dir="rtl"`}, {"xx", "Username"}} {
		w := authGet(s, n, authQuery(id), tc.lang)
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Fatal(tc, w.Body.String())
		}
	}
	var base map[string]string
	b, _ := locales.Messages.ReadFile("messages/en_US.json")
	if e := json.Unmarshal(b, &base); e != nil {
		t.Fatal(e)
	}
	for _, lang := range []string{"en_US", "fr", "de", "pt_BR", "iw"} {
		b, e := locales.Messages.ReadFile("messages/" + lang + ".json")
		if e != nil {
			t.Fatal(e)
		}
		var m map[string]string
		json.Unmarshal(b, &m)
		count := 0
		for k := range base {
			if strings.HasPrefix(k, "oauth.") {
				count++
				if m[k] == "" {
					t.Fatal(lang, k)
				}
			}
		}
		if count < 20 {
			t.Fatal("scope labels must be translated", count)
		}
	}
}
func TestAuthorizeSingleUseConcurrent(t *testing.T) {
	s, u, id, n := authFixture(t)
	w := authGet(s, n, authQuery(id), "")
	rid, c := authID(t, w), w.Result().Cookies()[0]
	var wg sync.WaitGroup
	codes := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()
			out := authPost(s, n, rid, u.Username, "long-password", "allow", c, nil)
			v, _ := url.Parse(out.Header().Get("Location"))
			codes <- v.Query().Get("code")
		}(context.Background())
	}
	wg.Wait()
	close(codes)
	count := 0
	for code := range codes {
		if code != "" {
			count++
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}
func TestAuthRequestExpiryCleanup(t *testing.T) {
	s, _, id, n := authFixture(t)
	ctx := context.Background()
	r := AuthRequest{ClientID: id}
	old, _, e := s.SaveAuthRequest(ctx, r, n.Add(-10*time.Minute))
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.SaveAuthRequest(ctx, r, n); e != nil {
		t.Fatal(e)
	}
	var count int
	s.pool.QueryRow(ctx, "SELECT count(*) FROM oauth_requests WHERE request_hash=$1", digest(old)).Scan(&count)
	if count != 0 {
		t.Fatal(count)
	}
}
