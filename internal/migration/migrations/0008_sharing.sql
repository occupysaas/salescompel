-- Plan 34: record ownership and sharing. Immutable once released.
-- Every record table gets owner_id; UserRole, Group and GroupMember live in
-- sc_setup next to the Plan 33 setup objects; sc_sharing holds org-wide
-- defaults, manual shares, owner-based rules and the derived closure tables
-- the runtime rebuilds in the same transaction as any role, group or user
-- change (internal/records/sharing_closure.go). Access is evaluated at query
-- time; nothing here is a materialized share per record.

-- Owners. The column stays nullable: a tenant with records and no users has
-- nobody to own them (fail closed: such records are reached only through the
-- org-wide default and bypass permissions). The write path always sets it.
ALTER TABLE public.accounts ADD COLUMN owner_id varchar(18);
ALTER TABLE public.contacts ADD COLUMN owner_id varchar(18);
CREATE INDEX accounts_owner ON public.accounts(tenant_id,owner_id);
CREATE INDEX contacts_owner ON public.contacts(tenant_id,owner_id);
DO $$
DECLARE t text;
BEGIN
 FOR t IN SELECT DISTINCT physical_table FROM sc_catalog.objects WHERE NOT standard AND physical_schema='sc_records' LOOP
  EXECUTE format('ALTER TABLE sc_records.%I ADD COLUMN owner_id varchar(18)',t);
  EXECUTE format('CREATE INDEX ON sc_records.%I(tenant_id,owner_id)',t);
 END LOOP;
END $$;

-- Roles, groups and members (D34-12, D34-13, D34-15).
CREATE TABLE sc_setup.user_roles(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 name varchar(80) NOT NULL,
 developer_name varchar(80) NOT NULL,
 parent_role_id varchar(18),
 rollup_description varchar(80),
 contact_access_for_account_owner varchar(40) CHECK(contact_access_for_account_owner IN ('None','Read','Edit')),
 opportunity_access_for_account_owner varchar(40) CHECK(opportunity_access_for_account_owner IN ('None','Read','Edit')),
 case_access_for_account_owner varchar(40) CHECK(case_access_for_account_owner IN ('None','Read','Edit')),
 PRIMARY KEY(tenant_id,id),
 FOREIGN KEY(tenant_id,parent_role_id) REFERENCES sc_setup.user_roles(tenant_id,id),
 CHECK(parent_role_id IS DISTINCT FROM id)
);
CREATE UNIQUE INDEX user_role_developer_names ON sc_setup.user_roles(tenant_id,lower(developer_name));
CREATE INDEX user_roles_parent ON sc_setup.user_roles(tenant_id,parent_role_id);

CREATE TABLE sc_setup.groups(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 name varchar(40) NOT NULL,
 developer_name varchar(80),
 type varchar(40) NOT NULL CHECK(type IN ('Regular','Role','RoleAndSubordinates','RoleAndSubordinatesInternal','Organization')),
 related_id varchar(18),
 does_include_bosses boolean NOT NULL DEFAULT false,
 PRIMARY KEY(tenant_id,id),
 CHECK((type IN ('Role','RoleAndSubordinates','RoleAndSubordinatesInternal'))=(related_id IS NOT NULL)),
 FOREIGN KEY(tenant_id,related_id) REFERENCES sc_setup.user_roles(tenant_id,id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX group_role_types ON sc_setup.groups(tenant_id,related_id,type) WHERE related_id IS NOT NULL;
CREATE UNIQUE INDEX group_organization ON sc_setup.groups(tenant_id) WHERE type='Organization';
CREATE UNIQUE INDEX group_developer_names ON sc_setup.groups(tenant_id,lower(developer_name)) WHERE type='Regular' AND developer_name IS NOT NULL;

CREATE TABLE sc_setup.group_members(
 id varchar(18) NOT NULL,
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 group_id varchar(18) NOT NULL,
 user_or_group_id varchar(18) NOT NULL,
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,group_id,user_or_group_id),
 FOREIGN KEY(tenant_id,group_id) REFERENCES sc_setup.groups(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX group_members_member ON sc_setup.group_members(tenant_id,user_or_group_id);

ALTER TABLE sc_setup.users ADD COLUMN user_role_id varchar(18);
ALTER TABLE sc_setup.users ADD FOREIGN KEY(tenant_id,user_role_id) REFERENCES sc_setup.user_roles(tenant_id,id);
CREATE INDEX users_role ON sc_setup.users(tenant_id,user_role_id);

DO $$
DECLARE r text;
BEGIN
 FOREACH r IN ARRAY ARRAY['user_roles','groups','group_members'] LOOP
  EXECUTE format('ALTER TABLE sc_setup.%I OWNER TO salescompel_owner',r);
  EXECUTE format('ALTER TABLE sc_setup.%I ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE sc_setup.%I FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('CREATE POLICY tenant_isolation ON sc_setup.%I USING(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),'''')) WITH CHECK(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),''''))',r);
  EXECUTE format('GRANT SELECT,INSERT,UPDATE,DELETE ON sc_setup.%I TO salescompel_records',r);
 END LOOP;
END $$;

-- Sharing storage.
CREATE SCHEMA sc_sharing;
ALTER SCHEMA sc_sharing OWNER TO salescompel_owner;
GRANT USAGE ON SCHEMA sc_sharing TO salescompel_records;

-- Internal org-wide default per object (D34-16). No row means the
-- Salesforce new-org default: Contact ControlledByParent, others Edit.
CREATE TABLE sc_sharing.object_settings(
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 object_id text NOT NULL,
 internal_access varchar(40) NOT NULL CHECK(internal_access IN ('None','Read','Edit','ControlledByParent')),
 PRIMARY KEY(tenant_id,object_id),
 FOREIGN KEY(tenant_id,object_id) REFERENCES sc_catalog.objects(tenant_id,id) ON DELETE CASCADE
);

-- Manual shares (D34-06). Owner rows are computed by the share views.
CREATE TABLE sc_sharing.shares(
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 id varchar(18) NOT NULL,
 object_id text NOT NULL,
 record_id varchar(18) NOT NULL,
 user_or_group_id varchar(18) NOT NULL,
 access_level varchar(40) NOT NULL CHECK(access_level IN ('Read','Edit')),
 contact_access_level varchar(40) CHECK(contact_access_level IN ('None','Read','Edit')),
 opportunity_access_level varchar(40) CHECK(opportunity_access_level IN ('None','Read','Edit')),
 case_access_level varchar(40) CHECK(case_access_level IN ('None','Read','Edit')),
 row_cause varchar(40) NOT NULL DEFAULT 'Manual' CHECK(row_cause='Manual'),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,record_id,user_or_group_id,row_cause),
 FOREIGN KEY(tenant_id,object_id) REFERENCES sc_catalog.objects(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX shares_grantee ON sc_sharing.shares(tenant_id,user_or_group_id);

-- Owner-based sharing rules (D34-30), managed by salescompel sharing rule.
CREATE TABLE sc_sharing.owner_rules(
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 id varchar(18) NOT NULL,
 object_id text NOT NULL,
 name varchar(80) NOT NULL,
 source_group_id varchar(18) NOT NULL,
 target_group_id varchar(18) NOT NULL,
 access_level varchar(40) NOT NULL CHECK(access_level IN ('Read','Edit')),
 contact_access_level varchar(40) CHECK(contact_access_level IN ('None','Read','Edit')),
 PRIMARY KEY(tenant_id,id),
 UNIQUE(tenant_id,object_id,name),
 FOREIGN KEY(tenant_id,object_id) REFERENCES sc_catalog.objects(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,source_group_id) REFERENCES sc_setup.groups(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,target_group_id) REFERENCES sc_setup.groups(tenant_id,id) ON DELETE CASCADE
);

-- Derived closure. user_groups: every group an active user is in, directly,
-- through nesting, through role groups and the Organization group.
-- user_principals: for each active user, the IDs whose access they hold:
-- themselves, their groups, every user in a role below theirs and those
-- users' groups. A record is reachable through ownership when its owner is
-- one of the principals, and through a share or rule when the grantee is.
CREATE TABLE sc_sharing.user_groups(
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 user_id varchar(18) NOT NULL,
 group_id varchar(18) NOT NULL,
 PRIMARY KEY(tenant_id,user_id,group_id)
);
CREATE INDEX user_groups_group ON sc_sharing.user_groups(tenant_id,group_id);
CREATE TABLE sc_sharing.user_principals(
 tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
 user_id varchar(18) NOT NULL,
 principal_id varchar(18) NOT NULL,
 PRIMARY KEY(tenant_id,user_id,principal_id)
);

DO $$
DECLARE r text;
BEGIN
 FOREACH r IN ARRAY ARRAY['object_settings','shares','owner_rules','user_groups','user_principals'] LOOP
  EXECUTE format('ALTER TABLE sc_sharing.%I OWNER TO salescompel_owner',r);
  EXECUTE format('ALTER TABLE sc_sharing.%I ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE sc_sharing.%I FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('CREATE POLICY tenant_isolation ON sc_sharing.%I USING(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),'''')) WITH CHECK(tenant_id=nullif(current_setting(''salescompel.tenant_id'',true),''''))',r);
  EXECUTE format('GRANT SELECT,INSERT,UPDATE,DELETE ON sc_sharing.%I TO salescompel_records',r);
 END LOOP;
END $$;

-- Share objects: stored Manual rows plus one computed Owner row per owned
-- record (D34-06). The Owner row ID is the 00r/03s prefix, twelve hex digits
-- of md5(record id) and the checksum.
CREATE VIEW sc_setup.account_shares WITH (security_invoker=true) AS
 SELECT s.tenant_id,s.id,s.record_id AS account_id,s.user_or_group_id,s.access_level AS account_access_level,
  s.contact_access_level,s.opportunity_access_level,s.case_access_level,s.row_cause
 FROM sc_sharing.shares s JOIN sc_catalog.objects o ON o.tenant_id=s.tenant_id AND o.id=s.object_id AND o.api_name='Account'
 UNION ALL
 SELECT a.tenant_id,('00r'||left(md5(a.id),12)||sc_setup.id_checksum('00r'||left(md5(a.id),12)))::varchar(18),a.id,a.owner_id,'All','Edit','Edit','Edit','Owner'
 FROM public.accounts a WHERE a.owner_id IS NOT NULL;
CREATE VIEW sc_setup.contact_shares WITH (security_invoker=true) AS
 SELECT s.tenant_id,s.id,s.record_id AS contact_id,s.user_or_group_id,s.access_level AS contact_access_level,s.row_cause
 FROM sc_sharing.shares s JOIN sc_catalog.objects o ON o.tenant_id=s.tenant_id AND o.id=s.object_id AND o.api_name='Contact'
 UNION ALL
 SELECT c.tenant_id,('03s'||left(md5(c.id),12)||sc_setup.id_checksum('03s'||left(md5(c.id),12)))::varchar(18),c.id,c.owner_id,'All','Owner'
 FROM public.contacts c WHERE c.owner_id IS NOT NULL;
ALTER VIEW sc_setup.account_shares OWNER TO salescompel_owner;
ALTER VIEW sc_setup.contact_shares OWNER TO salescompel_owner;
GRANT SELECT ON sc_setup.account_shares,sc_setup.contact_shares TO salescompel_records;

-- Organization gains the Account and Contact defaults (D34-20).
CREATE OR REPLACE VIEW sc_setup.organization WITH (security_invoker=true) AS
 SELECT s.tenant_id,('00D'||encode(substr(sha256(convert_to(s.tenant_id,'UTF8')),1,6),'hex')||'EAA')::varchar(18) AS id,s.tenant_id::varchar(80) AS name,
  s.time_zone_sid_key,s.default_locale_sid_key,s.language_locale_key,s.currency_iso_code::text AS currency_iso_code,
  NULL::varchar(5) AS instance_name,false AS is_sandbox,NULL::timestamptz AS trial_expiration_date,
  NULL::varchar(15) AS namespace_prefix,'SalesCompel'::varchar(40) AS organization_type,
  coalesce((SELECT os.internal_access FROM sc_sharing.object_settings os JOIN sc_catalog.objects o ON o.tenant_id=os.tenant_id AND o.id=os.object_id WHERE os.tenant_id=s.tenant_id AND o.api_name='Account'),'Edit')::varchar(40) AS default_account_access,
  coalesce((SELECT os.internal_access FROM sc_sharing.object_settings os JOIN sc_catalog.objects o ON o.tenant_id=os.tenant_id AND o.id=os.object_id WHERE os.tenant_id=s.tenant_id AND o.api_name='Contact'),'ControlledByParent')::varchar(40) AS default_contact_access
 FROM sc_catalog.settings s;

-- Seeding only; dropped below. Random part as in internal/recordid.New.
CREATE FUNCTION sc_sharing.seed_id(p_prefix text) RETURNS varchar
 LANGUAGE sql VOLATILE SET search_path = pg_catalog AS $f$
 SELECT (p_prefix||b||sc_setup.id_checksum(p_prefix||b))::varchar FROM (SELECT string_agg(substr('0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ',(floor(random()*62))::int+1,1),'') AS b FROM generate_series(1,12)) g
$f$;

-- Per tenant: catalog rows (generated from internal/metadata; keep in step,
-- TestCatalogSeedParity), the Organization group, owner backfill to the
-- default owner (first active System Administrator by ID, else first active
-- user), and the initial closure (no roles or groups yet: each active user
-- is in the Organization group).
DO $$
DECLARE tid text; owner varchar(18); org varchar(18); t text;
BEGIN
 FOR tid IN SELECT id FROM public.tenants ORDER BY id LOOP
  PERFORM set_config('salescompel.tenant_id',tid,true);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Account:OwnerId'),tid,(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Account'),'OwnerId','owner_id','{"v":2,"field":{"name":"OwnerId","label":"Owner ID","type":"reference","soapType":"tns:ID","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["User"],"relationshipName":"Owner","relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":true,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":true,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Restrict"}}',(SELECT coalesce(max(ordinal),-1)+1 FROM sc_catalog.fields WHERE tenant_id=tid AND object_id=(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Account')));
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Contact:OwnerId'),tid,(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Contact'),'OwnerId','owner_id','{"v":2,"field":{"name":"OwnerId","label":"Owner ID","type":"reference","soapType":"tns:ID","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["User"],"relationshipName":"Owner","relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":true,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":true,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Restrict"}}',(SELECT coalesce(max(ordinal),-1)+1 FROM sc_catalog.fields WHERE tenant_id=tid AND object_id=(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Contact')));
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':User:UserRoleId'),tid,(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='User'),'UserRoleId','user_role_id','{"v":2,"field":{"name":"UserRoleId","label":"Role ID","type":"reference","soapType":"tns:ID","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["UserRole"],"relationshipName":"UserRole","relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":true,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Restrict","childRelationshipName":"Users"}}',(SELECT coalesce(max(ordinal),-1)+1 FROM sc_catalog.fields WHERE tenant_id=tid AND object_id=(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='User')));
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Organization:DefaultAccountAccess'),tid,(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Organization'),'DefaultAccountAccess','default_account_access','{"v":2,"field":{"name":"DefaultAccountAccess","label":"Default Account Access","type":"picklist","soapType":"xsd:string","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null},{"active":true,"defaultValue":false,"label":"ControlledByParent","value":"ControlledByParent","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',(SELECT coalesce(max(ordinal),-1)+1 FROM sc_catalog.fields WHERE tenant_id=tid AND object_id=(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Organization')));
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Organization:DefaultContactAccess'),tid,(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Organization'),'DefaultContactAccess','default_contact_access','{"v":2,"field":{"name":"DefaultContactAccess","label":"Default Contact Access","type":"picklist","soapType":"xsd:string","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null},{"active":true,"defaultValue":false,"label":"ControlledByParent","value":"ControlledByParent","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',(SELECT coalesce(max(ordinal),-1)+1 FROM sc_catalog.fields WHERE tenant_id=tid AND object_id=(SELECT id FROM sc_catalog.objects WHERE tenant_id=tid AND api_name='Organization')));
  INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES(md5('object:'||tid||':UserRole'),tid,'UserRole','Role','Roles',true,'sc_setup','user_roles','00E');
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:Id'),tid,md5('object:'||tid||':UserRole'),'Id','id','{"v":2,"field":{"name":"Id","label":"Id","type":"id","soapType":"tns:ID","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Id"}}',0);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:Name'),tid,md5('object:'||tid||':UserRole'),'Name','name','{"v":2,"field":{"name":"Name","label":"Name","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":80,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":true,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',1);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:DeveloperName'),tid,md5('object:'||tid||':UserRole'),'DeveloperName','developer_name','{"v":2,"field":{"name":"DeveloperName","label":"Developer Name","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":80,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',2);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:ParentRoleId'),tid,md5('object:'||tid||':UserRole'),'ParentRoleId','parent_role_id','{"v":2,"field":{"name":"ParentRoleId","label":"Parent Role ID","type":"reference","soapType":"tns:ID","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["UserRole"],"relationshipName":"ParentRole","relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":true,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Restrict","childRelationshipName":"ChildRoles"}}',3);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:RollupDescription'),tid,md5('object:'||tid||':UserRole'),'RollupDescription','rollup_description','{"v":2,"field":{"name":"RollupDescription","label":"Description","type":"string","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":80,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',4);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:ContactAccessForAccountOwner'),tid,md5('object:'||tid||':UserRole'),'ContactAccessForAccountOwner','contact_access_for_account_owner','{"v":2,"field":{"name":"ContactAccessForAccountOwner","label":"Contact Access Level for Account Owner","type":"picklist","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',5);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:OpportunityAccessForAccountOwner'),tid,md5('object:'||tid||':UserRole'),'OpportunityAccessForAccountOwner','opportunity_access_for_account_owner','{"v":2,"field":{"name":"OpportunityAccessForAccountOwner","label":"Opportunity Access Level for Account Owner","type":"picklist","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',6);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':UserRole:CaseAccessForAccountOwner'),tid,md5('object:'||tid||':UserRole'),'CaseAccessForAccountOwner','case_access_for_account_owner','{"v":2,"field":{"name":"CaseAccessForAccountOwner","label":"Case Access Level for Account Owner","type":"picklist","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',7);
  INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES(md5('object:'||tid||':Group'),tid,'Group','Group','Groups',true,'sc_setup','groups','00G');
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Group:Id'),tid,md5('object:'||tid||':Group'),'Id','id','{"v":2,"field":{"name":"Id","label":"Id","type":"id","soapType":"tns:ID","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Id"}}',0);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Group:Name'),tid,md5('object:'||tid||':Group'),'Name','name','{"v":2,"field":{"name":"Name","label":"Name","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":true,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',1);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Group:DeveloperName'),tid,md5('object:'||tid||':Group'),'DeveloperName','developer_name','{"v":2,"field":{"name":"DeveloperName","label":"Developer Name","type":"string","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":80,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',2);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Group:Type'),tid,md5('object:'||tid||':Group'),'Type','type','{"v":2,"field":{"name":"Type","label":"Type","type":"picklist","soapType":"xsd:string","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"Regular","value":"Regular","validFor":null},{"active":true,"defaultValue":false,"label":"Role","value":"Role","validFor":null},{"active":true,"defaultValue":false,"label":"RoleAndSubordinates","value":"RoleAndSubordinates","validFor":null},{"active":true,"defaultValue":false,"label":"RoleAndSubordinatesInternal","value":"RoleAndSubordinatesInternal","validFor":null},{"active":true,"defaultValue":false,"label":"Organization","value":"Organization","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',3);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Group:RelatedId'),tid,md5('object:'||tid||':Group'),'RelatedId','related_id','{"v":2,"field":{"name":"RelatedId","label":"Related ID","type":"string","soapType":"xsd:string","nillable":true,"createable":false,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',4);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':Group:DoesIncludeBosses'),tid,md5('object:'||tid||':Group'),'DoesIncludeBosses','does_include_bosses','{"v":2,"field":{"name":"DoesIncludeBosses","label":"Include Bosses","type":"boolean","soapType":"xsd:boolean","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":false,"defaultedOnCreate":true,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Checkbox"}}',5);
  INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES(md5('object:'||tid||':GroupMember'),tid,'GroupMember','Group Member','Group Members',true,'sc_setup','group_members','011');
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':GroupMember:Id'),tid,md5('object:'||tid||':GroupMember'),'Id','id','{"v":2,"field":{"name":"Id","label":"Id","type":"id","soapType":"tns:ID","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Id"}}',0);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':GroupMember:GroupId'),tid,md5('object:'||tid||':GroupMember'),'GroupId','group_id','{"v":2,"field":{"name":"GroupId","label":"Group ID","type":"reference","soapType":"tns:ID","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["Group"],"relationshipName":"Group","relationshipOrder":null,"cascadeDelete":true,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Cascade","childRelationshipName":"GroupMembers"}}',1);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':GroupMember:UserOrGroupId'),tid,md5('object:'||tid||':GroupMember'),'UserOrGroupId','user_or_group_id','{"v":2,"field":{"name":"UserOrGroupId","label":"User/Group ID","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',2);
  INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES(md5('object:'||tid||':AccountShare'),tid,'AccountShare','Account Share','Account Share',true,'sc_setup','account_shares','00r');
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:Id'),tid,md5('object:'||tid||':AccountShare'),'Id','id','{"v":2,"field":{"name":"Id","label":"Id","type":"id","soapType":"tns:ID","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Id"}}',0);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:AccountId'),tid,md5('object:'||tid||':AccountShare'),'AccountId','account_id','{"v":2,"field":{"name":"AccountId","label":"Account ID","type":"reference","soapType":"tns:ID","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["Account"],"relationshipName":"Account","relationshipOrder":null,"cascadeDelete":true,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Cascade","childRelationshipName":"Shares"}}',1);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:UserOrGroupId'),tid,md5('object:'||tid||':AccountShare'),'UserOrGroupId','user_or_group_id','{"v":2,"field":{"name":"UserOrGroupId","label":"User/Group ID","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',2);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:AccountAccessLevel'),tid,md5('object:'||tid||':AccountShare'),'AccountAccessLevel','account_access_level','{"v":2,"field":{"name":"AccountAccessLevel","label":"Account Access","type":"picklist","soapType":"xsd:string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null},{"active":true,"defaultValue":false,"label":"All","value":"All","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',3);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:RowCause'),tid,md5('object:'||tid||':AccountShare'),'RowCause','row_cause','{"v":2,"field":{"name":"RowCause","label":"Row Cause","type":"picklist","soapType":"xsd:string","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"Owner","value":"Owner","validFor":null},{"active":true,"defaultValue":false,"label":"Manual","value":"Manual","validFor":null},{"active":true,"defaultValue":false,"label":"Rule","value":"Rule","validFor":null},{"active":true,"defaultValue":false,"label":"ImplicitChild","value":"ImplicitChild","validFor":null},{"active":true,"defaultValue":false,"label":"ImplicitParent","value":"ImplicitParent","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',4);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:ContactAccessLevel'),tid,md5('object:'||tid||':AccountShare'),'ContactAccessLevel','contact_access_level','{"v":2,"field":{"name":"ContactAccessLevel","label":"Contact Access","type":"picklist","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',5);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:OpportunityAccessLevel'),tid,md5('object:'||tid||':AccountShare'),'OpportunityAccessLevel','opportunity_access_level','{"v":2,"field":{"name":"OpportunityAccessLevel","label":"Opportunity Access","type":"picklist","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',6);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':AccountShare:CaseAccessLevel'),tid,md5('object:'||tid||':AccountShare'),'CaseAccessLevel','case_access_level','{"v":2,"field":{"name":"CaseAccessLevel","label":"Case Access","type":"picklist","soapType":"xsd:string","nillable":true,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"None","value":"None","validFor":null},{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',7);
  INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES(md5('object:'||tid||':ContactShare'),tid,'ContactShare','Contact Share','Contact Share',true,'sc_setup','contact_shares','03s');
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':ContactShare:Id'),tid,md5('object:'||tid||':ContactShare'),'Id','id','{"v":2,"field":{"name":"Id","label":"Id","type":"id","soapType":"tns:ID","nillable":false,"createable":false,"updateable":false,"externalId":false,"length":0,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Id"}}',0);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':ContactShare:ContactId'),tid,md5('object:'||tid||':ContactShare'),'ContactId','contact_id','{"v":2,"field":{"name":"ContactId","label":"Contact ID","type":"reference","soapType":"tns:ID","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["Contact"],"relationshipName":"Contact","relationshipOrder":null,"cascadeDelete":true,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Cascade","childRelationshipName":"Shares"}}',1);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':ContactShare:UserOrGroupId'),tid,md5('object:'||tid||':ContactShare'),'UserOrGroupId','user_or_group_id','{"v":2,"field":{"name":"UserOrGroupId","label":"User/Group ID","type":"string","soapType":"xsd:string","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Text"}}',2);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':ContactShare:ContactAccessLevel'),tid,md5('object:'||tid||':ContactShare'),'ContactAccessLevel','contact_access_level','{"v":2,"field":{"name":"ContactAccessLevel","label":"Contact Access","type":"picklist","soapType":"xsd:string","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"Read","value":"Read","validFor":null},{"active":true,"defaultValue":false,"label":"Edit","value":"Edit","validFor":null},{"active":true,"defaultValue":false,"label":"All","value":"All","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',3);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal) VALUES(md5('field:'||tid||':ContactShare:RowCause'),tid,md5('object:'||tid||':ContactShare'),'RowCause','row_cause','{"v":2,"field":{"name":"RowCause","label":"Row Cause","type":"picklist","soapType":"xsd:string","nillable":false,"createable":true,"updateable":false,"externalId":false,"length":40,"precision":0,"scale":0,"digits":0,"referenceTo":[],"relationshipName":null,"relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":false,"picklistValues":[{"active":true,"defaultValue":false,"label":"Owner","value":"Owner","validFor":null},{"active":true,"defaultValue":false,"label":"Manual","value":"Manual","validFor":null},{"active":true,"defaultValue":false,"label":"Rule","value":"Rule","validFor":null},{"active":true,"defaultValue":false,"label":"ImplicitChild","value":"ImplicitChild","validFor":null},{"active":true,"defaultValue":false,"label":"ImplicitParent","value":"ImplicitParent","validFor":null}],"restrictedPicklist":true,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":false,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Picklist"}}',4);
  INSERT INTO sc_catalog.fields(id,tenant_id,object_id,api_name,physical_column,definition,ordinal)
   SELECT md5('field:'||tid||':'||o.api_name||':OwnerId'),tid,o.id,'OwnerId','owner_id','{"v":2,"field":{"name":"OwnerId","label":"Owner ID","type":"reference","soapType":"tns:ID","nillable":false,"createable":true,"updateable":true,"externalId":false,"length":18,"precision":0,"scale":0,"digits":0,"referenceTo":["User"],"relationshipName":"Owner","relationshipOrder":null,"cascadeDelete":false,"restrictedDelete":true,"picklistValues":[],"restrictedPicklist":false,"dependentPicklist":false,"autoNumber":false,"calculated":false,"defaultValue":null,"defaultedOnCreate":true,"nameField":false,"compoundFieldName":null,"htmlFormatted":false,"filterable":true,"sortable":true,"groupable":true,"unique":false},"options":{"kind":"Lookup","deleteBehavior":"Restrict"}}',(SELECT coalesce(max(ordinal),-1)+1 FROM sc_catalog.fields f WHERE f.tenant_id=tid AND f.object_id=o.id)
   FROM sc_catalog.objects o WHERE o.tenant_id=tid AND NOT o.standard AND o.physical_schema='sc_records';
  org := sc_sharing.seed_id('00G');
  INSERT INTO sc_setup.groups(id,tenant_id,name,type) VALUES(org,tid,'All Internal Users','Organization');
  SELECT u.id INTO owner FROM sc_setup.users u JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id
   WHERE u.tenant_id=tid AND u.is_active ORDER BY (p.name='System Administrator') DESC,u.id LIMIT 1;
  IF owner IS NOT NULL THEN
   UPDATE public.accounts SET owner_id=owner WHERE tenant_id=tid;
   UPDATE public.contacts SET owner_id=owner WHERE tenant_id=tid;
   FOR t IN SELECT physical_table FROM sc_catalog.objects WHERE tenant_id=tid AND NOT standard AND physical_schema='sc_records' LOOP
    EXECUTE format('UPDATE sc_records.%I SET owner_id=$1 WHERE tenant_id=$2',t) USING owner,tid;
   END LOOP;
  END IF;
  INSERT INTO sc_sharing.user_groups(tenant_id,user_id,group_id) SELECT tid,u.id,org FROM sc_setup.users u WHERE u.tenant_id=tid AND u.is_active;
  INSERT INTO sc_sharing.user_principals(tenant_id,user_id,principal_id)
   SELECT tid,u.id,u.id FROM sc_setup.users u WHERE u.tenant_id=tid AND u.is_active
   UNION SELECT tid,u.id,org FROM sc_setup.users u WHERE u.tenant_id=tid AND u.is_active;
 END LOOP;
END $$;
DROP FUNCTION sc_sharing.seed_id(text);
