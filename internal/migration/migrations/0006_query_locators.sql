-- Plan 29: server-side query locators for paged SOQL results. Immutable once
-- released. Membership is the ordered ID array fixed at the first page; the
-- plan holds canonical query text and the catalog revision, and is re-parsed
-- and re-bound per page. Expiry is two days; the runtime role gets no UPDATE.
CREATE SCHEMA sc_query;
ALTER SCHEMA sc_query OWNER TO salescompel_owner;
GRANT USAGE ON SCHEMA sc_query TO salescompel_records;

CREATE TABLE sc_query.locators(
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 id text NOT NULL,
 user_id text NOT NULL,
 query_sha256 text NOT NULL,
 plan jsonb NOT NULL,
 ids text[] NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,id)
);
ALTER TABLE sc_query.locators OWNER TO salescompel_owner;
ALTER TABLE sc_query.locators ENABLE ROW LEVEL SECURITY;
ALTER TABLE sc_query.locators FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON sc_query.locators
 USING(tenant_id=nullif(current_setting('salescompel.tenant_id',true),''))
 WITH CHECK(tenant_id=nullif(current_setting('salescompel.tenant_id',true),''));
GRANT SELECT,INSERT,DELETE ON sc_query.locators TO salescompel_records;
CREATE INDEX locators_user ON sc_query.locators(tenant_id,user_id,created_at);
