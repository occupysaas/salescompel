-- Bootstrap retains v1 bounds; ensureCatalog validates and atomically upgrades to v2.
CREATE TABLE IF NOT EXISTS platform_catalog (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), version integer NOT NULL, definition jsonb NOT NULL
);
CREATE TABLE IF NOT EXISTS tenants (id text PRIMARY KEY);
CREATE TABLE IF NOT EXISTS principals (
 token_hash text PRIMARY KEY, tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 user_id text NOT NULL, role text NOT NULL CHECK(role IN ('admin','restricted'))
);
CREATE TABLE IF NOT EXISTS accounts (
 tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, id varchar(18) NOT NULL,
 name varchar(255) NOT NULL CHECK(length(trim(name))>0), compat_text varchar(255), compat_key varchar(255),
 PRIMARY KEY(tenant_id,id), UNIQUE(tenant_id,compat_key)
);
CREATE TABLE IF NOT EXISTS contacts (
 tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE, id varchar(18) NOT NULL,
 first_name varchar(255), last_name varchar(255) NOT NULL CHECK(length(trim(last_name))>0), account_id varchar(18),
 PRIMARY KEY(tenant_id,id), FOREIGN KEY(tenant_id,account_id) REFERENCES accounts(tenant_id,id)
);
CREATE INDEX IF NOT EXISTS contacts_parent ON contacts(tenant_id,account_id);

CREATE TABLE IF NOT EXISTS users (
 id varchar(18) PRIMARY KEY, tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 username text NOT NULL, username_canonical text NOT NULL UNIQUE, password_hash text NOT NULL,
 role text NOT NULL CHECK(role IN ('admin','restricted')), active boolean NOT NULL DEFAULT true,
 failed_attempts integer NOT NULL DEFAULT 0, locked_until timestamptz
);
CREATE TABLE IF NOT EXISTS oauth_apps (
 client_id text PRIMARY KEY, tenant_id text NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
 name text NOT NULL, secret_hash text NOT NULL DEFAULT '', public boolean NOT NULL DEFAULT false,
 active boolean NOT NULL DEFAULT true, redirect_uris text[] NOT NULL, grants text[] NOT NULL, scopes text[] NOT NULL,
 jwt_cert_pem text NOT NULL DEFAULT '', run_as_user_id varchar(18) REFERENCES users(id)
);
CREATE TABLE IF NOT EXISTS oauth_app_jwt_users (
 client_id text NOT NULL REFERENCES oauth_apps(client_id) ON DELETE CASCADE,
 user_id varchar(18) NOT NULL REFERENCES users(id) ON DELETE CASCADE, PRIMARY KEY(client_id,user_id)
);
CREATE TABLE IF NOT EXISTS oauth_requests (
 request_hash text PRIMARY KEY, client_id text NOT NULL, redirect_uri text NOT NULL, scope text NOT NULL,
 state text NOT NULL DEFAULT '', code_challenge text NOT NULL DEFAULT '', cookie_hash text NOT NULL,
 expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS oauth_codes (
 code_hash text PRIMARY KEY, client_id text NOT NULL, user_id varchar(18) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 redirect_uri text NOT NULL, scope text NOT NULL, code_challenge text NOT NULL DEFAULT '',
 expires_at timestamptz NOT NULL, used boolean NOT NULL DEFAULT false
);
CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
 token_hash text PRIMARY KEY, client_id text NOT NULL, user_id varchar(18) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope text NOT NULL, issued_at timestamptz NOT NULL, code_hash text REFERENCES oauth_codes(code_hash) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS oauth_access_tokens (
 token_hash text PRIMARY KEY, client_id text NOT NULL, user_id varchar(18) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 scope text NOT NULL, issued_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
 refresh_token_hash text REFERENCES oauth_refresh_tokens(token_hash) ON DELETE CASCADE,
 code_hash text REFERENCES oauth_codes(code_hash) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS oauth_jwt_ids (
 client_id text NOT NULL, jti text NOT NULL, expires_at timestamptz NOT NULL, PRIMARY KEY(client_id,jti)
);
CREATE INDEX IF NOT EXISTS oauth_access_user ON oauth_access_tokens(user_id);
CREATE INDEX IF NOT EXISTS oauth_refresh_user ON oauth_refresh_tokens(user_id);

-- Invalidate verified User snapshots when credentials change.
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_version bigint NOT NULL DEFAULT 0;
