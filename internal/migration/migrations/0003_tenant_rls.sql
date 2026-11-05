-- Groups are bootstrapped by the DBA; no database passwords are created here.
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO salescompel_records,salescompel_identity,salescompel_owner;
GRANT USAGE ON SCHEMA sc_catalog,sc_records TO salescompel_records;
GRANT USAGE ON SCHEMA sc_control TO salescompel_records,salescompel_identity;
GRANT SELECT ON sc_control.schema_migrations TO salescompel_records,salescompel_identity;
GRANT SELECT ON public.principals,public.oauth_apps,public.oauth_app_jwt_users TO salescompel_identity;
-- PostgreSQL row locks require UPDATE on at least one column. App names are
-- display-only; runtime cannot alter active, secrets, grants or user bindings.
GRANT UPDATE(name) ON public.oauth_apps TO salescompel_identity;
GRANT SELECT,UPDATE ON public.users TO salescompel_identity;
GRANT SELECT,INSERT,UPDATE,DELETE ON public.oauth_requests,public.oauth_codes,public.oauth_refresh_tokens,public.oauth_access_tokens,public.oauth_jwt_ids TO salescompel_identity;
DO $$
DECLARE r record; perm text;
BEGIN
 FOR r IN SELECT n.nspname,c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND (n.nspname IN ('sc_catalog','sc_records') OR (n.nspname='public' AND c.relname IN ('accounts','contacts'))) LOOP
  EXECUTE format('ALTER TABLE %I.%I ENABLE ROW LEVEL SECURITY',r.nspname,r.relname);
  EXECUTE format('ALTER TABLE %I.%I FORCE ROW LEVEL SECURITY',r.nspname,r.relname);
  IF EXISTS(SELECT FROM pg_policies WHERE schemaname=r.nspname AND tablename=r.relname) THEN
   RAISE EXCEPTION 'unexpected pre-existing row policy; migration refused';
  END IF;
   EXECUTE format('CREATE POLICY tenant_isolation ON %I.%I USING(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),'''')) WITH CHECK(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),''''))',r.nspname,r.relname);
  IF r.nspname='sc_catalog' THEN perm:='SELECT'; ELSE perm:='SELECT,INSERT,UPDATE,DELETE'; END IF;
  EXECUTE format('GRANT %s ON %I.%I TO salescompel_records',perm,r.nspname,r.relname);
 END LOOP;
 FOR r IN SELECT n.nspname,c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind='r' AND n.nspname IN ('public','sc_catalog','sc_records','sc_control') LOOP
  EXECUTE format('ALTER TABLE %I.%I OWNER TO salescompel_owner',r.nspname,r.relname);
 END LOOP;
END $$;
ALTER SCHEMA sc_catalog OWNER TO salescompel_owner;
ALTER SCHEMA sc_records OWNER TO salescompel_owner;
ALTER SCHEMA sc_control OWNER TO salescompel_owner;
GRANT CREATE ON SCHEMA public TO salescompel_owner;
