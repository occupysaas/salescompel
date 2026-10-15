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
