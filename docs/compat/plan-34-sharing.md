# Plan 34 - Sharing: observations and decisions

Status: implemented, 2026-11-17; no Salesforce compatibility
certification. Later tasks cite rows here by ID (`D34-01` and so on). A row's
decision is what SalesCompel does; its status says how well a public source
backs it.

## Reference review (2026-11-17)

No trial-org session was available, so nothing here is observed. Plan Task 1
records "no session". Every row is `documented`, `implied` or `assumed`, as in
the Plan 33 record:

- `documented`: a public Salesforce page states it.
- `implied`: a public Salesforce page suggests it without stating it.
- `assumed`: no public Salesforce source was found. The value is a
  SalesCompel choice and is logged under [Gaps](#gaps).

Access: developer.salesforce.com returned HTTP 403 to direct fetches, and
several help.salesforce.com articles returned only a JavaScript shell. Most
rows were read from search-result extracts of the official pages named in the
Source column, not full page reads. Field lists for AccountShare,
ContactShare, UserRole, Group, GroupMember and Organization are therefore
`implied` until a full read or a session confirms them. CLEAN_ROOM.md applies:
only public Salesforce documentation was used.

### Changelog review

Releases from Summer '25 to Winter '27 (the latest general release) were
searched for sharing, OWD, role, group, share-object and `OwnerId` changes.
The release-notes "Sharing" index pages could not be read.

- **Asynchronous sharing recalculation** (Summer '25 onward, enabled in all
  orgs by April 2026): owner-based rules and account owner shares recalculate
  asynchronously after group or role changes; Apex and Flow stay synchronous
  until a release update enforced in Spring '27 ([async][n1], [apex RU][n2]).
  SalesCompel recalculates nothing: access is evaluated at query time, so
  changes take effect at once (D34-17).
- **`RoleAndSubordinatesInternal` group type** (Winter '26): "Roles and
  Internal Subordinates"; `RoleAndSubordinates` now includes portal and site
  subordinates ([groups RU][n3]). Adopted in D34-12.
- **Queue-level Grant Access Using Hierarchies** (Spring '26): queues are out
  of scope.
- **Implicit child shares computed at access time** (Spring '24 release
  update): child case and contact implicit shares are no longer stored
  ([implicit RU][n4]). Matches the query-time design.

No change to `OwnerId`, manual shares or OWD values was found for Winter '27.

## Decisions

| ID | Question | Decision | Status | Source / note |
| --- | --- | --- | --- | --- |
| D34-01 | REST GET of a record the user can't see | HTTP 404, `NOT_FOUND`, the same body as an unknown ID | implied | The REST status page tells callers to check sharing on 404 ([REST codes][r1]); Collections retrieve returns `null` for inaccessible and invalid IDs alike ([retrieve][r2]). |
| D34-02 | PATCH or DELETE of a readable record without the needed access | HTTP 403, `INSUFFICIENT_ACCESS_OR_READONLY`, message `insufficient access rights on object id` | documented (code); status and message assumed | Code in the StatusCode list ([StatusCode][r3]); status and message reuse D33-06. |
| D34-03 | PATCH or DELETE of a hidden record | As an unknown ID (D34-01) | assumed | Follows from D34-01; no existence leak. |
| D34-04 | Who may create a manual share | Owner, users above the owner in the role hierarchy, Modify All on the object, Modify All Data. Full Access can't be granted. | documented | [Granting access][h1], [sharing considerations][h2]. |
| D34-05 | Refused manual share | HTTP 403, `INSUFFICIENT_ACCESS_OR_READONLY` | assumed | No source for the error. |
| D34-06 | AccountShare fields and levels | AccountId, UserOrGroupId, AccountAccessLevel (Read, Edit; All only on Owner rows), ContactAccessLevel, OpportunityAccessLevel, CaseAccessLevel, RowCause. Inserts take RowCause `Manual` (the default); other causes are read-only. The level must be more open than the OWD. Inserting a share for an existing (record, grantee) pair updates that row. | implied | [AccountShare][o1]. Opportunity and Case levels are stored, not enforced. |
| D34-07 | `<Object>__Share` | ParentId (not updatable), UserOrGroupId, AccessLevel, RowCause; same rules as D34-06 | documented | [Apex sharing][a1]. Not built in this plan: custom objects have no share object yet (G34-15). |
| D34-08 | Share objects under Public Read/Write | The share object isn't available (`INVALID_TYPE` / 404) while the OWD is Public Read/Write | implied | A share can't be more open than the OWD; Apex sharing reasons need a non-maximal OWD ([Apex sharing][a1]). |
| D34-09 | Who may transfer ownership | Owner, users above the owner, Modify All on the object, Modify All Data. The new owner needs Read on the object. Transfer Record is stored and not enforced (it governs mass transfer). | documented | [Transfer][h3], [account owner][h4]. Spec revised: Transfer Record no longer gates single-record transfers. |
| D34-10 | Manual shares on transfer | Deleted in the same transaction. For an Account, the old owner's manual shares on its Contacts are deleted too. The "keep manual shares" org setting isn't offered. | documented | [Transfer][h3]; setting logged as G34-03. |
| D34-11 | Account transfer and Contacts | Contacts under the Account owned by the old owner move to the new owner | assumed | Mass Transfer documents moving contacts ([mass transfer][h5]); the plain API update isn't documented. G34-04. |
| D34-12 | Group types | `Regular` (API-writable by Manage Users), and system-maintained `Role`, `RoleAndSubordinates`, `RoleAndSubordinatesInternal`, `Organization`. Each role gets its three groups. With no portal users, the two subordinate types have the same members. Other types are refused and grant nothing. | implied | [Group][o2], [groups RU][n3]. Which types are API-creatable conflicts between sources; Regular only is chosen. G34-05. |
| D34-13 | GroupMember | GroupId, UserOrGroupId (user or group); writable for Regular groups; a group may contain a group | implied | [GroupMember][o3], [nesting][h6]. |
| D34-14 | `DoesIncludeBosses` | Stored on Regular groups, not enforced (no access through a member's managers) | implied | [Group][o2]. G34-06. |
| D34-15 | UserRole | ParentRoleId, Name, ContactAccessForAccountOwner, OpportunityAccessForAccountOwner, CaseAccessForAccountOwner (Read, Edit), plus DeveloperName and RollupDescription. Writable by Manage Users. Deleting a role with users or children is refused. | implied (fields); assumed (DeveloperName, RollupDescription, delete rule, error) | [UserRole fields][f1], [role fields][h7]. |
| D34-16 | OWD values and pairing | Account: None, Read, Edit. Contact: None, Read, Edit, ControlledByParent. Custom: None, Read, Edit, ControlledByParent. A master-detail detail is always ControlledByParent. If Account is Private, Contact must be Private or ControlledByParent. | documented | [Sharing defaults][h8], [sharing model fields][h9]. |
| D34-17 | Recalculation | None. Access is evaluated at query time, so changes take effect in the same transaction. | documented (Salesforce is async) | Divergence: Salesforce recalculates some changes asynchronously ([async][n1]). No client can rely on the delay. |
| D34-18 | New-tenant OWDs | Account Edit, Contact ControlledByParent, custom objects Edit | documented | New-org defaults since Spring '20 ([default OWDs][h10]). Spec revised from "Edit everywhere". |
| D34-19 | Migrated-tenant OWDs | Account Edit, Contact ControlledByParent, custom objects Edit | assumed | Same effective access as today (Contact follows a Public Read/Write Account). |
| D34-20 | `Organization.DefaultAccountAccess`, `DefaultContactAccess` | Returned read-only with the D34-16 values | implied | [Organization][o4]. |
| D34-21 | Implicit parent access | Read on an Account through a Contact the user reaches by ownership, hierarchy, sharing or rules. View All / Modify All on Contact don't count. | documented | [Sharing across objects][h11]. |
| D34-22 | Account owner's access to child Contacts | From the owner's role `ContactAccessForAccountOwner` when Contact OWD is Private | documented | [Sharing across objects][h11], [role fields][h7]. |
| D34-23 | Private contacts (no Account) | Visible only to the owner and View All / Modify All holders; not through the hierarchy, shares or rules | documented | [Sharing across objects][h11]. Spec revised (hierarchy removed). |
| D34-24 | Grant Access Using Hierarchies | Always on, including custom objects | documented (standard); custom-object off switch not offered | [Role guidelines][h12]. G34-07. |
| D34-25 | Deactivating a user | Owned records keep their owner. Role and GroupMember rows are kept. The user leaves the effective closure and loses all record access at once. Manual shares granted to them are deleted in the same transaction. | documented (records, role, groups, loss of access); assumed (share deletion timing) | [Deactivation][h13], [obsolete shares][h14]. Spec revised: memberships are kept. |
| D34-26 | Hidden parent in SOQL relationship paths | Null in results and filters | assumed | No source found. G34-08. |
| D34-27 | Sharing in `WITH` clauses over the API | Sharing applies in every mode; `SYSTEM_MODE` grants nothing extra | assumed | The modes are documented for Apex only ([user mode][a2]). G34-09. |
| D34-28 | `COUNT()` and aggregates | Count admitted rows only | assumed | No source found; anything else would leak existence. |
| D34-29 | Upsert by external ID matching a hidden record | Fails as a duplicate external ID (`DUPLICATE_VALUE`); doesn't update or create | assumed | Not documented; a known issue shows non-unique external-ID lookups returning 404 without View All Data ([known issue][k1]). G34-10. |
| D34-30 | Owner-based sharing rules | Records owned by members of a source group or role shared with a target group or role at Read or Edit; never more restrictive than the OWD; at most 300 per object. Managed by `salescompel sharing rule` until Plan 35. | documented | [Sharing rules][h15]. Rule rows aren't returned by share queries yet (G34-14). |
| D34-31 | Static-token principals | Own nothing; records they create go to the default owner (the first active System Administrator by ID) | assumed | No Salesforce equivalent; G34-02. The migration backfills existing records to the same user. |
| D34-32 | Delete under Public Read/Write | Needs owner-level access: the owner, a user above the owner, Modify All on the object or Modify All Data. Edit access alone isn't enough. | documented | [Record access levels][h1]. Behaviour change: before this plan, any user with Delete on the object could delete any record. |
| D34-33 | Contacts with no Account after migration | Owned by the default owner (D34-31) and, under the new-tenant Contact default, visible only to that owner and View All / Modify All holders (D34-23) | documented (rule); assumed (migration) | Behaviour change for existing tenants: Standard Users lose access to Account-less contacts until an admin sets Contact to Read or Edit. |
| D34-34 | Share objects in the catalog | AccountShare and ContactShare appear in describe and SOQL only while the parent's default is None or Read. Every share query returns the computed `Owner` row plus `Manual` rows. Any API user can create a share; the record-level rule (D34-04) decides. | implied | Extends D34-08. |
| D34-35 | Sharing setup objects | UserRole, Group and GroupMember are readable by every API user; writes need Manage Users. Changing a role, group, membership or user rebuilds the tenant's sharing closure in the same transaction. | assumed | Read access mirrors the Setup UI; G34-16. |

## Gaps

| ID | Gap | Detail |
| --- | --- | --- |
| G34-01 | No observation | Every row is documentation or assumption. Rows marked `assumed` are first to check when a session is available. |
| G34-02 | Static principals | `provision` tokens have no User row, own nothing, and see only what OWDs and Organization-group shares and rules give. Because delete needs owner-level access (D34-32), a static `restricted` token can't delete records it created; the JSforce fixture cleanup now deletes as the admin. |
| G34-03 | Keep manual shares on transfer | The org setting that keeps manual shares on owner change isn't offered. |
| G34-04 | Account transfer cascade | D34-11 follows the Mass Transfer documentation; the plain API behaviour is unconfirmed. |
| G34-05 | Group types | Personal, Queue, Territory, Manager and DataAnalytics groups aren't modeled. Sources disagree on which types are API-creatable. |
| G34-06 | `DoesIncludeBosses` | Stored, not enforced. |
| G34-07 | Hierarchy switch | Custom objects can't turn off Grant Access Using Hierarchies. |
| G34-08 | Hidden parent | D34-26 is a candidate choice. |
| G34-09 | `WITH` modes over the API | D34-27 is a candidate choice. |
| G34-10 | Upsert onto a hidden match | D34-29 is a candidate choice. |
| G34-11 | Teams, territories, restriction rules, criteria rules, queues as owners, external OWDs | Out of scope; fail closed. |
| G34-12 | Opportunity and Case access levels | Stored on AccountShare and UserRole, enforced when those objects arrive. |
| G34-13 | Key prefixes | Not confirmed by a readable page. Chosen: AccountShare `00r`, ContactShare `03s`, UserRole `00E`, Group `00G`, GroupMember `011`, sharing rules `02c`. A session should confirm them. |
| G34-14 | Rule rows in share queries | Sharing rules grant access but AccountShare and ContactShare queries don't list `Rule` rows. |
| G34-15 | Custom `__Share` objects | Only AccountShare and ContactShare exist. Custom objects get OWDs, ownership and hierarchy access but no manual shares. |
| G34-16 | Setup-object visibility | Every API user can read UserRole, Group and GroupMember. Salesforce's exact read rule wasn't found. |
| G34-17 | Browser leg | The access matrix runs through JSforce and simple-salesforce. The web app has no record pages for a browser check yet; it follows when it does. |
| G34-18 | Transfer Record permission | Stored, not enforced (D34-09). |

## Implementation and verification

- **Ownership (Tasks 2-3):** migration 0008 adds `owner_id` to Accounts,
  Contacts and custom objects and backfills it to the default owner
  (D34-31). `OwnerId` is a lookup to User with relationship `Owner`, not
  nillable and defaulted on create; the web editor no longer requires a
  value for fields the server defaults.
- **Roles and groups (Task 4):** UserRole, Group and GroupMember are setup
  objects. Every role, group, membership or user write rebuilds the tenant's
  `sc_sharing.user_groups` and `user_principals` closure in the same
  transaction (D34-17, D34-35).
- **Access check (Tasks 5-7):** one SQL predicate per object and level is
  added to every SOQL root, child subquery, semi-join, parent join and
  locator page, and to REST reads. Owner in the caller's principals, manual
  shares, owner-based rules, Controlled by Parent, the account owner's
  contact access (D34-22, honouring a `None` role setting) and implicit
  parent read (D34-21) are its terms. View All, Modify All and Modify All
  Data bypass it.
- **Writes (Task 8):** update needs Edit, delete needs owner-level access
  (D34-32), transfer deletes manual shares (D34-10) and moves the old
  owner's Contacts (D34-11), lookups need Read on the target, and an upsert
  onto a hidden match is `DUPLICATE_VALUE` (D34-29).
- **Share objects (Task 9):** AccountShare and ContactShare are views over
  manual shares plus a computed Owner row, readable by the records login
  (`TestRuntimeReadsShareViews`).
- **Commands (Task 10):** `salescompel sharing set-default` and
  `salescompel sharing rule add|list|remove`.
- **Matrix and size (Task 13):** `TestSharingMatrix` checks 7 users x 4
  Accounts x 3 defaults x {SOQL, REST read, update, delete} against a plain
  Go model and finds no unexpected grant or denial. `go test -tags
  sharingsize ./internal/platform -run TestSharingSize` (1,000 users in a
  six-level tree of 364 roles, 100,000 Accounts, Private): seeding about
  2.5 s, closure rebuild 1.2 s, top-role user 59 ms for `COUNT()` and 92 ms
  for a 200-row page, leaf user 94 ms and 120 ms, against a 5 s budget.
- **Real clients (Task 14):** the OAuth gate runs
  `sharing/jsforce-private-share` (under a Private Account default, a peer
  can't query or retrieve the owner's Account until the owner creates an
  AccountShare at Read, then reads but gets 403 on update; the share query
  returns Owner and Manual rows), `sharing/jsforce-transfer` (transfer to
  the peer removes the owner's access and the manual share) and
  `sharing/python-hierarchy` (simple-salesforce: a manager above the owner's
  role reads and edits; a peer outside the hierarchy gets an empty query
  and 404). The default goes back to Public Read/Write afterwards.
- **Existing suites adjusted for the new rules:** fixture cleanup deletes
  records made by the static `restricted` token as the admin (G34-02), and
  the Plan 33 time-zone Contact sits under an Account (D34-33).

| Full attempt | Revision | UTC interval | Outcome | Run ID | Summary SHA-256 |
| --- | --- | --- | --- | --- | --- |
| A | `882a74992c9f34859fe17ee983eef70067df804d` | 2026-11-17T08:51:26.195Z → 2026-11-17T08:59:03.995Z | passed / 0 | `0c8906da32021327ecc45cc3` | `517f1ef26f5f3e705a6f70cccaa28a95b4751007b9d29137fc068f094e4a8a56` |

The run used a disposable PostgreSQL 17 database in a Linux container with
the Playwright Chromium build. Earlier attempts on this branch failed and
led to fixes: the skipped size test (now behind a build tag), the OpenAPI
object-name pattern, the static-token cleanup, the share views calling a
function the records login can't execute, the web editor requiring
`OwnerId`, and a duplicate fixture-journal operation ID.

## Evidence and Sources

Session: none. Pages read 2026-11-17, mostly through search-result extracts
(see Reference review).

[r1]: https://developer.salesforce.com/docs/platform/api-rest/guide/errorcodes.html
[r2]: https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-retrieve.html
[r3]: https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-concepts-core-data-objects.html
[o1]: https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_accountshare.htm
[o2]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-group.html
[o3]: https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_groupmember.htm
[o4]: https://developer.salesforce.com/docs/platform/object-reference/references/standard-objects/object-organization.html
[f1]: https://developer.salesforce.com/docs/platform/field-reference/references/field-reference/salesforce-field-reference-userrole.html
[a1]: https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/apex-bulk-sharing-understanding.html
[a2]: https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_classes_enforce_usermode.htm
[h1]: https://help.salesforce.com/s/articleView?id=platform.granting_access_to_records.htm
[h2]: https://help.salesforce.com/s/articleView?language=en_US&id=platform.security_sharing_considerations.htm
[h3]: https://help.salesforce.com/s/articleView?id=000385539&language=en_US&type=1
[h4]: https://help.salesforce.com/s/articleView?language=en_US&id=sf.account_owner.htm&type=5
[h5]: https://help.salesforce.com/s/articleView?language=en_US&id=platform.admin_transfer.htm&type=5
[h6]: https://help.salesforce.com/s/articleView?id=000381884&language=en_US&type=1
[h7]: https://help.salesforce.com/s/articleView?id=platform.user_role_fields.htm&language=en_US&type=5
[h8]: https://help.salesforce.com/s/articleView?id=sf.admin_sharing.htm&language=en_US&type=5
[h9]: https://help.salesforce.com/s/articleView?id=sf.sharing_model_fields.htm&language=en_US&type=5
[h10]: https://help.salesforce.com/s/articleView?id=security_sharing_owd_default_settings.htm&language=en_US&type=5
[h11]: https://help.salesforce.com/s/articleView?language=en_US&id=sf.sharing_across_objects.htm&type=5
[h12]: https://help.salesforce.com/s/articleView?id=platform.setup_roles_guidelines.htm&language=en_US&type=5
[h13]: https://help.salesforce.com/s/articleView?language=en_US&id=platform.users_deactivate_considerations.htm&type=5
[h14]: https://help.salesforce.com/s/articleView?id=security_sharing_delete_obsolete_shares.htm&language=en_US&type=5
[h15]: https://help.salesforce.com/s/articleView?id=sf.security_about_sharing_rules.htm&language=en_US&type=5
[n1]: https://help.salesforce.com/s/articleView?id=005103630&language=en_US&type=1
[n2]: https://help.salesforce.com/s/articleView?id=release-notes.rn_customization_apex_sharing_ru.htm&release=260&type=5&language=en_US
[n3]: https://help.salesforce.com/s/articleView?id=002628970&language=en_US&type=1
[n4]: https://help.salesforce.com/s/articleView?id=release-notes.rn_sharing_account_sharing_update.htm&language=en_US&release=244&type=5
[k1]: https://help.salesforce.com/s/issue?id=a028c00000qPwqsAAC&language=en_US
