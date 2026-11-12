-- Plan 29: child relationship names for lookup and master-detail fields.
-- Immutable once released. Names match DefaultChildRelationshipName: the
-- child plural label forced to ASCII (non-ASCII removed first so collation-
-- sensitive classes and case-folded bracket ranges cannot keep accented or
-- look-alike letters, then every non-alphanumeric), plus "__r" on custom
-- children. When that is not a valid identifier (empty or leading-digit
-- after stripping, or longer than 40 characters), the mint falls back to
-- "Rel" plus the field api_name stripped the same way and truncated to 40
-- (FallbackChildRelationshipName), so every minted name matches
-- ^[A-Za-z][A-Za-z0-9_]{0,39}$. Collisions on the same parent take a numeric
-- suffix on the full name (Widgets__r, Widgets__r2), truncating the base when
-- the suffix would exceed the cap.
DO $$
DECLARE tid text; r record; base text; nm text; sfx text; n int;
BEGIN
 FOR tid IN SELECT id FROM public.tenants ORDER BY id LOOP
  PERFORM set_config('salescompel.tenant_id',tid,true);
  CONTINUE WHEN NOT EXISTS(SELECT FROM sc_catalog.revisions WHERE tenant_id=tid);
  FOR r IN
   SELECT f.id AS fid, f.api_name AS fname, o.api_name AS child, o.plural_label AS plural,
          f.definition->'field'->'referenceTo'->>0 AS parent
   FROM sc_catalog.fields f JOIN sc_catalog.objects o ON o.tenant_id=f.tenant_id AND o.id=f.object_id
   WHERE f.tenant_id=tid AND f.definition->'options'->>'kind' IN ('Lookup','MasterDetail')
     AND NOT (f.definition->'options' ? 'childRelationshipName')
   ORDER BY o.api_name, f.ordinal, f.api_name
  LOOP
   base := regexp_replace(regexp_replace(r.plural, '[^[:ascii:]]', '', 'g'), '[^A-Za-z0-9]', '', 'g')
        || CASE WHEN right(r.child,3)='__c' THEN '__r' ELSE '' END;
   IF base !~ '^[A-Za-z][A-Za-z0-9_]{0,39}$' THEN
    base := left('Rel' || regexp_replace(regexp_replace(r.fname, '[^[:ascii:]]', '', 'g'), '[^A-Za-z0-9]', '', 'g'), 40);
   END IF;
   n := 1;
   LOOP
    sfx := CASE WHEN n=1 THEN '' ELSE n::text END;
    nm := left(base, 40-length(sfx)) || sfx;
    EXIT WHEN NOT EXISTS (
     SELECT FROM sc_catalog.fields f2
     WHERE f2.tenant_id=tid
       AND f2.definition->'options'->>'kind' IN ('Lookup','MasterDetail')
       AND f2.definition->'field'->'referenceTo'->>0 = r.parent
       AND lower(f2.definition->'options'->>'childRelationshipName') = lower(nm)
    );
    n := n + 1;
   END LOOP;
   UPDATE sc_catalog.fields
    SET definition = definition || jsonb_build_object('options', definition->'options' || jsonb_build_object('childRelationshipName', nm))
    WHERE tenant_id=tid AND id=r.fid;
  END LOOP;
 END LOOP;
END $$;
