# Plan 14 — External-ID normalization contract

Status: proposed for review, 2026-10-20. Spec only; no implementation or live collection has started.
Milestones: bounded M1 evidence work supporting M3 external-ID behavior.

## Intent and selection

Resolve the concrete blocker found by [Plan 12](../../compat/plan-12-external-id-equality.md): `nfc-full-fold-v1` matches the measured case/accent pairs but disagrees with both leading- and trailing-space lookups. Lookup observations alone do not establish creation normalization, direct-write uniqueness or stored spelling. Adding trimming to the runtime now could introduce unsupported matches and ambiguous data.

The executor selects this as the next foundation package after completed Plan 12 and the implemented Plan 13 prototype. Deliver a fixed, recoverable reference experiment and a decision table separating these behaviors. Use the existing TypeScript compatibility lab, API 65.0, locked clients and synthetic Account field `Compat_Key__c`. Work directly on `main` under the repository convention. No new libraries, product code, catalog migration or UI changes belong to this package.

Success is a complete repeatable observation set that supports a subsequent runtime design, or an exact blocker explaining why that design remains unsafe. An unexpected equality result is useful evidence, never a reason to change the capture to fit a hypothesis. This package does not authorize promoting Plan 12's helper into production.

## Existing facts and open questions

[Plan 9](../../compat/plan-9-external-id-contract.md) observed ASCII case, composed/decomposed accent and sharp-s lookup matches, distinct dotless-I keys, and lookup matching for one leading/trailing ASCII space. Its ordinary-write duplicate probes covered ASCII variants only. Its missing-key race rounds returned one create and one `DUPLICATE_VALUE`; stored spelling varied by winner across captures. Those races remain a separate unresolved contract.

[Plan 11](../../compat/plan-11-external-id-body-validation.md) established omitted/null body-key handling and rejection of the tested non-null values. Every upsert here omits `Compat_Key__c` from the body, isolating normalization from that validation rule.

Current `Store.Upsert` performs raw tenant/key locking and SQL equality, then injects the path spelling into writes. Ordinary writes use the existing literal database uniqueness constraint. Neither describes the desired reference contract. General SOQL text equality and [Plan 10's foundation evidence](../../compat/plan-10-foundation-decision.md) remain independent and blocked.

The experiment must answer, separately: what spelling a new record stores; whether each lookup finds it; whether ordinary creation permits a second variant; whether changing another record to that variant conflicts; and whether explicitly changing the same record preserves or replaces spelling.

## Fixed input matrix

Each row has an independent random ASCII ownership prefix `{p}`. Actual prefixes differ for every workflow, so cases cannot influence each other. JSON escapes below identify exact code points; spaces are significant. No trimming, case folding or Unicode normalization occurs in probe inputs, raw artifacts or comparison.

| Row | A | B |
| --- | --- | --- |
| leading-space | `{p}key` | ` {p}key` |
| leading-space-reverse | ` {p}key` | `{p}key` |
| trailing-space | `{p}key` | `{p}key ` |
| trailing-space-reverse | `{p}key ` | `{p}key` |
| ascii | `{p}AbC` | `{p}abc` |
| accent | `{p}\u00e9` | `{p}e\u0301` |
| sharp-s | `{p}\u00df` | `{p}ss` |
| dotless-i | `{p}I` | `{p}\u0131` |

Only the whitespace pairs are tested in both directions. Reverse Unicode/case behavior, tabs, NBSP, other whitespace and arbitrary Unicode remain unmeasured. Keys, including ownership prefixes, must fit the existing 80-character bound without truncation; use short ASCII markers and BMP suffixes to avoid introducing a length-semantics experiment.

## Workflows and observation rules

For every row, execute these four workflows with independent fixtures:

1. **Lookup:** ordinary POST with key A; PATCH external-ID path B with a different owned Name. Record the seed's actual stored key, then response and complete final state. Distinguish update of the seed from creation of a second record. Do not substitute an upsert seed for the POST.
2. **Create uniqueness:** ordinary POST with A; ordinary POST with B under a second owned Name. Record both outcomes and final rows, including any normalization of either stored spelling.
3. **Other-record update:** ordinary POST with A; POST a second record with a distinct case-owned spare key; PATCH the second record by ID to B and change its Name. Independently read both records before and after. A rejection must leave both unchanged; acceptance records the actual resulting state. If the spare seed unexpectedly conflicts, mark the dependent step unavailable rather than changing keys dynamically.
4. **Same-record spelling:** ordinary POST with A; PATCH that ID to B with a changed owned Name. Record whether it succeeds and the exact resulting key, independently of whether A and B match in lookup.

Add four isolated **missing-key upsert/replay** workflows: leading space, trailing space, leading-space-reverse and trailing-space-reverse. Use each row's B as the first path key after verifying no owned record exists. Repeat the identical B path with a changed owned Name. Observe first-create spelling, replay identity and final spelling. These controls distinguish upsert creation from POST creation. Both requests omit the body key.

This produces **36 scenarios and 80 non-cleanup mutation operations per capture**: eight rows times nine operations across four workflows, plus eight isolated upsert operations. Maximum potential records are **64**: seven per row across the four workflows plus two per isolated replay case. These are ceilings, not expected reference outcomes. Freeze exact scenario/operation identifiers, key templates and order in a new manifest; do not alter Plan 9's 51-operation manifest or Plan 11's sixteen-case manifest.

Use only ordinary Account POST, record-ID PATCH/GET, the existing external-ID PATCH route, bounded marker discovery queries and exact-ID DELETE for cleanup. URL-encode path keys exactly once; do not decode percent strings inside a normalization helper. No concurrent requests, native-client projections, permission changes, metadata deployment, null/empty keys, general error precedence or query-equality probes are included.

After each mutation, retain raw status, body presence and success/error structure, including `errorCode`, presence/value of `fields`, success/created flags and returned IDs where present. Retain message text privately; do not require literal prose equality or copy vendor prose into product requirements. Read known IDs directly and discover all case-owned Names after every step. The evidence must retain submitted key, stored key and identity relationships separately. An error alone does not prove absence of a hidden write.

If any prerequisite seed fails, preserve its actual response and mark dependent operations unavailable. Authentication failures, malformed successes, incomplete discovery and missing state evidence block assessment. Do not reinterpret these as normalization or uniqueness results. A valid measured rejection of a test operation is a result, even when it contradicts the anticipated behavior.

## Ownership, authorization and recovery

Reuse Plan 9/11's established administrator-only identity binding, private configuration, preflight, durable journals and recovery machinery where practical. This suite needs no fresh/restricted foundation sessions and does not consume or modify preserved Plan 5 session artifacts. Existing named disposable-tenant authorization is a prerequisite to execution; creating this spec makes no reference requests.

Before writes, verify HTTPS origin, pinned org/administrator identity and API 65.0; recheck the Text(80), externalId=true, unique=true, caseSensitive=false declaration. Require disposable opt-in, private recovery location and no unresolved prior relevant recovery state. Missing credentials or metadata produce a blocker; this package must not silently provision identities or alter permissions/fields.

Journal every intended mutation before dispatch, all allowed Name transitions, exact submitted key variants and every returned/discovered ID. Names are independent ownership markers because keys may normalize unexpectedly. Ownership must be established by exact case markers and durable intent/ID bindings; unexpected keys remain evidence to reconcile, never permission to delete an otherwise unproven record. Preflight collisions for all intended Names and keys. Never use prefix-based deletion or normalization-dependent key lookup as the only ownership proof.

Check identity before cleanup and at finish. Identity mismatch is durably latched and stops mutations/deletion. Distinguish intent, dispatched, definitive response and unresolved outcome. A lost response can commit later: empty reads or elapsed time cannot resolve a dispatched unknown request. Retain that block across crashes; recovery can delete proven owned rows but cannot claim clean or permit run two while unresolved operations remain.

Keep Plan 9's outer ceilings per capture: 80 potentially created records, 160 non-cleanup mutations, ten-minute collection and five-minute cleanup/reconciliation; the fixed suite uses at most 64 records and 80 mutations. Requests are sequential, with 15-second wire deadlines and no mutation retries. Bounded reconciliation reads may retry at most three times. Budget exhaustion stops collection and preserves unresolved journal state. Recovery is separately runnable and bounded.

Delete only proven owned exact IDs, then independently verify absence of every known ID and all owned Names. Preserve IDs even when keys or Names change. Start the second capture only after verified first-run cleanup, with identities and prior recovery state rechecked. No live execution occurs while writing or reviewing this spec.

## Evidence and assessment

Introduce a separately identified versioned report/schema and commands for capture, comparison and recovery, using existing lab patterns. Exact module and command names are left to the builder. Historical suites and accepted profiles retain their meaning; shared-helper changes require their existing regression tests.

Bind reports to the fixed manifest, reviewed executable probe source, profile bytes, API, locked client versions, origin/administrator config, run ID and capture times. Store raw responses, independent state reads and recovery evidence outside the checkout in create-exclusive 0600 files under 0700 directories. Reject symlink escapes, corrupt/missing artifacts and binding mismatches. Never retain authorization headers. Publish only synthetic examples, reviewed outcomes and digests/times, with no private identities or record IDs.

Require two fresh sequential captures with identical bindings and complete operation coverage. Compare exact status/body structure, error codes/fields shape, submitted/stored spelling, Name changes, record counts and identity relationships under one-to-one run-local ID aliasing. Ignore only explicitly documented volatile metadata and message prose; retain their raw bytes. Do not lowercase, trim or normalize values to make captures agree. Record order may be compared by aliased identity; records and values may not be dropped. Cleanup and exact-ID absence evidence must be digest-bound and complete for every discovered ID.

Assessment distinguishes **repeatable**, **drift** and **blocked** at scenario and suite level. A complete, unexpected but repeatable result is repeatable evidence, not proof that an anticipated product rule passed. Missing/unavailable evidence blocks completeness. Repeatability over these inputs establishes no universal equivalence relation.

Publish `docs/compat/plan-14-external-id-normalization-contract.md` during execution with case-level outcomes, exact stored spellings, capture/binding digests and times, cleanup verification, contradictions, remaining unknowns and candidate differences. Keep lookup matching, POST creation, upsert creation, ordinary-write uniqueness and same-record spelling as separate columns. If live prerequisites fail, publish a precise blocker and offline verification rather than fabricate observations.

## Handoff and scope boundary

The decision must explicitly say whether the finite observations support one shared candidate equivalence rule, or reveal differences that need separate lookup and storage rules. If proposing NFC/full-fold with ASCII-space handling, state the exact transformation, ordering and evidence limits as a candidate design recommendation. Never infer general trimming from these one-space cases, or claim universal Salesforce collation parity.

A later reviewed product design must settle supported normalization scope, preserved versus explicitly edited spelling, error envelopes, uniqueness and locking across all writes, collision-blocking transactional migration from catalog v2, rollback and mixed-version deployment. Any diagnostic for that migration must use the selected rule; Plan 12's no-trim collision report is insufficient if the rule changes. No runtime helper or migration is implemented here, even after repeatable evidence. Missing-key races require a separate decision and remain open.

Plan 10 collection, Plan 5 query contracts, general Unicode behavior, tenant-wide migration readiness, broader metadata support and Plan 13 visual acceptance are not closed by this result. No milestone or compatibility certification claim follows.

## Verification and completion

Meaningful local tests must cover the exact 36-scenario/80-operation manifest and bounds; directional whitespace preservation; POST versus upsert seeding; actual stored-key capture; same-record versus other-record updates; unexpected duplicate/normalization responses; and rejected requests that nevertheless mutate state. Simulate response loss followed by late commit after initial absence reads, identity change, changed Name/key, multiple discovered IDs, missing IDs, ambiguous ownership and cleanup failure. These must preserve evidence or block safely.

Schema/comparison tests must reject incomplete coverage, changed probe/manifest bindings, corrupt state artifacts, invalid alias mappings and incomplete absence attestations. Distinct stored spellings and hidden state changes must produce drift. Agreeing non-hypothesized outcomes must remain faithfully repeatable. Historical Plan 9/11 tests must still pass.

Run `npm --prefix compat/clients/node run check` and `git diff --check`. No product database tests are required for an evidence-only change; any unexpected product changes require rescoping before execution. Obtain fresh whole-change review and resolve Important/Critical findings. Report offline tooling completion separately from live-evidence completion. A blocked collection is an honest bounded deliverable, while the normalization contract remains unresolved.

## Builder check

A fresh software-engineer reviewer read this design and the relevant existing lab contracts and reported “nothing important left.” No material planning questions required a maintainer decision. The executor selected the foundation follow-up, finite input matrix and evidence-only boundary; those are recommendations for review, not new reference observations or approval to implement.
