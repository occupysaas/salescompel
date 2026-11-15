-- Plan 33: users, profiles, permission sets and org defaults. Immutable once
-- released. Non-secret User fields live in sc_setup; credentials stay in
-- public.users, which the records login never reads. The records login
-- reaches public.users only through the two SECURITY DEFINER functions below.
ALTER TABLE sc_catalog.settings
 ADD COLUMN time_zone_sid_key varchar(40) NOT NULL DEFAULT 'America/Los_Angeles',
 ADD COLUMN default_locale_sid_key varchar(40) NOT NULL DEFAULT 'en_US',
 ADD COLUMN language_locale_key varchar(40) NOT NULL DEFAULT 'en_US';

CREATE SCHEMA sc_setup;
ALTER SCHEMA sc_setup OWNER TO salescompel_owner;
GRANT USAGE ON SCHEMA sc_setup TO salescompel_records,salescompel_identity;

-- Bumped by every setup write; cached effective permissions are keyed by it.
CREATE TABLE sc_setup.revisions(tenant_id text PRIMARY KEY REFERENCES public.tenants(id) ON DELETE CASCADE,revision bigint NOT NULL);

CREATE TABLE sc_setup.profiles(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 name varchar(255) NOT NULL,
 description varchar(255),
 permission_set_id varchar(18) NOT NULL,
 PRIMARY KEY(tenant_id,id)
);
CREATE UNIQUE INDEX profile_names ON sc_setup.profiles(tenant_id,lower(name));

CREATE TABLE sc_setup.permission_sets(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 name varchar(80) NOT NULL,
 label varchar(80) NOT NULL,
 description varchar(255),
 is_owned_by_profile boolean NOT NULL DEFAULT false,
 profile_id varchar(18),
 permissions_modify_all_data boolean NOT NULL DEFAULT false,
 permissions_view_all_data boolean NOT NULL DEFAULT false,
 permissions_api_enabled boolean NOT NULL DEFAULT false,
 permissions_manage_users boolean NOT NULL DEFAULT false,
 -- Stored, not enforced Permissions* fields (D33-18); they grant nothing.
 extra_permissions jsonb NOT NULL DEFAULT '{}' CHECK(jsonb_typeof(extra_permissions)='object'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,profile_id),
 CHECK(is_owned_by_profile=(profile_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,profile_id) REFERENCES sc_setup.profiles(tenant_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX permission_set_names ON sc_setup.permission_sets(tenant_id,lower(name));
-- Each profile owns exactly one set; the pair is written in one transaction.
ALTER TABLE sc_setup.profiles ADD FOREIGN KEY(tenant_id,permission_set_id) REFERENCES sc_setup.permission_sets(tenant_id,id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE sc_setup.users(
 id varchar(18) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 username varchar(80) NOT NULL,
 username_canonical varchar(80) NOT NULL UNIQUE CHECK(username_canonical=lower(username)),
 first_name varchar(40),
 last_name varchar(80) NOT NULL,
 alias varchar(8) NOT NULL,
 email varchar(128) NOT NULL,
 is_active boolean NOT NULL DEFAULT true,
 profile_id varchar(18) NOT NULL,
 time_zone_sid_key varchar(40) NOT NULL,
 locale_sid_key varchar(40) NOT NULL,
 language_locale_key varchar(40) NOT NULL,
 email_encoding_key varchar(40) NOT NULL,
 PRIMARY KEY(tenant_id,id),
 FOREIGN KEY(tenant_id,profile_id) REFERENCES sc_setup.profiles(tenant_id,id)
);
CREATE INDEX users_profile ON sc_setup.users(tenant_id,profile_id);

-- Dependency checks back up the write path's D33-10 and D33-11 errors.
CREATE TABLE sc_setup.object_permissions(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 parent_id varchar(18) NOT NULL,
 sobject_type varchar(255) NOT NULL,
 permissions_read boolean NOT NULL DEFAULT false,
 permissions_create boolean NOT NULL DEFAULT false,
 permissions_edit boolean NOT NULL DEFAULT false,
 permissions_delete boolean NOT NULL DEFAULT false,
 permissions_view_all_records boolean NOT NULL DEFAULT false,
 permissions_modify_all_records boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,parent_id,sobject_type),
 FOREIGN KEY(tenant_id,parent_id) REFERENCES sc_setup.permission_sets(tenant_id,id) ON DELETE CASCADE,
 CHECK((NOT permissions_create OR permissions_read) AND (NOT permissions_edit OR permissions_read)
  AND (NOT permissions_delete OR (permissions_read AND permissions_edit))
  AND (NOT permissions_view_all_records OR permissions_read)
  AND (NOT permissions_modify_all_records OR (permissions_read AND permissions_edit AND permissions_delete AND permissions_view_all_records)))
);

CREATE TABLE sc_setup.field_permissions(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 parent_id varchar(18) NOT NULL,
 sobject_type varchar(255) NOT NULL,
 field varchar(511) NOT NULL CHECK(split_part(field,'.',1)=sobject_type),
 permissions_read boolean NOT NULL DEFAULT false,
 permissions_edit boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,parent_id,field),
 FOREIGN KEY(tenant_id,parent_id) REFERENCES sc_setup.permission_sets(tenant_id,id) ON DELETE CASCADE,
 CHECK(NOT permissions_edit OR permissions_read)
);

CREATE TABLE sc_setup.permission_set_assignments(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 assignee_id varchar(18) NOT NULL,
 permission_set_id varchar(18) NOT NULL,
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,assignee_id,permission_set_id),
 FOREIGN KEY(tenant_id,assignee_id) REFERENCES sc_setup.users(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,permission_set_id) REFERENCES sc_setup.permission_sets(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX permission_set_assignments_set ON sc_setup.permission_set_assignments(tenant_id,permission_set_id);

DO $$
DECLARE r text;
BEGIN
 FOREACH r IN ARRAY ARRAY['revisions','profiles','permission_sets','users','object_permissions','field_permissions','permission_set_assignments'] LOOP
  EXECUTE format('ALTER TABLE sc_setup.%I OWNER TO salescompel_owner',r);
  EXECUTE format('ALTER TABLE sc_setup.%I ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE sc_setup.%I FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('CREATE POLICY tenant_isolation ON sc_setup.%I USING(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),'''')) WITH CHECK(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),''''))',r);
  EXECUTE format('GRANT SELECT,INSERT,UPDATE,DELETE ON sc_setup.%I TO salescompel_records',r);
 END LOOP;
END $$;
-- Token and login checks need exactly these columns, never names or settings.
GRANT SELECT(id,tenant_id,is_active,profile_id) ON sc_setup.users TO salescompel_identity;

-- Salesforce ID suffix for a 15-character base (internal/recordid.Checksum).
CREATE FUNCTION sc_setup.id_checksum(p_base text) RETURNS text
 LANGUAGE plpgsql IMMUTABLE STRICT SET search_path = pg_catalog AS $$
DECLARE
 alphabet constant text := 'ABCDEFGHIJKLMNOPQRSTUVWXYZ012345';
 suffix text := '';
 bits int;
 code int;
BEGIN
 IF octet_length(p_base)<>15 THEN
  RETURN NULL;
 END IF;
 FOR g IN 0..2 LOOP
  bits := 0;
  FOR j IN 0..4 LOOP
   code := ascii(substr(p_base,g*5+j+1,1));
   IF code BETWEEN 65 AND 90 THEN
    bits := bits | (1<<j);
   END IF;
  END LOOP;
  suffix := suffix||substr(alphabet,bits+1,1);
 END LOOP;
 RETURN suffix;
END $$;

-- Rejects a call whose tenant isn't the transaction's tenant, a malformed
-- User ID, or a username that breaks Plan 26's rules (ASCII, one @, <= 80).
CREATE FUNCTION sc_setup.check_identity_args(p_id varchar,p_tenant text,p_username text) RETURNS void
 LANGUAGE plpgsql STABLE SET search_path = pg_catalog AS $$
BEGIN
 IF p_tenant IS NULL OR p_tenant IS DISTINCT FROM nullif(current_setting('salescompel.tenant_id',true),'') THEN
  RAISE EXCEPTION 'identity tenant does not match the transaction tenant' USING ERRCODE='42501';
 END IF;
 IF p_id IS NULL OR octet_length(p_id)<>18 OR p_id !~ '^005[[:alnum:]]{15}$' OR substr(p_id,16)<>sc_setup.id_checksum(left(p_id,15)) THEN
  RAISE EXCEPTION 'malformed User ID' USING ERRCODE='22023';
 END IF;
 IF p_username IS NULL OR octet_length(p_username)>80 OR octet_length(p_username)<>char_length(p_username) OR p_username !~ '^[^@]*@[^@]*$' THEN
  RAISE EXCEPTION 'invalid username' USING ERRCODE='22023';
 END IF;
END $$;

-- Creates a credential row with no usable password: login is refused until
-- the operator sets one. Returns nothing, so no credential data leaves.
CREATE FUNCTION sc_setup.create_identity(p_id varchar,p_tenant text,p_username text) RETURNS void
 LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
 PERFORM sc_setup.check_identity_args(p_id,p_tenant,p_username);
 IF EXISTS(SELECT FROM public.users WHERE username_canonical=lower(p_username)) THEN
  RAISE EXCEPTION 'duplicate username' USING ERRCODE='23505',CONSTRAINT='users_username_canonical_key';
 END IF;
 INSERT INTO public.users(id,tenant_id,username,username_canonical,password_hash,role)
  VALUES(p_id,p_tenant,p_username,lower(p_username),'','restricted');
END $$;

-- Renames a credential row in the caller's tenant.
CREATE FUNCTION sc_setup.rename_identity(p_id varchar,p_tenant text,p_username text) RETURNS void
 LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
 PERFORM sc_setup.check_identity_args(p_id,p_tenant,p_username);
 IF EXISTS(SELECT FROM public.users WHERE username_canonical=lower(p_username) AND id<>p_id) THEN
  RAISE EXCEPTION 'duplicate username' USING ERRCODE='23505',CONSTRAINT='users_username_canonical_key';
 END IF;
 UPDATE public.users SET username=p_username,username_canonical=lower(p_username) WHERE id=p_id AND tenant_id=p_tenant;
 IF NOT FOUND THEN
  RAISE EXCEPTION 'identity not found' USING ERRCODE='P0002';
 END IF;
END $$;

-- Seeding only; dropped below. Random part as in internal/recordid.New.
CREATE FUNCTION sc_setup.new_id(p_prefix text) RETURNS varchar
 LANGUAGE plpgsql VOLATILE SET search_path = pg_catalog AS $$
DECLARE
 alphabet constant text := 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
 -- Bytes 0-5 of two version-4 UUIDs are fully random.
 b bytea := substr(uuid_send(gen_random_uuid()),1,6)||substr(uuid_send(gen_random_uuid()),1,6);
 base text := p_prefix;
BEGIN
 FOR i IN 0..11 LOOP
  base := base||substr(alphabet,get_byte(b,i)%62+1,1);
 END LOOP;
 RETURN base||sc_setup.id_checksum(base);
END $$;

DO $$
DECLARE r text;
BEGIN
 FOREACH r IN ARRAY ARRAY['id_checksum(text)','check_identity_args(varchar,text,text)','create_identity(varchar,text,text)','rename_identity(varchar,text,text)','new_id(text)'] LOOP
  EXECUTE 'ALTER FUNCTION sc_setup.'||r||' OWNER TO salescompel_owner';
  EXECUTE 'REVOKE ALL ON FUNCTION sc_setup.'||r||' FROM PUBLIC';
 END LOOP;
END $$;
GRANT EXECUTE ON FUNCTION sc_setup.create_identity(varchar,text,text),sc_setup.rename_identity(varchar,text,text) TO salescompel_records;

-- Seed the four profiles for every existing tenant, per tenant context so
-- forced row security holds for a non-superuser operator. Standard User and
-- Read Only cover Account, Contact and every custom object existing now;
-- Standard User has no grant on Account.Compat_Text__c. Permissionable
-- fields match metadata.Permissionable. Keep in step with metadata.SeedTenant.
DO $$
DECLARE
 tid text;
 prof record;
 pid varchar;
 sid varchar;
BEGIN
 FOR tid IN SELECT id FROM public.tenants ORDER BY id LOOP
  PERFORM set_config('salescompel.tenant_id',tid,true);
  INSERT INTO sc_setup.revisions(tenant_id,revision) VALUES(tid,1);
  FOR prof IN SELECT * FROM (VALUES
   ('System Administrator',true,true,true,true,'none'),
   ('Standard User',false,false,true,false,'edit'),
   ('Read Only',false,false,true,false,'read'),
   ('Minimum Access',false,false,false,false,'none')) AS v(name,mad,vad,api,mu,access) LOOP
   pid := sc_setup.new_id('00e');
   sid := sc_setup.new_id('0PS');
   INSERT INTO sc_setup.profiles(id,tenant_id,name,permission_set_id) VALUES(pid,tid,prof.name,sid);
   INSERT INTO sc_setup.permission_sets(id,tenant_id,name,label,is_owned_by_profile,profile_id,permissions_modify_all_data,permissions_view_all_data,permissions_api_enabled,permissions_manage_users)
    VALUES(sid,tid,'X'||pid,prof.name,true,pid,prof.mad,prof.vad,prof.api,prof.mu);
   CONTINUE WHEN prof.access='none';
   INSERT INTO sc_setup.object_permissions(id,tenant_id,parent_id,sobject_type,permissions_read,permissions_create,permissions_edit,permissions_delete)
    SELECT sc_setup.new_id('110'),tid,sid,o.api_name,true,prof.access='edit',prof.access='edit',prof.access='edit'
    FROM sc_catalog.objects o WHERE o.tenant_id=tid AND (NOT o.standard OR o.api_name IN ('Account','Contact'));
   INSERT INTO sc_setup.field_permissions(id,tenant_id,parent_id,sobject_type,field,permissions_read,permissions_edit)
    SELECT sc_setup.new_id('01k'),tid,sid,o.api_name,o.api_name||'.'||f.api_name,true,
     prof.access='edit' AND (coalesce((f.definition->'field'->>'createable')::boolean,false) OR coalesce((f.definition->'field'->>'updateable')::boolean,false))
    FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.tenant_id=f.tenant_id AND o.id=f.object_id
    WHERE o.tenant_id=tid AND (NOT o.standard OR o.api_name IN ('Account','Contact'))
     AND coalesce((f.definition->'field'->>'nillable')::boolean,false)
     AND NOT coalesce((f.definition->'field'->>'nameField')::boolean,false)
     AND NOT coalesce((f.definition->'options'->>'generated')::boolean,false)
     AND lower(f.api_name) NOT IN ('id','createddate','createdbyid','lastmodifieddate','lastmodifiedbyid','systemmodstamp','isdeleted')
     AND NOT (prof.access='edit' AND o.api_name='Account' AND f.api_name='Compat_Text__c');
  END LOOP;
  -- admin becomes System Administrator and restricted Standard User. Name,
  -- alias and email come from the username; settings from the org defaults.
  INSERT INTO sc_setup.users(id,tenant_id,username,username_canonical,last_name,alias,email,is_active,profile_id,time_zone_sid_key,locale_sid_key,language_locale_key,email_encoding_key)
   SELECT u.id,u.tenant_id,u.username,lower(u.username),
    left(coalesce(nullif(split_part(u.username,'@',1),''),u.username),80),
    left(coalesce(nullif(split_part(u.username,'@',1),''),u.username),8),
    u.username,u.active,p.id,
    coalesce(s.time_zone_sid_key,'America/Los_Angeles'),coalesce(s.default_locale_sid_key,'en_US'),coalesce(s.language_locale_key,'en_US'),'UTF-8'
   FROM public.users u
   JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.name=CASE u.role WHEN 'admin' THEN 'System Administrator' ELSE 'Standard User' END
   LEFT JOIN sc_catalog.settings s ON s.tenant_id=u.tenant_id
   WHERE u.tenant_id=tid;
  -- Each user's system-maintained assignment to their profile's owned set.
  INSERT INTO sc_setup.permission_set_assignments(id,tenant_id,assignee_id,permission_set_id)
   SELECT sc_setup.new_id('0Pa'),tid,u.id,p.permission_set_id
   FROM sc_setup.users u JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id
   WHERE u.tenant_id=tid;
 END LOOP;
 PERFORM set_config('salescompel.tenant_id','',true);
END $$;
DROP FUNCTION sc_setup.new_id(text);
