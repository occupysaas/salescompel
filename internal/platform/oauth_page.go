package platform

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/language"
	"salescompel/locales"
)

//go:embed oauth_page.html
var oauthPageHTML string

//go:embed oauth_page.css
var oauthPageCSS string
var oauthPageTemplate = template.Must(template.New("oauth").Parse(oauthPageHTML))

const authCookie = "salescompel_oauth_request"

type AuthRequest struct{ ClientID, RedirectURI, Scope, State, Challenge string }

func (s *Store) SaveAuthRequest(ctx context.Context, in AuthRequest, now time.Time) (id, binding string, err error) {
	id, err = newSecret()
	if err != nil {
		return
	}
	binding, err = newSecret()
	if err != nil {
		return
	}
	tx, e := s.authPool().Begin(ctx)
	if e != nil {
		return "", "", e
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM oauth_requests WHERE expires_at<=$1", now); err != nil {
		return
	}
	_, err = tx.Exec(ctx, "INSERT INTO oauth_requests(request_hash,client_id,redirect_uri,scope,state,code_challenge,cookie_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", digest(id), in.ClientID, in.RedirectURI, in.Scope, in.State, in.Challenge, digest(binding), now.Add(10*time.Minute))
	if err != nil {
		return
	}
	err = tx.Commit(ctx)
	return
}
func (s *Store) LoadAuthRequest(ctx context.Context, id, binding string, now time.Time) (AuthRequest, error) {
	var in AuthRequest
	if id == "" || binding == "" {
		return in, grantError("invalid_request")
	}
	err := s.authPool().QueryRow(ctx, "SELECT client_id,redirect_uri,scope,state,code_challenge FROM oauth_requests WHERE request_hash=$1 AND cookie_hash=$2 AND expires_at>$3", digest(id), digest(binding), now).Scan(&in.ClientID, &in.RedirectURI, &in.Scope, &in.State, &in.Challenge)
	if err != nil {
		return AuthRequest{}, grantError("invalid_request")
	}
	return in, nil
}
func (s *Store) DeleteAuthRequest(ctx context.Context, id string) error {
	tag, err := s.authPool().Exec(ctx, "DELETE FROM oauth_requests WHERE request_hash=$1", digest(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return grantError("invalid_request")
	}
	return nil
}

var oauthLanguages = []string{"en_US", "fr", "de", "pt_BR", "iw"}
var oauthMatcher = language.NewMatcher([]language.Tag{language.AmericanEnglish, language.French, language.German, language.BrazilianPortuguese, language.Hebrew})

func oauthMessages(header string) (map[string]string, string, string) {
	tags, _, _ := language.ParseAcceptLanguage(header)
	_, i, _ := oauthMatcher.Match(tags...)
	name := oauthLanguages[i]
	b, _ := locales.Messages.ReadFile("messages/" + name + ".json")
	m := map[string]string{}
	_ = json.Unmarshal(b, &m)
	dir := "ltr"
	lang := strings.ReplaceAll(name, "_", "-")
	if name == "iw" {
		dir = "rtl"
		lang = "he"
	}
	return m, lang, dir
}

type oauthPageData struct {
	M                         map[string]string
	Lang, Dir, App, ID, Error string
	Scopes                    []string
}

func renderOAuthPage(w http.ResponseWriter, r *http.Request, status int, app, id, scope, errKey string) {
	m, lang, dir := oauthMessages(r.Header.Get("Accept-Language"))
	d := oauthPageData{M: m, Lang: lang, Dir: dir, App: app, ID: id}
	if errKey != "" {
		d.Error = m[errKey]
	}
	for _, s := range strings.Fields(scope) {
		d.Scopes = append(d.Scopes, m["oauth.scope."+s])
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.WriteHeader(status)
	_ = oauthPageTemplate.Execute(w, d)
}
func oauthRedirect(w http.ResponseWriter, r *http.Request, in AuthRequest, key, value string) {
	u, e := url.Parse(in.RedirectURI)
	if e != nil {
		http.Error(w, "Invalid OAuth request", 400)
		return
	}
	q := u.Query()
	q.Set(key, value)
	q.Set("state", in.State)
	u.RawQuery = q.Encode()
	status := 302
	if r.Method == "POST" {
		status = 303
	}
	http.Redirect(w, r, u.String(), status)
}
func handleAuthorizeCSS(_ *Store, _ Options, w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET", "HEAD") {
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	if r.Method != "HEAD" {
		_, _ = w.Write([]byte(oauthPageCSS))
	}
}
func handleAuthorize(s *Store, o Options, w http.ResponseWriter, r *http.Request) {
	fail := func() { renderOAuthPage(w, r, 400, "", "", "", "oauth.invalid") }
	if r.Method == "POST" {
		if origin, ok := r.Header["Origin"]; ok && (len(origin) != 1 || origin[0] != o.PublicURL) {
			fail()
			return
		}
		if site, ok := r.Header["Sec-Fetch-Site"]; ok && (len(site) != 1 || site[0] != "same-origin") {
			fail()
			return
		}
		v, ok := oauthForm(w, r)
		if !ok {
			return
		}
		cookie, e := r.Cookie(authCookie)
		if e != nil {
			fail()
			return
		}
		in, e := s.LoadAuthRequest(r.Context(), v.Get("request_id"), cookie.Value, o.now())
		if e != nil {
			fail()
			return
		}
		app, e := s.LookupApp(r.Context(), in.ClientID)
		if e != nil || !app.Active || !slices.Contains(app.RedirectURIs, in.RedirectURI) || !slices.Contains(app.Grants, "authorization_code") {
			fail()
			return
		}
		if v.Get("decision") == "deny" {
			if s.DeleteAuthRequest(r.Context(), v.Get("request_id")) != nil {
				fail()
				return
			}
			oauthRedirect(w, r, in, "error", "access_denied")
			return
		}
		if v.Get("decision") != "allow" {
			fail()
			return
		}
		u, e := s.VerifyLogin(r.Context(), v.Get("username"), v.Get("password"), o.now())
		if e != nil || (app.Tenant != "" && u.Tenant != app.Tenant) {
			renderOAuthPage(w, r, 200, app.Name, v.Get("request_id"), in.Scope, "oauth.failure")
			return
		}
		// Claim the pending request atomically. Losing concurrent POSTs cannot issue a code.
		if s.DeleteAuthRequest(r.Context(), v.Get("request_id")) != nil {
			fail()
			return
		}
		code, e := s.IssueCode(r.Context(), CodeInput{ClientID: in.ClientID, UserID: u.ID, RedirectURI: in.RedirectURI, Scope: in.Scope, Challenge: in.Challenge, loginVersion: &u.authVersion}, o.now())
		if e != nil {
			renderOAuthPage(w, r, 400, app.Name, "", "", "oauth.failure")
			return
		}
		oauthRedirect(w, r, in, "code", code)
		return
	}
	if r.Method != "GET" {
		method(w, r, "GET", "POST")
		return
	}
	q, e := url.ParseQuery(r.URL.RawQuery)
	if e != nil {
		fail()
		return
	}
	for _, v := range q {
		if len(v) != 1 {
			fail()
			return
		}
	}
	app, e := s.LookupApp(r.Context(), q.Get("client_id"))
	if e != nil || !app.Active || !slices.Contains(app.RedirectURIs, q.Get("redirect_uri")) {
		fail()
		return
	}
	in := AuthRequest{ClientID: app.ClientID, RedirectURI: q.Get("redirect_uri"), State: q.Get("state"), Challenge: q.Get("code_challenge"), Scope: strings.Join(app.Scopes, " ")}
	reject := func(code string) { oauthRedirect(w, r, in, "error", code) }
	if q.Get("response_type") != "code" {
		reject("unsupported_response_type")
		return
	}
	if !slices.Contains(app.Grants, "authorization_code") {
		reject("unauthorized_client")
		return
	}
	if q.Get("code_challenge_method") != "" && q.Get("code_challenge_method") != "S256" {
		reject("invalid_request")
		return
	}
	if in.Challenge != "" {
		b, err := base64.RawURLEncoding.DecodeString(in.Challenge)
		if err != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != in.Challenge {
			reject("invalid_request")
			return
		}
	} else if app.Public || q.Get("code_challenge_method") != "" {
		reject("invalid_request")
		return
	}
	if q.Has("scope") {
		scopes := strings.Fields(q.Get("scope"))
		for _, scope := range scopes {
			if !slices.Contains(app.Scopes, scope) {
				reject("invalid_scope")
				return
			}
		}
		in.Scope = strings.Join(scopes, " ")
	}
	if slices.Contains(strings.Fields(q.Get("prompt")), "none") {
		reject("login_required")
		return
	}
	id, binding, e := s.SaveAuthRequest(r.Context(), in, o.now())
	if e != nil {
		if !errors.Is(e, context.Canceled) {
			fail()
		}
		return
	}
	http.SetCookie(w, &http.Cookie{Name: authCookie, Value: binding, Path: "/services/oauth2/authorize", MaxAge: 600, HttpOnly: true, Secure: strings.HasPrefix(o.PublicURL, "https://"), SameSite: http.SameSiteStrictMode})
	renderOAuthPage(w, r, 200, app.Name, id, in.Scope, "")
}
