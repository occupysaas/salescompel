package platform

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type TokenGrant struct {
	ClientID string
	User     User
	Scope    string
	Refresh  bool
	CodeHash string
}
type IssuedTokens struct {
	Access, Refresh, Scope string
	User                   User
	IssuedAt               time.Time
}
type CodeInput struct {
	ClientID, UserID, RedirectURI, Scope, Challenge string
	loginVersion                                    *int64
}
type Introspection struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Sub       string `json:"sub,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
}
type oauthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *oauthError) Error() string { return e.Code + ": " + e.Description }
func grantError(code string) *oauthError {
	descriptions := map[string]string{"invalid_grant": "Invalid authorization grant", "redirect_uri_mismatch": "Redirect URI does not match", "invalid_client": "Invalid client credentials", "invalid_request": "Invalid OAuth request", "invalid_scope": "Invalid scope", "unsupported_grant_type": "Unsupported grant type", "unsupported_response_type": "Unsupported response type", "access_denied": "Access denied"}
	d := descriptions[code]
	if d == "" {
		d = "OAuth request failed"
	}
	return &oauthError{Code: code, Description: d}
}

var errInvalidToken = errors.New("invalid token")

type routeClass int

const (
	routeData routeClass = iota
	routeIdentity
)

func scopeAllows(p Principal, route routeClass) bool {
	if p.Username == "" {
		return true
	}
	for _, scope := range strings.Fields(p.Scope) {
		if scope == "api" || scope == "full" {
			return true
		}
		if route == routeIdentity && (scope == "id" || scope == "profile" || scope == "email" || scope == "address" || scope == "phone") {
			return true
		}
	}
	return false
}

func (s *Store) AuthenticateAt(ctx context.Context, token string, now time.Time) (Principal, error) {
	var p Principal
	if token == "" {
		return p, problem(401, "INVALID_SESSION_ID", "Bearer token required")
	}
	e := s.pool.QueryRow(ctx, `SELECT u.tenant_id,u.id,u.role,u.username,t.scope FROM oauth_access_tokens t JOIN users u ON u.id=t.user_id LEFT JOIN oauth_apps a ON a.client_id=t.client_id WHERE t.token_hash=$1 AND t.expires_at>$2 AND u.active AND (t.client_id=$3 OR (a.active AND a.tenant_id=u.tenant_id))`, digest(token), now, PlatformCLIClientID).Scan(&p.Tenant, &p.User, &p.Role, &p.Username, &p.Scope)
	if e == nil {
		return p, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Principal{}, dbError(e)
	}
	e = s.pool.QueryRow(ctx, "SELECT tenant_id,user_id,role FROM principals WHERE token_hash=$1", digest(token)).Scan(&p.Tenant, &p.User, &p.Role)
	if e == nil {
		return p, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Principal{}, dbError(e)
	}

	return Principal{}, problem(401, "INVALID_SESSION_ID", "Invalid session")
}

// Locks always follow app -> user -> code/refresh/access. Admin invalidation locks
// the corresponding parent first, so issuance cannot commit after invalidation.
func lockTokenUser(ctx context.Context, tx pgx.Tx, client, user string) (User, error) {
	var u User
	var tenant string
	if client != PlatformCLIClientID {
		var active bool
		if e := tx.QueryRow(ctx, "SELECT tenant_id,active FROM oauth_apps WHERE client_id=$1 FOR UPDATE", client).Scan(&tenant, &active); e != nil || !active {
			return u, grantError("invalid_grant")
		}
	}
	e := tx.QueryRow(ctx, "SELECT id,tenant_id,username,role,active,auth_version FROM users WHERE id=$1 FOR UPDATE", user).Scan(&u.ID, &u.Tenant, &u.Username, &u.Role, &u.Active, &u.authVersion)
	if e != nil || !u.Active || (tenant != "" && tenant != u.Tenant) {
		return User{}, grantError("invalid_grant")
	}
	return u, nil
}
func insertTokens(ctx context.Context, tx pgx.Tx, g TokenGrant, now time.Time, ttl time.Duration, refreshHash string) (IssuedTokens, error) {
	v := IssuedTokens{User: g.User, Scope: g.Scope, IssuedAt: now}
	if ttl <= 0 {
		return v, grantError("invalid_grant")
	}
	var e error
	if _, e = tx.Exec(ctx, "DELETE FROM oauth_access_tokens WHERE expires_at<=$1", now); e != nil {
		return v, e
	}
	// JWT IDs remain reserved at the exact validity boundary. Used codes stay
	// available for replay detection and descendant-token revocation.
	if _, e = tx.Exec(ctx, "DELETE FROM oauth_jwt_ids WHERE expires_at<$1", now); e != nil {
		return v, e
	}
	v.Access, e = accessTokenValue(g.User.Tenant)
	if e != nil {
		return IssuedTokens{}, e
	}
	var code *string
	if g.CodeHash != "" {
		code = &g.CodeHash
	}
	if g.Refresh {
		v.Refresh, e = newSecret()
		if e != nil {
			return IssuedTokens{}, e
		}
		refreshHash = digest(v.Refresh)
		if _, e = tx.Exec(ctx, "INSERT INTO oauth_refresh_tokens(token_hash,client_id,user_id,scope,issued_at,code_hash) VALUES($1,$2,$3,$4,$5,$6)", refreshHash, g.ClientID, g.User.ID, g.Scope, now, code); e != nil {
			return IssuedTokens{}, e
		}
	}
	var refresh *string
	if refreshHash != "" {
		refresh = &refreshHash
	}
	_, e = tx.Exec(ctx, "INSERT INTO oauth_access_tokens(token_hash,client_id,user_id,scope,issued_at,expires_at,refresh_token_hash,code_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", digest(v.Access), g.ClientID, g.User.ID, g.Scope, now, now.Add(ttl), refresh, code)
	return v, e
}
func (s *Store) IssueTokens(ctx context.Context, g TokenGrant, now time.Time, ttl time.Duration) (IssuedTokens, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return IssuedTokens{}, e
	}
	defer tx.Rollback(ctx)
	u, e := lockTokenUser(ctx, tx, g.ClientID, g.User.ID)
	if e != nil {
		return IssuedTokens{}, e
	}
	if u.authVersion != g.User.authVersion {
		return IssuedTokens{}, grantError("invalid_grant")
	}
	g.User = u
	v, e := insertTokens(ctx, tx, g, now, ttl, "")
	if e != nil {
		return IssuedTokens{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return IssuedTokens{}, e
	}
	return v, nil
}
func (s *Store) IssueCode(ctx context.Context, in CodeInput, now time.Time) (string, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	u, e := lockTokenUser(ctx, tx, in.ClientID, in.UserID)
	if e != nil {
		return "", e
	}
	if in.loginVersion != nil && *in.loginVersion != u.authVersion {
		return "", grantError("invalid_grant")
	}
	code, e := newSecret()
	if e != nil {
		return "", e
	}
	_, e = tx.Exec(ctx, "INSERT INTO oauth_codes(code_hash,client_id,user_id,redirect_uri,scope,code_challenge,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)", digest(code), in.ClientID, in.UserID, in.RedirectURI, in.Scope, in.Challenge, now.Add(10*time.Minute))
	if e != nil {
		return "", e
	}
	if e = tx.Commit(ctx); e != nil {
		return "", e
	}
	return code, nil
}
func (s *Store) RedeemCode(ctx context.Context, code, clientID, redirectURI, verifier string, now time.Time, ttl time.Duration) (IssuedTokens, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return IssuedTokens{}, e
	}
	defer tx.Rollback(ctx)
	hash := digest(code)
	var client, user, redirect, scope, challenge string
	var expiry time.Time
	var used bool
	e = tx.QueryRow(ctx, "SELECT client_id,user_id,redirect_uri,scope,code_challenge,expires_at FROM oauth_codes WHERE code_hash=$1", hash).Scan(&client, &user, &redirect, &scope, &challenge, &expiry)
	if e != nil || client != clientID {
		return IssuedTokens{}, grantError("invalid_grant")
	}
	if redirect != redirectURI {
		return IssuedTokens{}, grantError("redirect_uri_mismatch")
	}
	if (challenge == "" && verifier != "") || (challenge != "" && !verifyPKCE(challenge, verifier)) {
		return IssuedTokens{}, grantError("invalid_grant")
	}
	u, e := lockTokenUser(ctx, tx, client, user)
	if e != nil {
		return IssuedTokens{}, e
	}
	e = tx.QueryRow(ctx, "UPDATE oauth_codes SET used=true WHERE code_hash=$1 AND NOT used RETURNING used", hash).Scan(&used)
	if errors.Is(e, pgx.ErrNoRows) {
		if _, e = tx.Exec(ctx, "DELETE FROM oauth_access_tokens WHERE code_hash=$1", hash); e != nil {
			return IssuedTokens{}, e
		}
		if _, e = tx.Exec(ctx, "DELETE FROM oauth_refresh_tokens WHERE code_hash=$1", hash); e != nil {
			return IssuedTokens{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return IssuedTokens{}, e
		}
		return IssuedTokens{}, grantError("invalid_grant")
	}
	if e != nil {
		return IssuedTokens{}, e
	}
	// Check expiry after the atomic claim so a previously used code still
	// invalidates its entire token family when replayed after its deadline.
	if !now.Before(expiry) {
		return IssuedTokens{}, grantError("invalid_grant")
	}
	refresh := false
	for _, v := range strings.Fields(scope) {
		refresh = refresh || v == "refresh_token" || v == "offline_access"
	}
	v, e := insertTokens(ctx, tx, TokenGrant{ClientID: client, User: u, Scope: scope, Refresh: refresh, CodeHash: hash}, now, ttl, "")
	if e != nil {
		return IssuedTokens{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return IssuedTokens{}, e
	}
	return v, nil
}
func (s *Store) RefreshAccess(ctx context.Context, refresh, clientID string, now time.Time, ttl time.Duration) (IssuedTokens, error) {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return IssuedTokens{}, e
	}
	defer tx.Rollback(ctx)
	hash := digest(refresh)
	var user, client, scope string
	var code *string
	e = tx.QueryRow(ctx, "SELECT client_id,user_id FROM oauth_refresh_tokens WHERE token_hash=$1", hash).Scan(&client, &user)
	if e != nil || client != clientID {
		return IssuedTokens{}, grantError("invalid_grant")
	}
	u, e := lockTokenUser(ctx, tx, client, user)
	if e != nil {
		return IssuedTokens{}, e
	}
	e = tx.QueryRow(ctx, "SELECT scope,code_hash FROM oauth_refresh_tokens WHERE token_hash=$1 FOR UPDATE", hash).Scan(&scope, &code)
	if e != nil {
		return IssuedTokens{}, grantError("invalid_grant")
	}
	g := TokenGrant{ClientID: client, User: u, Scope: scope}
	if code != nil {
		g.CodeHash = *code
	}
	v, e := insertTokens(ctx, tx, g, now, ttl, hash)
	if e != nil {
		return IssuedTokens{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return IssuedTokens{}, e
	}
	return v, nil
}
func (s *Store) RevokeToken(ctx context.Context, token string) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	hash := digest(token)
	var client, user, kind string
	e = tx.QueryRow(ctx, "SELECT client_id,user_id,'access' FROM oauth_access_tokens WHERE token_hash=$1 UNION ALL SELECT client_id,user_id,'refresh' FROM oauth_refresh_tokens WHERE token_hash=$1", hash).Scan(&client, &user, &kind)
	if errors.Is(e, pgx.ErrNoRows) {
		return errInvalidToken
	}
	if e != nil {
		return e
	}
	if _, e = lockTokenUser(ctx, tx, client, user); e != nil {
		return errInvalidToken
	}
	table := "oauth_access_tokens"
	if kind == "refresh" {
		table = "oauth_refresh_tokens"
	}
	tag, e := tx.Exec(ctx, "DELETE FROM "+table+" WHERE token_hash=$1", hash)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errInvalidToken
	}
	return tx.Commit(ctx)
}
func (s *Store) Introspect(ctx context.Context, token, clientID string, now time.Time) (Introspection, error) {
	var i Introspection
	var issued time.Time
	var expires *time.Time
	e := s.pool.QueryRow(ctx, `SELECT t.scope,t.client_id,u.username,u.id,t.kind,t.issued_at,t.expires_at FROM (SELECT token_hash,scope,client_id,user_id,'access_token' AS kind,issued_at,expires_at FROM oauth_access_tokens UNION ALL SELECT token_hash,scope,client_id,user_id,'refresh_token',issued_at,NULL::timestamptz FROM oauth_refresh_tokens) t JOIN users u ON u.id=t.user_id LEFT JOIN oauth_apps a ON a.client_id=t.client_id WHERE t.token_hash=$1 AND t.client_id=$2 AND u.active AND (t.expires_at IS NULL OR t.expires_at>$3) AND (t.client_id=$4 OR (a.active AND a.tenant_id=u.tenant_id))`, digest(token), clientID, now, PlatformCLIClientID).Scan(&i.Scope, &i.ClientID, &i.Username, &i.Sub, &i.TokenType, &issued, &expires)
	if errors.Is(e, pgx.ErrNoRows) {
		return Introspection{}, nil
	}
	if e != nil {
		return Introspection{}, e
	}
	i.Active = true
	i.Iat = issued.Unix()
	if expires != nil {
		i.Exp = expires.Unix()
	}
	return i, nil
}
