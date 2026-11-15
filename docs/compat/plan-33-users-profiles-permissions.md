# Plan 33 - Users, profiles and permission sets: observations and decisions

Status: decisions recorded, 2026-11-14; no Salesforce compatibility
certification. Later tasks cite rows here by ID (`D33-01` and so on) and copy
the exact values. A row's decision is what SalesCompel does; its status says
how well a public source backs it.

## Reference review (2026-11-14)

No trial-org session was available, so nothing here is observed. Spec Task 1
Step 2 is recorded as "no session". Every row is `documented` or `assumed`.

- `documented`: a public Salesforce page states the behavior or the code. The
  Source column says what the page does and does not state. Where a page names
  an error code but not the message text, the code is documented and the
  message is marked as assumed in the decision.
- `assumed`: no public Salesforce source states it. The value is a
  SalesCompel choice, usually from widely seen Salesforce behavior. Each
  `assumed` row is also logged under [Gaps](#gaps) as a candidate for
  correction when a session is available.

Access: direct requests to developer.salesforce.com return HTTP 403. Pages
were read through the public `r.jina.ai` reader proxy, which follows the
redirect to the current docs platform
(`developer.salesforce.com/docs/platform/...`). The help.salesforce.com
pages (including the supported time zones article) returned only a
JavaScript loading shell and could not be read. CLEAN_ROOM.md applies: only
public documentation and public vendor pages were used; no client or plugin
implementation code was inspected.

Retrieved and read: User, PermissionSet, ObjectPermissions, FieldPermissions
and PermissionSetAssignment object references; the REST API status codes page;
Core Data Types Used in API Calls (StatusCode list); SOQL Date Formats and
Date Literals; the Metadata API Profile page (search snippet only for Minimum
Access).

### Changelog review

Winter '27 (API 68.0) is the latest general release; Spring '27 notes did not
exist at review time (the Plan 29 review reached the same conclusion on
2026-11-12). Items found, all from third-party summaries because the official
release-notes index could not be read (CAPTCHA shell):

- **Profile Filtering release update** (introduced Summer '26, enforced
  Winter '27): users see only their own profile name without View All Profiles
  ([Salesforce Ben Winter '27 preview][s1]). Not adopted; recorded as a gap
  (G33-08). It would hide `Profile` rows from users who lack the permission.
- **`PermissionsUseAnyApiAuth` required for SOAP `login()`** (Winter '27
  per [Salesforce Ben][s1]; another summary places it later). Out of scope
  (SOAP login is not served); `PermissionsUseAnyApiAuth` is not exposed.
- **Profile permission retirement** (moving object and field permissions out
  of profiles) was reported as no longer scheduled
  ([Salesforce Dictionary, July 2026][s2]). Profiles keep owning a permission
  set, as Plan 33 models.
- **ObjectPermissions `PermissionsViewAllFields`** (API 63.0) exists on the
  current ObjectPermissions reference ([ObjectPermissions][o1]). Not exposed in
  Plan 33 (G33-04).
- **Summer '26 field access summary change**
  ([Salesforce Break][s3]): not read in detail; no effect on the five
  setup objects was identified.

No change to User, PermissionSet, PermissionSetAssignment, FieldPermissions or
SOQL date literals was found after 2026-11-12.

## Decisions

HTTP statuses follow the REST status-code page ([REST codes][r1]): 400 for a
request that can't be understood, 403 for a refused request, 404 for a missing
resource. Error bodies use the standard array
`[{"message": ..., "errorCode": ..., "fields": [...]}]`. `errorCode` values
are drawn from the StatusCode list ([StatusCode][r2]).

| ID | Question | Decision | Status | Source / note |
| --- | --- | --- | --- | --- |
| D33-01 | Hidden object in SOQL `FROM`, relationship traversal, child subquery (REST query) | HTTP 400, `errorCode` `INVALID_TYPE`, message `sObject type '<Name>' is not supported.` (the REST query also appends the `ERROR at Row:1:Column:n` location block already produced by Plan 29 for unknown objects) | documented (code); message assumed | `INVALID_TYPE` is "The specified sObject type is invalid" ([StatusCode][r2]). A hidden object behaves as a nonexistent one. The message text matches Plan 29's unknown-object error. |
| D33-02 | Hidden object over REST (`/sobjects/<Name>`, describe, create, record GET/PATCH/DELETE) | HTTP 404, `errorCode` `NOT_FOUND`, message `The requested resource does not exist`. The global describe omits the object. | documented (status); code and message assumed | The REST status page documents 404 for a nonexistent resource, "for example, you try to create a record using a misspelled object name" ([REST codes][r1]). The exact body is the one the candidate already returns for an unknown object. |
| D33-03 | Hidden field in SOQL `SELECT`, `WHERE`, `ORDER BY`, `GROUP BY` | HTTP 400, `errorCode` `INVALID_FIELD`, message `No such column '<Field>' on entity '<Object>'. If you are attempting to use a custom field, be sure to append the '__c' after the custom field name. Please reference your WSDL or the describe call for the appropriate names.` Same under `WITH SECURITY_ENFORCED` and `WITH USER_MODE`. | documented (code); message and the two WITH clauses assumed | `INVALID_FIELD` is "The specified field name is invalid" ([StatusCode][r2]). The message is the one Plan 29 already emits for a nonexistent column. Salesforce's own `SECURITY_ENFORCED` message differs; recorded in G33-05. |
| D33-04 | Hidden field in a create or update body (no Read, so no Edit) | HTTP 400, `errorCode` `INVALID_FIELD`, message `No such column '<Field>' on sobject of type <Object>`, `fields: ["<Field>"]` | documented (code); message assumed | `INVALID_FIELD` as above. Hidden means the field behaves as nonexistent, as in the spec. |
| D33-05 | Field readable but not editable, in a create or update body | HTTP 400, `errorCode` `INVALID_FIELD_FOR_INSERT_UPDATE`, message `Unable to create/update fields: <Field>. Please check the security settings of this field and verify that it is read-only for this user.`, `fields: ["<Field>"]` | documented (code); message assumed | `INVALID_FIELD_FOR_INSERT_UPDATE` is in the StatusCode list, although the page describes only the person-account case ([StatusCode][r2]). The message is widely quoted in vendor support pages ([Gearset][v1], [eone][v2]), not a Salesforce page. |
| D33-06 | Missing object permission on write: Create, Edit or Delete not granted | HTTP 403, `errorCode` `INSUFFICIENT_ACCESS_OR_READONLY`, message `insufficient access rights on object id` | documented (code); message assumed | `INSUFFICIENT_ACCESS_OR_READONLY` is "You can't perform the specified action because you don't have sufficient permissions" ([StatusCode][r2]); the 403 status is "Verify that the logged-in user has appropriate permissions" ([REST codes][r1]). A write to an object the user can't read at all is D33-02, not this row. |
| D33-07 | API Enabled not granted (token from a connected app or `PlatformCLI`, any REST or Composite call) | HTTP 403, `errorCode` `API_CURRENTLY_DISABLED`, message `API is not enabled for this User`. First-party web UI session marker is exempt. `API_DISABLED_FOR_ORG` is not used. | documented (code, status); message assumed | `API_DISABLED_FOR_ORG` is the org-level code ("API access isn't enabled for the org", [StatusCode][r2]) and needs no per-user check. The per-user case is reported by integrators as `API_CURRENTLY_DISABLED` / "API is not enabled for this User" ([revenue.io][v3], [symbo.ai][v4]). The StatusCode page describes `API_CURRENTLY_DISABLED` only as a temporary system problem, so the use here is a documented-code, assumed-meaning match. Salesforce's own login path returns `INVALID_LOGIN` text for the SOAP case, which is not served. |
| D33-08 | Missing Manage Users (or Modify All Data) on a write to User, Profile, PermissionSet, ObjectPermissions, FieldPermissions or PermissionSetAssignment | HTTP 403, `errorCode` `INSUFFICIENT_ACCESS_OR_READONLY`, message `insufficient access rights on object id` | documented (rule); code and message assumed | The User reference requires Manage Internal Users to create or update ([User][o2]); PermissionSetAssignment requires View Setup, Assign Permission Sets or Manage Users to access ([PSA][o3]). Plan 33 uses Manage Users and Modify All Data only (spec). The error shape is the D33-06 one. |
| D33-09 | Missing Manage Users and View All Data on a read of a setup object | Behaves as a hidden object: D33-01 in SOQL, D33-02 over REST | assumed | The references restrict these objects to View Setup and Configuration (and, for PSA, two more permissions) as of Summer '20 / API 49.0 ([ObjectPermissions][o1], [FieldPermissions][o4], [PSA][o3]) but don't state the failing error. |
| D33-10 | ObjectPermissions dependency violated (Create or Edit without Read; Delete without Read or Edit; View All Records without Read; Modify All Records without Read, Edit, Delete and View All Records) | HTTP 400, `errorCode` `FIELD_INTEGRITY_EXCEPTION`, message `Permission <Label> depends on permission(s): <Label>[, <Label>]` where labels are `Read`, `Create`, `Edit`, `Delete`, `View All Records`, `Modify All Records`; `fields: ["PermissionsEdit"]` (the dependent field). Never silently corrected. | documented (dependency rules); code and message assumed | The dependencies are stated field by field ([ObjectPermissions][o1]): Create requires Read; Edit requires Read; Delete requires Read and Edit; View All Records requires Read; Modify All Records requires Read, Delete, Edit and View All Records. `FIELD_INTEGRITY_EXCEPTION` is "An attempt was made to pass an incorrect value to a field" ([StatusCode][r2]). The error text was not found in any public page. |
| D33-11 | FieldPermissions dependency violated (`PermissionsEdit` true with `PermissionsRead` false) | HTTP 400, `errorCode` `FIELD_INTEGRITY_EXCEPTION`, message `Permission Edit depends on permission(s): Read`, `fields: ["PermissionsEdit"]`. A row whose Read and Edit are both false is deleted on update and rejected on create with `FIELD_INTEGRITY_EXCEPTION` `A field permission must grant at least Read`. | documented (rule); code and message assumed | "`PermissionsEdit` ... Requires `PermissionsRead` for the same field to be `true`"; a record without `PermissionsRead` "will be deleted" ([FieldPermissions][o4]). Updating a set of permissions to all false deletes the row ([PermissionSet][o5]). The reject-on-create wording is a SalesCompel choice. |
| D33-12 | FieldPermissions or ObjectPermissions `Field` or `SobjectType` names nothing in the catalog | HTTP 400, `errorCode` `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST`, message `bad value for restricted picklist field: <value>`, `fields: ["Field"]` or `["SobjectType"]` | documented (field type); code and message assumed | Both fields are restricted picklists ([FieldPermissions][o4], [ObjectPermissions][o1]). The restricted-picklist code name was not found in the retrieved StatusCode excerpt; it is the code Salesforce is widely reported to return. |
| D33-13 | FieldPermissions on a field that can't be permissioned (`Id`, `CreatedById`, `CreatedDate`, `IsDeleted`, `LastModifiedById`, `LastModifiedDate`, `SystemModstamp`, `OwnerId`, master-detail fields, universally required custom fields, and in SalesCompel every non-nillable field and name field) | HTTP 400, `errorCode` `FIELD_INTEGRITY_EXCEPTION`, message `Field <Object>.<Field> does not support field permissions`. Formula and auto-number fields accept `PermissionsRead` only; `PermissionsEdit` true is `FIELD_INTEGRITY_EXCEPTION` `Field is read-only`. | documented (the lists); code and message assumed | The reference lists the always-readable and always-writable types and says these have no FieldPermissions records, and that formula and auto-number fields allow only `PermissionsRead` ([FieldPermissions][o4]). Salesforce's insert error for such a row isn't stated. The non-nillable and name-field extension follows the spec. |
| D33-14 | Duplicate ObjectPermissions (ParentId, SobjectType), FieldPermissions (ParentId, Field) or PermissionSetAssignment (AssigneeId, PermissionSetId) | HTTP 400, `errorCode` `DUPLICATE_VALUE`, message `duplicate value found: <unknown> duplicates value on record with id: <existing Id>` | documented (code); message assumed | `DUPLICATE_VALUE`: "You can't supply a duplicate value for a field that must be unique" ([StatusCode][r2]). |
| D33-15 | Duplicate Username (create or update, any tenant, case-insensitive) | HTTP 400, `errorCode` `DUPLICATE_USERNAME`, message `Duplicate Username.`, `fields: ["Username"]` | documented (code); message assumed | `DUPLICATE_USERNAME`: "A create, update, or upsert failed because of a duplicate username" ([StatusCode][r2]); the User reference says the username must be unique across all organizations and a duplicate "is rejected" ([User][o2]). The case-insensitive rule is Plan 26's. |
| D33-16 | Required fields on User insert | `Username`, `LastName`, `Alias`, `Email`, `ProfileId`, `TimeZoneSidKey`, `LocaleSidKey`, `LanguageLocaleKey`, `EmailEncodingKey`. Missing: HTTP 400, `errorCode` `REQUIRED_FIELD_MISSING`, message `Required fields are missing: [<Field>[, <Field>]]`, `fields` lists them. No org-default fill over the API. Other fields the reference labels "Required" (digest and notification frequencies, Marketing User, Offline User and similar) are defaulted booleans or picklists and are not required of the caller. | documented | Each of the nine fields is marked "Required" on the [User][o2] reference (Alias, Email, EmailEncodingKey, LanguageLocaleKey, LastName, LocaleSidKey, ProfileId, TimeZoneSidKey, Username). `REQUIRED_FIELD_MISSING` is in the StatusCode list ([StatusCode][r2]). The message wording is assumed. |
| D33-17 | Username shape | Must be an email address. Stored as sent; uniqueness is compared case-insensitively. A value that isn't an email address: HTTP 400, `errorCode` `INVALID_EMAIL_ADDRESS`, message `Username: invalid email address: <value>`. | assumed | The reference says the value "must be in the form of an email address, using all lowercase characters" ([User][o2]). Salesforce is reported to accept mixed case; the all-lowercase statement isn't enforced here (G33-09). |
| D33-18 | Extra `Permissions*` fields to expose on PermissionSet and Profile (stored, listed in describe and OpenAPI as not enforced, grant nothing) | The four enforced ones `PermissionsApiEnabled`, `PermissionsViewAllData`, `PermissionsModifyAllData`, `PermissionsManageUsers`, plus these unenforced ones: `PermissionsViewSetup`, `PermissionsAssignPermissionSets`, `PermissionsManageInternalUsers`, `PermissionsViewAllUsers`, `PermissionsViewAllProfiles`, `PermissionsManageProfilesPermissionsets`, `PermissionsEditTask`, `PermissionsEditEvent`, `PermissionsExportReport`, `PermissionsApiUserOnly`, `PermissionsPasswordNeverExpires`, `PermissionsTransferAnyEntity` | assumed | The PermissionSet and Profile references say there is "one field for each permission" and that the count varies by org and license ([PermissionSet][o5]); the retrieved pages don't list the field names. Names `PermissionsViewSetup`, `AssignPermissionSets` and `ManageInternalUsers` follow permission names stated in the [PSA][o3] and [User][o2] references; the rest are well-known field names. Chosen because clients (`sf` CLI, JSforce) and metadata tools commonly send them. Unknown `Permissions*` names are rejected with HTTP 400 `INVALID_FIELD`. `PermissionsUseAnyApiAuth` and `ViewAllFields` are deliberately not exposed (changelog review). |
| D33-19 | Does Minimum Access have API Enabled? | No. Minimum Access seeds with no permissions, so its users can't use the API unless a permission set grants `PermissionsApiEnabled`. | assumed | The Metadata API says a new profile deployed with no permissions inherits the "Minimum Access - Salesforce" profile (API 60.0+), and the REST docs say API access is enabled by default in several editions, but no retrieved page states the checkbox on Minimum Access ([Profile metadata][o6], [REST editions][r3]). Spec default kept. |
| D33-20 | Inactive user, or token for a deactivated user | HTTP 401, `errorCode` `INVALID_SESSION_ID`, message `Session expired or invalid`, as for any invalid token | documented (status); code and message assumed | 401 is "The session ID or OAuth token used has expired or is invalid" ([REST codes][r1]). |
| D33-21 | Writes to a profile-owned PermissionSet, or a direct create, move or delete of a profile's PermissionSetAssignment | HTTP 403, `errorCode` `INSUFFICIENT_ACCESS_OR_READONLY`, message `insufficient access rights on object id` | documented (rule); code and message assumed | "You can query permission sets that are owned by profiles but not modify them" ([PermissionSet][o5]). ObjectPermissions and FieldPermissions on a profile-owned set follow the same rule in this plan (the spec lets admins edit profile grants through them); that part is a SalesCompel rule (G33-06). |
| D33-22 | TimeZoneSidKey values | Restricted picklist (`User.TimeZoneSidKey`). Accepted values are the IANA zone IDs in the list below, which is the Salesforce picklist as far as it could be reconstructed. Every value loads with Go's `time.LoadLocation`. Any other value: HTTP 400, `errorCode` `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST`, message `bad value for restricted picklist field: <value>`. The API doesn't use the user time zone for stored values; it matters for date literals (D33-26). | documented (semantics); list assumed | "restricted picklist ... named using region and key city, according to ISO standards"; "the API doesn't use a User time zone when querying or setting values" ([User][o2]). The authoritative list is on a help.salesforce.com page that could not be read. The list below was reconstructed from public knowledge ([Salesforce Ben][v5] describes the keys but its list text could not be retrieved) and checked only for loading in Go. |
| D33-23 | LocaleSidKey values | Restricted picklist. Accepted values are the Salesforce locale codes in the list below (language, plus `_COUNTRY` where needed). Any other: D33-22 error with the value. | documented (semantics); list assumed | "restricted picklist ... named according to the language, and the country if necessary, using two-letter ISO codes" ([User][o2]). The locale only affects UI formatting, not the API. Partial public lists exist ([Sage People][v6], a vendor page); the list below is a minimal assumed subset. |
| D33-24 | LanguageLocaleKey values | Restricted picklist: `en_US`, `de`, `es`, `fr`, `it`, `ja`, `sv`, `ko`, `zh_TW`, `zh_CN`, `pt_BR`, `nl_NL`, `da`, `th`, `fi`, `ru`, `es_MX`, `no`. Any other: D33-22 error. | documented (field exists, required); list assumed | The field is "Required" and is "the user's language, such as French or Chinese (Traditional)" ([User][o2]). Values from [Sage People][v6] and common knowledge. |
| D33-25 | EmailEncodingKey values | Restricted picklist: `UTF-8`, `ISO-8859-1`, `Shift_JIS`, `ISO-2022-JP`, `EUC-JP`, `ks_c_5601-1987`, `Big5`, `Big5-HKSCS`, `GB2312`, `x-SJIS_0213`. Any other: D33-22 error. | documented (examples `ISO-8859-1`, `UTF-8`); rest assumed | "the email encoding for the user, such as `ISO-8859-1` or `UTF-8`" and a restricted picklist ([User][o2]). The rest is from common knowledge. |
| D33-26 | Week start for `THIS_WEEK`, `LAST_WEEK`, `NEXT_WEEK`, `LAST_N_WEEKS`, `NEXT_N_WEEKS`, `N_WEEKS_AGO` for `en_US` | Sunday, 12:00:00 AM in the user's time zone. The user's `LocaleSidKey` selects the first day of the week. Assumed starts for the other listed locales (CLDR convention): Monday for `en_GB`, `de`, `fr`, `it`, `es`, `nl_NL`, `sv`, `da`, `fi`, `no`, `ru`, `zh_CN`; Sunday for `ja`, `ko`, `zh_TW`, `th`, `es_MX`, `pt_BR`. | documented (that locale sets the start); values assumed | "The exact start and stop of the range depends on the date literal and the locale of the user"; weeks start "on the first day of the week" ([Date literals][s4]). `DAY_IN_WEEK()` is 1 for Sunday (Plan 29 record). The per-locale values are CLDR convention, not a Salesforce statement. Only `en_US` (Sunday) is load-bearing for the acceptance tests. |
| D33-27 | Date literals use which clock | The running user's `TimeZoneSidKey`; a date-time field range is local midnight to local midnight compared as instants; a date field uses the local calendar date. A user with no time zone isn't possible (D33-16); a static-token principal uses the org default. Results stay in UTC. | documented | "the exact start and stop of the range depends on ... the locale of the user sending the query. If the user hasn't set a personal locale, the range depends on the locale of the org" ([Date literals][s4]). |
| D33-28 | Last-admin behavior | A write that would leave no active user with Modify All Data, or no active user with Manage Users, is rejected: HTTP 400, `errorCode` `INVALID_OPERATION`, message `Can't remove Manage Users or Modify All Data from the last active user who has it.` Applies to: updating a profile-owned or assigned set's two permissions to false, deleting a PermissionSetAssignment, changing ProfileId, setting IsActive false. | assumed | No public Salesforce statement of this rule was found; searches returned only generic advice. Recorded as a SalesCompel safety rule (spec "Users can't lock themselves out"). |

### TimeZoneSidKey values (D33-22)

IANA IDs, restricted to this list (the Salesforce picklist as reconstructed;
assumed, see the row):

```text
Pacific/Kiritimati Pacific/Chatham Pacific/Auckland Pacific/Enderbury
Pacific/Tongatapu Pacific/Fiji Asia/Kamchatka Pacific/Norfolk
Pacific/Guadalcanal Australia/Lord_Howe Australia/Sydney
Pacific/Port_Moresby Australia/Adelaide Australia/Darwin Australia/Brisbane
Asia/Seoul Asia/Tokyo Asia/Hong_Kong Asia/Kuala_Lumpur Asia/Manila
Asia/Shanghai Asia/Singapore Asia/Taipei Australia/Perth Asia/Bangkok
Asia/Ho_Chi_Minh Asia/Jakarta Asia/Rangoon Asia/Dhaka Asia/Kathmandu
Asia/Colombo Asia/Kolkata Asia/Karachi Asia/Tashkent Asia/Yekaterinburg
Asia/Kabul Asia/Baku Asia/Dubai Asia/Tbilisi Asia/Yerevan Asia/Tehran
Africa/Nairobi Asia/Baghdad Asia/Kuwait Asia/Riyadh Europe/Minsk
Europe/Moscow Africa/Cairo Africa/Johannesburg Asia/Beirut Asia/Jerusalem
Europe/Athens Europe/Bucharest Europe/Helsinki Europe/Istanbul
Africa/Algiers Europe/Amsterdam Europe/Berlin Europe/Brussels Europe/Paris
Europe/Prague Europe/Rome Europe/Lisbon Africa/Casablanca Atlantic/Canary
Europe/Dublin Europe/London GMT Atlantic/Cape_Verde Atlantic/South_Georgia
America/St_Johns America/Sao_Paulo America/Argentina/Buenos_Aires
America/Santiago Atlantic/Bermuda America/Halifax America/Puerto_Rico
America/Caracas America/Bogota America/Indiana/Indianapolis
America/New_York America/Panama America/Lima America/Mexico_City
America/Chicago America/El_Salvador America/Denver America/Phoenix
America/Los_Angeles America/Tijuana America/Anchorage Pacific/Honolulu
Pacific/Niue Pacific/Pago_Pago
```

All 94 values load with `time.LoadLocation` (checked 2026-11-14 with Go
1.25). `Asia/Rangoon` is a legacy IANA alias that Go still resolves; keep it
because clients send it. The implementation embeds this list and tests that
each entry loads. The list is data: correcting it from an observed picklist
changes the list, not the code.

### LocaleSidKey values (D33-23)

Minimal assumed set: `en_US`, `en_GB`, `en_CA`, `en_AU`, `en_IN`, `de`,
`de_DE`, `de_CH`, `fr`, `fr_FR`, `fr_CA`, `es`, `es_ES`, `es_MX`, `it`,
`it_IT`, `ja`, `ja_JP`, `ko`, `ko_KR`, `zh_CN`, `zh_TW`, `pt_BR`, `pt_PT`,
`nl_NL`, `sv`, `sv_SE`, `da`, `da_DK`, `fi`, `fi_FI`, `no`, `no_NO`, `ru`,
`ru_RU`, `th`, `th_TH`. Note: `en_US` is the only value tests depend on.
The real picklist has well over 100 values (G33-07).

## Gaps

Recorded as gaps, not blockers. "Candidate default" means the spec or this
document chose the value without a public source.

| ID | Gap | Detail |
| --- | --- | --- |
| G33-01 | Welcome email | A user created over REST has no password and receives no welcome email, as email sending isn't available. The operator sets a password with `salescompel user set-password`. |
| G33-02 | Unenforced permissions | The `Permissions*` fields in D33-18 other than the four enforced ones are stored and grant nothing. Describe and the OpenAPI document mark them not enforced. |
| G33-03 | Groups and muting | PermissionSetGroup, MutingPermissionSet, PermissionSetLicense, UserLicense and license limits are not modeled. Muting changes how FieldPermissions parents are read ([FieldPermissions][o4]); here `ParentId` refers only to a PermissionSet. |
| G33-04 | Object permission extras | `PermissionsViewAllFields` (API 63.0) and other ObjectPermissions fields not listed in the spec are not exposed. Modify All Data stores no ObjectPermissions rows, as on Salesforce ([ObjectPermissions][o1]); the effective union grants everything through the system permission. |
| G33-05 | `WITH SECURITY_ENFORCED` message | Salesforce's own error for a field blocked by `SECURITY_ENFORCED` differs from `INVALID_FIELD`. D33-03 uses `INVALID_FIELD` for both clauses. No source was found. |
| G33-06 | Profile-owned set edits | Salesforce says profile-owned permission sets can be queried but not modified ([PermissionSet][o5]). Plan 33 lets admins edit profile grants by writing ObjectPermissions and FieldPermissions against the profile-owned set (spec); writes to the PermissionSet row itself follow D33-21. |
| G33-07 | Locale formatting and picklist completeness | `LocaleSidKey` affects number, date and currency formatting in the Salesforce UI only. SalesCompel does not format by locale. D33-22 to D33-25 are reconstructed lists, not the authoritative picklists. |
| G33-08 | Profile filtering | Winter '27's View All Profiles requirement ([Salesforce Ben][s1]) is not implemented. Every user can read the profile list their permissions allow under D33-09. |
| G33-09 | Username case | The reference says usernames are lowercase email addresses ([User][o2]); SalesCompel accepts mixed case and compares case-insensitively (Plan 26). |
| G33-10 | Provisioned principals | `salescompel provision` tokens have no User row. Their stored `role` maps `admin` to System Administrator and `restricted` to Standard User permissions, with the org-default time zone. They can't be managed over the API, and can't be API-disabled. |
| G33-11 | Password-less REST users | A REST-created user can't log in until `salescompel user set-password` runs. No password-expiry or forced-reset flow exists (`INVALID_OPERATION_WITH_EXPIRED_PASSWORD` is not used). |
| G33-12 | Error messages | Every message marked assumed in the Decisions table is a candidate default. Clients that match on `errorCode` and HTTP status are unaffected; clients that match on message text may differ. |
| G33-13 | Week start per locale | D33-26 lists locale week starts from CLDR convention; only `en_US` is relied on. |
| G33-14 | Changelog gap | The official Winter '27 release notes could not be read. Only third-party summaries were reviewed. |

## Evidence and Sources

Session: none. Public pages read 2026-11-14 through the reader proxy (see
Reference review). Where a source is a vendor or blog page rather than
Salesforce, the row says so.

[o1]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-objectpermissions.html
[o2]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-user.html
[o3]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-permissionsetassignment.html
[o4]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-fieldpermissions.html
[o5]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-permissionset.html
[o6]: https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/meta_profile.htm
[r1]: https://developer.salesforce.com/docs/platform/api-rest/guide/errorcodes.html
[r2]: https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-concepts-core-data-objects.html
[r3]: https://developer.salesforce.com/docs/platform/api-rest/guide/intro-rest-compatible-editions.html
[s4]: https://developer.salesforce.com/docs/atlas.en-us.soql_sosl.meta/soql_sosl/sforce_api_calls_soql_select_dateformats.htm
[s1]: https://www.salesforceben.com/salesforce-winter-27-release-what-to-expect-and-how-to-prepare/
[s2]: https://salesforcedictionary.com/news/salesforce-news-july-9-2026-profile-permissions-retirement-cancelled
[s3]: https://salesforcebreak.com/2026/05/12/field-access-permissions-changes-summer-26/
[v1]: https://docs.gearset.com/en/articles/11145405-resolving-data-deployment-errors-unable-to-create-update-fields
[v2]: https://www.eonesolutions.com/help-article/salesforce-error-unable-to-create-update-fields-check-security-settings/
[v3]: https://support.revenue.io/integrations/salesforce/common-salesforce-errors/api-access-is-disabled-for-this-user/
[v4]: https://help.symbo.ai/en/articles/9472177-salesforce-error-api_currently_disabled-api-is-disabled-for-this-user
[v5]: https://www.salesforceben.com/list-timezonesidkey-salesforce/
[v6]: https://help-people.sage.com/Content/SP_Platform/Localization/Localization_LocaleCodes.htm
