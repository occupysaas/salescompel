SELECT jsonb_object_agg(c.relname,jsonb_build_object(
 'columns',(SELECT jsonb_agg(jsonb_build_array(a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,pg_get_expr(d.adbin,d.adrelid)) ORDER BY a.attnum) FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped),
 'constraints',(SELECT jsonb_agg(pg_get_constraintdef(x.oid) ORDER BY pg_get_constraintdef(x.oid)) FROM pg_constraint x WHERE x.conrelid=c.oid)))
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname=current_schema() AND c.relname IN ('users','oauth_apps','oauth_app_jwt_users','oauth_requests','oauth_codes','oauth_refresh_tokens','oauth_access_tokens','oauth_jwt_ids','accounts','contacts','principals','tenants','platform_catalog');
