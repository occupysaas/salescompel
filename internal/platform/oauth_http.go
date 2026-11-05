package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	PublicURL      string
	AccessTokenTTL time.Duration
	Now            func() time.Time
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}
func (o Options) ttl() time.Duration {
	if o.AccessTokenTTL > 0 {
		return o.AccessTokenTTL
	}
	return 2 * time.Hour
}
func loopback(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}
func ResolvePublicURL(flagValue, addr string, tls bool) (string, error) {
	bad := errors.New("public URL must be an origin using HTTPS, or HTTP on loopback")
	if flagValue == "" {
		host, _, e := net.SplitHostPort(addr)
		if e != nil || !loopback(host) {
			return "", bad
		}
		scheme := "http"
		if tls {
			scheme = "https"
		}
		flagValue = scheme + "://" + addr
	}
	u, e := url.Parse(flagValue)
	if e != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Scheme != "https" && (u.Scheme != "http" || !loopback(strings.ToLower(u.Hostname())))) {
		return "", bad
	}
	// Match browser Origin serialization: DNS names are case-insensitive and
	// default ports are omitted. Keep brackets for an IPv6 host without a port.
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	u.Path = ""
	return u.String(), nil
}
func writeOAuthError(w http.ResponseWriter, e error) {
	var oe *oauthError
	if !errors.As(e, &oe) {
		writeJSON(w, 500, &oauthError{Code: "server_error", Description: "OAuth request failed"})
		return
	}
	writeJSON(w, 400, oe)
}
func oauthForm(w http.ResponseWriter, r *http.Request) (url.Values, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, grantError("invalid_request"))
		return nil, false
	}
	mt, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || mt != "application/x-www-form-urlencoded" {
		writeOAuthError(w, grantError("invalid_request"))
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if e = r.ParseForm(); e != nil {
		writeOAuthError(w, grantError("invalid_request"))
		return nil, false
	}
	for _, vs := range r.Form {
		if len(vs) != 1 {
			writeOAuthError(w, grantError("invalid_request"))
			return nil, false
		}
	}
	return r.Form, true
}
func clientCredentials(r *http.Request, v url.Values) (string, string, error) {
	id, secret := v.Get("client_id"), v.Get("client_secret")
	if r.Header.Get("Authorization") != "" {
		a, b, ok := r.BasicAuth()
		if !ok {
			return "", "", grantError("invalid_client")
		}
		if (v.Has("client_id") && id != a) || (v.Has("client_secret") && secret != b) {
			return "", "", grantError("invalid_request")
		}
		id, secret = a, b
	}
	return id, secret, nil
}
func validClientSecret(a OAuthApp, secret string) bool {
	return secret != "" && a.SecretHash != "" && subtle.ConstantTimeCompare([]byte(digest(secret)), []byte(a.SecretHash)) == 1
}

const jwtGrant = "urn:ietf:params:oauth:grant-type:jwt-bearer"

func handleToken(s *Store, o Options, w http.ResponseWriter, r *http.Request) {
	v, ok := oauthForm(w, r)
	if !ok {
		return
	}
	grant := v.Get("grant_type")
	if grant == "" {
		writeOAuthError(w, grantError("invalid_request"))
		return
	}
	if !slices.Contains([]string{"authorization_code", "refresh_token", "client_credentials", jwtGrant}, grant) {
		writeOAuthError(w, grantError("unsupported_grant_type"))
		return
	}
	id, secret, e := clientCredentials(r, v)
	if e != nil {
		writeOAuthError(w, e)
		return
	}
	n := o.now()
	var claims JWTClaims
	var parsed parsedJWT
	if grant == jwtGrant {
		claims, parsed, e = parseJWT(v.Get("assertion"))
		if e != nil || (id != "" && id != claims.Issuer) {
			writeOAuthError(w, grantError("invalid_grant"))
			return
		}
		id = claims.Issuer
	}
	a, e := s.LookupApp(r.Context(), id)
	if e != nil || !a.Active {
		code := "invalid_client_id"
		if grant == jwtGrant {
			code = "invalid_grant"
		}
		writeOAuthError(w, grantError(code))
		return
	}
	if grant != jwtGrant && (!a.Public || grant == "client_credentials") && !validClientSecret(a, secret) {
		writeOAuthError(w, grantError("invalid_client"))
		return
	}
	if !slices.Contains(a.Grants, grant) {
		writeOAuthError(w, grantError("unsupported_grant_type"))
		return
	}
	// Validate before a code is redeemed or an assertion ID is claimed. Code
	// and refresh grants retain their stored scopes regardless of this subset.
	requested := a.Scopes
	if v.Has("scope") {
		requested = strings.Fields(v.Get("scope"))
		for _, scope := range requested {
			if !slices.Contains(a.Scopes, scope) {
				writeOAuthError(w, grantError("invalid_scope"))
				return
			}
		}
	}
	var tokens IssuedTokens
	switch grant {
	case "authorization_code":
		if a.Public && v.Get("code_verifier") == "" {
			writeOAuthError(w, grantError("invalid_grant"))
			return
		}
		tokens, e = s.RedeemCode(r.Context(), v.Get("code"), id, v.Get("redirect_uri"), v.Get("code_verifier"), n, o.ttl())
	case "refresh_token":
		tokens, e = s.RefreshAccess(r.Context(), v.Get("refresh_token"), id, n, o.ttl())
	default:
		var u User
		if grant == jwtGrant {
			cert, ce := parseJWTCertificate(a.JWTCertPEM, n)
			if ce != nil || verifyJWT(parsed, cert, []string{o.PublicURL, o.PublicURL + "/"}, n) != nil {
				writeOAuthError(w, grantError("invalid_grant"))
				return
			}
			e = s.authPool().QueryRow(r.Context(), `SELECT u.id,u.tenant_id,u.username,u.role,u.active,u.auth_version FROM users u JOIN oauth_app_jwt_users j ON j.user_id=u.id WHERE j.client_id=$1 AND u.username_canonical=$2 AND u.tenant_id=$3 AND u.active`, id, strings.ToLower(claims.Subject), a.Tenant).Scan(&u.ID, &u.Tenant, &u.Username, &u.Role, &u.Active, &u.authVersion)
			if e == nil && claims.ID != "" {
				// Keep the identifier for the entire assertion validity window. The
				// conditional upsert admits only one concurrent use, even after expiry.
				expiry := time.Unix(parsed.expires, 0)
				if parsed.expires >= 100000000000 {
					expiry = time.Unix(parsed.issuedAt, 0).Add(5 * time.Minute)
				}
				tag, claimErr := s.authPool().Exec(r.Context(), `INSERT INTO oauth_jwt_ids(client_id,jti,expires_at) VALUES($1,$2,$3) ON CONFLICT(client_id,jti) DO UPDATE SET expires_at=EXCLUDED.expires_at WHERE oauth_jwt_ids.expires_at < $4`, id, claims.ID, expiry, n)
				e = claimErr
				if e == nil && tag.RowsAffected() != 1 {
					e = grantError("invalid_grant")
				}
			}
		} else {
			e = s.authPool().QueryRow(r.Context(), `SELECT id,tenant_id,username,role,active,auth_version FROM users WHERE id=$1 AND tenant_id=$2 AND active`, a.RunAsUserID, a.Tenant).Scan(&u.ID, &u.Tenant, &u.Username, &u.Role, &u.Active, &u.authVersion)
		}
		if e != nil {
			writeOAuthError(w, grantError("invalid_grant"))
			return
		}
		scopes := []string{}
		for _, scope := range requested {
			if scope != "refresh_token" && scope != "offline_access" {
				scopes = append(scopes, scope)
			}
		}
		tokens, e = s.IssueTokens(r.Context(), TokenGrant{ClientID: id, User: u, Scope: strings.Join(scopes, " ")}, n, o.ttl())
	}
	if e != nil {
		writeOAuthError(w, e)
		return
	}
	identity := o.PublicURL + "/id/" + orgID(tokens.User.Tenant) + "/" + tokens.User.ID
	issued := strconv.FormatInt(tokens.IssuedAt.UnixMilli(), 10)
	result := map[string]any{"access_token": tokens.Access, "scope": tokens.Scope, "instance_url": o.PublicURL, "id": identity, "token_type": "Bearer", "issued_at": issued}
	if tokens.Refresh != "" {
		result["refresh_token"] = tokens.Refresh
	}
	if secret != "" && validClientSecret(a, secret) {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(identity + issued))
		result["signature"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	writeJSON(w, 200, result)
}
func handleRevoke(s *Store, o Options, w http.ResponseWriter, r *http.Request) {
	v, ok := oauthForm(w, r)
	if !ok {
		return
	}
	if e := s.RevokeToken(r.Context(), v.Get("token")); e != nil {
		if errors.Is(e, errInvalidToken) {
			writeOAuthError(w, &oauthError{Code: "invalid_token", Description: "Invalid token"})
		} else {
			writeOAuthError(w, e)
		}
		return
	}
	w.WriteHeader(200)
}
func handleIntrospect(s *Store, o Options, w http.ResponseWriter, r *http.Request) {
	v, ok := oauthForm(w, r)
	if !ok {
		return
	}
	id, sec, e := clientCredentials(r, v)
	if e != nil {
		writeOAuthError(w, e)
		return
	}
	a, e := s.LookupApp(r.Context(), id)
	if e != nil || !a.Active || !validClientSecret(a, sec) {
		writeOAuthError(w, grantError("invalid_client"))
		return
	}
	result, e := s.Introspect(r.Context(), v.Get("token"), id, o.now())
	if e != nil {
		writeOAuthError(w, e)
		return
	}
	writeJSON(w, 200, result)
}

func handleDiscovery(_ *Store, o Options, w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	writeJSON(w, 200, map[string]any{
		"issuer": o.PublicURL, "authorization_endpoint": o.PublicURL + "/services/oauth2/authorize", "token_endpoint": o.PublicURL + "/services/oauth2/token", "revocation_endpoint": o.PublicURL + "/services/oauth2/revoke", "introspection_endpoint": o.PublicURL + "/services/oauth2/introspect", "userinfo_endpoint": o.PublicURL + "/services/oauth2/userinfo",
		"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token", "client_credentials", jwtGrant}, "scopes_supported": []string{"api", "full", "id", "profile", "email", "address", "phone", "web", "refresh_token", "offline_access"}, "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
	})
}
func identityURL(p Principal, o Options) string {
	return o.PublicURL + "/id/" + orgID(p.Tenant) + "/" + p.User
}
func handleIdentity(p Principal, o Options, w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	if "/"+strings.TrimLeft(r.URL.EscapedPath(), "/") != "/id/"+orgID(p.Tenant)+"/"+p.User {
		writeError(w, problem(403, "INSUFFICIENT_ACCESS", "Identity does not belong to this token"))
		return
	}
	username := p.Username
	if username == "" {
		username = p.User
	}
	writeJSON(w, 200, map[string]any{"id": identityURL(p, o), "user_id": p.User, "organization_id": orgID(p.Tenant), "username": username})
}
func isOrganizationDiscovery(q string) bool {
	// Tokenize identifiers and commas separately, so whitespace never changes a
	// token boundary ("Org anization" must not become "Organization").
	q = strings.ReplaceAll(q, ",", " , ")
	return strings.EqualFold(strings.Join(strings.Fields(q), " "), "SELECT Name , InstanceName , IsSandbox , TrialExpirationDate , NamespacePrefix , OrganizationType FROM Organization")
}
