package platform

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/url"
	"salescompel/internal/metadata"
	"strings"
	"time"
	"unicode/utf8"
)

const PlatformCLIClientID = "PlatformCLI"

var errBadCredentials = errors.New("invalid credentials")

type User struct {
	ID, Tenant, Username, Role string
	Active                     bool
	authVersion                int64
}
type AppInput struct {
	Tenant, Name                           string
	Public                                 bool
	RedirectURIs, Grants, Scopes, JWTUsers []string
	JWTCertPEM, RunAs                      string
}
type OAuthApp struct {
	ClientID, Tenant, Name, SecretHash, JWTCertPEM, RunAsUserID string
	Public, Active                                              bool
	RedirectURIs, Grants, Scopes                                []string
}

func validUsername(u string) bool {
	if len(u) > 80 || strings.Count(u, "@") != 1 {
		return false
	}
	for _, c := range u {
		if c > 127 {
			return false
		}
	}
	return true
}
func validPassword(p string) bool {
	return utf8.ValidString(p) && utf8.RuneCountInString(p) >= 12 && len(p) <= 1024
}
func (s *Store) passwordSlot(ctx context.Context) (func(), error) {
	select {
	case s.passwordWork <- struct{}{}:
		return func() { <-s.passwordWork }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (s *Store) passwordHash(ctx context.Context, p string) (string, error) {
	release, e := s.passwordSlot(ctx)
	if e != nil {
		return "", e
	}
	defer release()
	return hashPassword(p)
}
func (s *Store) CreateUser(ctx context.Context, tenant, username, password, role string) (string, error) {
	if tenant == "" || !validUsername(username) || !validPassword(password) || (role != "admin" && role != "restricted") {
		return "", errors.New("invalid user configuration")
	}
	h, e := s.passwordHash(ctx, password)
	if e != nil {
		return "", e
	}
	random, e := newSecret()
	if e != nil {
		return "", e
	}
	base := "005" + random[:12]
	id := base + idChecksum(base)
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "INSERT INTO tenants(id) VALUES($1) ON CONFLICT DO NOTHING", tenant); e != nil {
		return "", e
	}
	if e = metadata.SeedTenant(ctx, tx, tenant); e != nil {
		return "", e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO users(id,tenant_id,username,username_canonical,password_hash,role) VALUES($1,$2,$3,$4,$5,$6)", id, tenant, username, strings.ToLower(username), h, role); e != nil {
		return "", errors.New("user could not be created; username may already exist")
	}
	return id, tx.Commit(ctx)
}
func deleteUserTokens(ctx context.Context, tx pgx.Tx, id string) error {
	for _, table := range []string{"oauth_access_tokens", "oauth_refresh_tokens", "oauth_codes"} {
		if _, e := tx.Exec(ctx, "DELETE FROM "+table+" WHERE user_id=$1", id); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) changeUser(ctx context.Context, username, password string, deactivate bool) error {
	h := ""
	var e error
	if !deactivate {
		if !validPassword(password) {
			return errors.New("invalid password length")
		}
		h, e = s.passwordHash(ctx, password)
		if e != nil {
			return e
		}
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var id string
	if deactivate {
		e = tx.QueryRow(ctx, "UPDATE users SET active=false WHERE username_canonical=$1 RETURNING id", strings.ToLower(username)).Scan(&id)
	} else {
		e = tx.QueryRow(ctx, "UPDATE users SET auth_version=auth_version+1,password_hash=$2,failed_attempts=0,locked_until=NULL WHERE username_canonical=$1 RETURNING id", strings.ToLower(username), h).Scan(&id)
	}
	if e != nil {
		return errors.New("user could not be updated")
	}
	if e = deleteUserTokens(ctx, tx, id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Store) SetPassword(ctx context.Context, u, p string) error {
	return s.changeUser(ctx, u, p, false)
}
func (s *Store) DeactivateUser(ctx context.Context, u string) error {
	return s.changeUser(ctx, u, "", true)
}
func (s *Store) VerifyLogin(ctx context.Context, username, password string, now time.Time) (User, error) {
	var u User
	if len(password) > 1024 {
		return u, errBadCredentials
	}
	release, e := s.passwordSlot(ctx)
	if e != nil {
		return u, errBadCredentials
	}
	defer release()
	var h string
	e = s.pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE username_canonical=$1", strings.ToLower(username)).Scan(&h)
	if e != nil {
		verifyDummyPassword(password)
		return u, errBadCredentials
	}
	valid := verifyPassword(h, password)
	// Hash outside the transaction, then serialize attempt accounting and recheck the hash.
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return u, errBadCredentials
	}
	defer tx.Rollback(ctx)
	var current string
	var failed int
	var locked *time.Time
	e = tx.QueryRow(ctx, "SELECT id,tenant_id,username,role,active,password_hash,failed_attempts,locked_until,auth_version FROM users WHERE username_canonical=$1 FOR UPDATE", strings.ToLower(username)).Scan(&u.ID, &u.Tenant, &u.Username, &u.Role, &u.Active, &current, &failed, &locked, &u.authVersion)
	if e != nil || !u.Active || current != h {
		return User{}, errBadCredentials
	}
	if locked != nil && now.Before(*locked) {
		return User{}, errBadCredentials
	}
	if !valid {
		if locked != nil {
			failed = 0
		}
		failed++
		var until *time.Time
		if failed >= 10 {
			v := now.Add(15 * time.Minute)
			until = &v
		}
		_, e = tx.Exec(ctx, "UPDATE users SET failed_attempts=$2,locked_until=$3 WHERE id=$1", u.ID, failed, until)
		if e == nil {
			e = tx.Commit(ctx)
		}
		return User{}, errBadCredentials
	}
	if _, e = tx.Exec(ctx, "UPDATE users SET failed_attempts=0,locked_until=NULL WHERE id=$1", u.ID); e != nil {
		return User{}, errBadCredentials
	}
	if e = tx.Commit(ctx); e != nil {
		return User{}, errBadCredentials
	}
	return u, nil
}
func validRedirect(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}
func (s *Store) CreateApp(ctx context.Context, in AppInput, now time.Time) (string, string, error) {
	bad := errors.New("invalid app configuration")
	if in.Tenant == "" || strings.TrimSpace(in.Name) == "" {
		return "", "", bad
	}
	for _, u := range in.RedirectURIs {
		if !validRedirect(u) {
			return "", "", bad
		}
	}
	for _, g := range in.Grants {
		if g != "authorization_code" && g != "refresh_token" && g != "client_credentials" && g != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			return "", "", bad
		}
		if g == "client_credentials" && in.Public {
			return "", "", bad
		}
		if g == "urn:ietf:params:oauth:grant-type:jwt-bearer" && in.JWTCertPEM == "" {
			return "", "", bad
		}
	}
	for _, scope := range in.Scopes {
		switch scope {
		case "api", "full", "id", "profile", "email", "address", "phone", "web", "refresh_token", "offline_access":
		default:
			return "", "", bad
		}
	}
	if in.JWTCertPEM != "" {
		if _, e := parseJWTCertificate(in.JWTCertPEM, now); e != nil {
			return "", "", bad
		}
	}
	id, e := newSecret()
	if e != nil {
		return "", "", e
	}
	secret, h := "", ""
	if !in.Public {
		secret, e = newSecret()
		if e != nil {
			return "", "", e
		}
		h = digest(secret)
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return "", "", e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "INSERT INTO tenants(id) VALUES($1) ON CONFLICT DO NOTHING", in.Tenant); e != nil {
		return "", "", e
	}
	lookup := func(name string) (string, error) {
		var user string
		e := tx.QueryRow(ctx, "SELECT id FROM users WHERE username_canonical=$1 AND tenant_id=$2 AND active", strings.ToLower(name), in.Tenant).Scan(&user)
		if e != nil {
			return "", bad
		}
		return user, nil
	}
	var runAs *string
	if in.RunAs != "" {
		v, e := lookup(in.RunAs)
		if e != nil {
			return "", "", e
		}
		runAs = &v
	}
	ids := []string{}
	for _, name := range in.JWTUsers {
		v, e := lookup(name)
		if e != nil {
			return "", "", e
		}
		ids = append(ids, v)
	}
	if in.RedirectURIs == nil {
		in.RedirectURIs = []string{}
	}
	if in.Grants == nil {
		in.Grants = []string{}
	}
	if in.Scopes == nil {
		in.Scopes = []string{}
	}
	_, e = tx.Exec(ctx, "INSERT INTO oauth_apps(client_id,tenant_id,name,secret_hash,public,redirect_uris,grants,scopes,jwt_cert_pem,run_as_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", id, in.Tenant, in.Name, h, in.Public, in.RedirectURIs, in.Grants, in.Scopes, in.JWTCertPEM, runAs)
	if e != nil {
		return "", "", e
	}
	for _, user := range ids {
		if _, e = tx.Exec(ctx, "INSERT INTO oauth_app_jwt_users VALUES($1,$2) ON CONFLICT DO NOTHING", id, user); e != nil {
			return "", "", e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return "", "", e
	}
	return id, secret, nil
}
func (s *Store) LookupApp(ctx context.Context, id string) (OAuthApp, error) {
	if id == PlatformCLIClientID {
		return OAuthApp{ClientID: id, Name: id, Public: true, Active: true, RedirectURIs: []string{"http://localhost:1717/OauthRedirect"}, Grants: []string{"authorization_code", "refresh_token"}, Scopes: []string{"api", "web", "refresh_token"}}, nil
	}
	var a OAuthApp
	e := s.pool.QueryRow(ctx, "SELECT client_id,tenant_id,name,secret_hash,jwt_cert_pem,COALESCE(run_as_user_id,''),public,active,redirect_uris,grants,scopes FROM oauth_apps WHERE client_id=$1", id).Scan(&a.ClientID, &a.Tenant, &a.Name, &a.SecretHash, &a.JWTCertPEM, &a.RunAsUserID, &a.Public, &a.Active, &a.RedirectURIs, &a.Grants, &a.Scopes)
	return a, e
}
func (s *Store) RotateAppSecret(ctx context.Context, id string) (string, error) {
	secret, e := newSecret()
	if e != nil {
		return "", e
	}
	tag, e := s.pool.Exec(ctx, "UPDATE oauth_apps SET secret_hash=$2 WHERE client_id=$1 AND NOT public", id, digest(secret))
	if e != nil {
		return "", e
	}
	if tag.RowsAffected() != 1 {
		return "", errors.New("confidential app required")
	}
	return secret, nil
}
func (s *Store) DisableApp(ctx context.Context, id string) error {
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE oauth_apps SET active=false WHERE client_id=$1", id)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("app not found")
	}
	for _, table := range []string{"oauth_access_tokens", "oauth_refresh_tokens", "oauth_codes", "oauth_requests", "oauth_jwt_ids"} {
		if _, e = tx.Exec(ctx, "DELETE FROM "+table+" WHERE client_id=$1", id); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) JWTUserAllowed(ctx context.Context, clientID, username string) (User, error) {
	var u User
	e := s.pool.QueryRow(ctx, "SELECT u.id,u.tenant_id,u.username,u.role,u.active,u.auth_version FROM users u JOIN oauth_app_jwt_users j ON j.user_id=u.id JOIN oauth_apps a ON a.client_id=j.client_id WHERE j.client_id=$1 AND u.username_canonical=$2 AND u.active AND a.active AND a.tenant_id=u.tenant_id", clientID, strings.ToLower(username)).Scan(&u.ID, &u.Tenant, &u.Username, &u.Role, &u.Active, &u.authVersion)
	return u, e
}
