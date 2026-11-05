CREATE SCHEMA sc_catalog;
CREATE SCHEMA sc_records;
CREATE TABLE sc_catalog.revisions(tenant_id text PRIMARY KEY REFERENCES public.tenants(id) ON DELETE CASCADE,revision bigint NOT NULL);
CREATE TABLE sc_catalog.objects(id text PRIMARY KEY,tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,api_name text NOT NULL,label text NOT NULL,plural_label text NOT NULL,standard boolean NOT NULL,physical_schema text NOT NULL,physical_table text NOT NULL,prefix varchar(3) NOT NULL,UNIQUE(tenant_id,id),UNIQUE(tenant_id,prefix));
CREATE UNIQUE INDEX object_names ON sc_catalog.objects(tenant_id,lower(api_name));
CREATE TABLE sc_catalog.fields(id text PRIMARY KEY,tenant_id text NOT NULL,object_id text NOT NULL,api_name text NOT NULL,physical_column text NOT NULL,definition jsonb NOT NULL,ordinal integer NOT NULL,FOREIGN KEY(tenant_id,object_id) REFERENCES sc_catalog.objects(tenant_id,id) ON DELETE CASCADE);
CREATE UNIQUE INDEX field_names ON sc_catalog.fields(tenant_id,object_id,lower(api_name));
CREATE TABLE sc_catalog.artifacts(id text PRIMARY KEY,tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,source bytea NOT NULL,active boolean NOT NULL DEFAULT false CHECK(NOT active));
