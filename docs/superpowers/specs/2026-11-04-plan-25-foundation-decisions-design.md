# Plan 25 — Foundation decisions design

- **Status:** Draft for maintainer review, 2026-11-04.
- **Roadmap:** Plan 25, stage A, of the [first release roadmap](2026-11-04-salescompel-first-release-roadmap-design.md). No dependencies.
- **Closes:** the waits in Plans 5, 10, 12 and 14. Their historical captures and blocked outcomes are preserved unchanged.

## Why

Plans 5, 10 and 14 are blocked on fresh live captures, and Plan 12 could not promote its hypothesis to runtime. Meanwhile the candidate has behavior clients will trip over: it rejects 15-character IDs and lowercase API names, sorts nulls differently from what was observed, and treats `AbC` and `abc` as different external IDs, which can create duplicate records.

This plan chooses a default for each open question using the roadmap's rule: follow what we have and log the gap. It then implements those defaults.

## Goals

- Every Plan 5, 10, 12 and 14 question has a recorded decision, basis and gap.
- The candidate implements the decisions in the first table below.
- External-ID lookup, uniqueness and concurrency use one tenant-scoped equivalence rule.
- Existing databases upgrade safely, or refuse without changing data.
- A required acceptance gate proves the behavior through real clients.

## Non-goals

- **No live captures.** Plan 10's and Plan 14's collection tooling stays available but is not a dependency.
- **No query engine.** Plan 29 replaces the parser. This plan makes small changes to the existing parser only.
- **No pagination.** Plan 29 owns query locators, `nextRecordsUrl`, batch headers and LIMIT above 2000. The candidate keeps its truthful rejection until then. The decisions it must follow are recorded below.
- **No general dynamic external IDs.** Plan 27 generalizes this to any external-ID field. This plan implements it for `Account.Compat_Key__c`.
- **No Composite URL changes.** Plan 31 extends Composite. Its URLs keep the 18-character, canonical-case form for now, and its bodies pick up the new field behavior through the shared write path.

## Evidence labels

| Label | Meaning |
| --- | --- |
| **Observed** | Plan 9's two complete, clean runs, or Plan 11's accepted evidence |
| **Provisional** | Plan 5's two focused captures, whose pair was never accepted |
| **Documented** | Official Salesforce documentation, not observed here |
| **Default** | A candidate choice with no reference evidence; always listed as a gap |

## Decisions to implement now

| Topic | Decision | Basis |
| --- | --- | --- |
| 15-character IDs | Accepted wherever 18-character IDs are, and case-sensitive. Responses always return 18 characters. | Documented |
| 18-character IDs | The first 15 characters are exact. The 3-character suffix is checked case-insensitively against the computed checksum. | Documented; suffix case is Default |
| Malformed or wrong-prefix ID on a record route | 404 `NOT_FOUND` | Default |
| Malformed `Id` or reference value in a SOQL filter | 400 `INVALID_QUERY_FILTER_OPERATOR` | Default |
| Malformed or wrong-prefix reference field on a write | 400 `MALFORMED_ID`, with `fields` | Default |
| Object and field API names | Case-insensitive in REST paths, SOQL and request bodies. Responses use canonical casing. | Documented |
| Duplicate fields in one JSON body, exact or differing by case | The last value wins. A denied field is denied under any spelling. | Provisional |
| Case-differing fields in one Composite body | 400 `INVALID_FIELD`. Its decoded body has lost the order needed to pick a winner. | Default |
| Unknown object in a REST path | 404 `NOT_FOUND` | Default; JSforce evidence does not establish this status |
| Unknown object in SOQL `FROM` | 400 `INVALID_TYPE` (unchanged) | Documented |
| Error envelope | Array of objects with `errorCode`, `message` and, for errors about a specific field, `fields`. | Documented; existing schema |
| Blank optional text | Reads back as null (unchanged). | Provisional |
| SOQL text equality | Simple Unicode case-insensitive. No normalization: composed and decomposed accents stay distinct. | Provisional |
| Default null ordering | Nulls first for both ASC and DESC. | Provisional |
| Explicit null ordering | `NULLS FIRST` and `NULLS LAST` are honored. | Documented |
| Tied ordering | Candidate keeps its stable tie-break on `Id`. A stable order is one valid ordering, but no reference order is implied. | Default |

### External IDs

**Equivalence rule, `canonicalExternalID` version `trim-nfc-full-fold-v1`:** remove leading and trailing ASCII spaces (U+0020 only), apply NFC, apply locale-independent full case folding, then apply NFC again. Tabs, NBSP and interior spaces stay significant. URL decoding happens once, before this rule, and percent signs stay literal.

This matches all nine Plan 12 rows, including the two whitespace rows the earlier hypothesis failed. Plan 12 warned that trimming must not be added silently; it is added here deliberately, for this reason: without it, an upsert for `" key"` creates a second record, where the reference updates the existing one.

| Topic | Decision | Basis |
| --- | --- | --- |
| Lookup | Uses the equivalence rule. | Observed |
| Uniqueness on create and update of another record | Same rule. Conflict returns 400 `DUPLICATE_VALUE`. | Observed for ASCII variants; extended by Default |
| Stored spelling on create | The spelling supplied, including any spaces. | Observed without spaces; spaces are Default |
| Stored spelling on upsert update | Never changes. | Observed |
| Ordinary PATCH on the same record | May change spelling. | Default |
| Key in an upsert body | Unchanged from Plan 11. Validation applies to the canonical field name under any case. | Observed |
| Whitespace-only upsert key | 400 `INVALID_FIELD`. | Default |
| Whitespace-only key in POST or PATCH | Treated as blank, so it is stored as null. | Default |
| Missing-key race | Serialized. Concurrent upserts for equivalent keys give one 201, then 200s. The first writer's spelling is stored. | Default, see below |
| Multiple matches (300) | Cannot occur while the field is unique. | Not applicable |

**The race differs from the reference on purpose.** Plan 9 observed the losing upsert receive 400 `DUPLICATE_VALUE`. Serializing gives the same single record and the same stored-spelling behavior, and returns a success the reference would also return for those requests in sequence. Clients that retry on `DUPLICATE_VALUE` keep working. Reproducing a timing-dependent failure would add failures without adding safety. A POST racing with another write still gets `DUPLICATE_VALUE` from the database constraint.

**Storage.** A new nullable `accounts.compat_key_canonical text` column holds the computed key, because folding can make it longer than 80 characters. `UNIQUE(tenant_id, compat_key_canonical)` replaces `UNIQUE(tenant_id, compat_key)`, and a check requires both columns to be null or both non-null. `Compat_Key__c` keeps its 80-character limit on the supplied spelling. Describe output does not change.

**Upgrade.** Catalog version 3 runs in the existing advisory-locked startup transaction, under the existing exclusive table lock, and computes keys in Go. It refuses to start, with no data changed and no row values in the error, if either of these exists:

- Two records in one tenant share a canonical key.
- A record has a whitespace-only key.

Operators resolve those by hand. The upgrade never rewrites or deletes `compat_key`. Upgrades from versions 1 and 2 and fresh installs all work.

## Decisions recorded for Plan 29

| Topic | Decision | Basis |
| --- | --- | --- |
| Default page size | 2,000, with `totalSize` covering all pages | Provisional |
| `LIMIT` above 2,000 | Valid and paged | Provisional |
| `Sforce-Query-Options: batchSize` | Honored; bounds set in Plan 29 from documentation | Provisional, bounds Default |
| Locator lifetime | Two days, using the existing named maintainer assumption | Documented; exact boundary unknown |
| Replay and concurrent retrieval | Allowed for the same user, including from another session | Provisional |
| Another user's locator | 400 `INVALID_QUERY_LOCATOR` | Provisional |
| Mutation during paging | Membership is fixed at the first page. Later pages show current field values. | Provisional; deletion rules are Default |

Until then, the candidate continues to reject `LIMIT` above 2,000 and results larger than 2,000.

## Known gaps left open

These go on the compatibility list in the decision record:

- Whitespace behavior on creation and on uniqueness. Plan 14's six unobserved dimensions all stay unobserved.
- Unicode beyond Plan 12's finite vectors, other locales, and external IDs on fields other than the fixture field.
- HTTP statuses for errors where only client-visible codes were seen.
- Exact reference behavior for malformed IDs.
- Text ordering collation and reference order for ties.
- Query text equality for characters where simple case mapping and Salesforce differ.
- Composite URLs with 15-character IDs or non-canonical names.
- Release-history reconciliation of `winter23-query-cursor-retention` and `release-content-unavailable`. These findings keep their recorded status and move to certification under the roadmap's M1R rule.

## How closure works

- Plans 5, 10, 12 and 14 each get a dated note pointing to the decision record. Their earlier text, captures and outcomes stay as written.
- `docs/compat/contracts/` stays unchanged, because Plan 18 and the lab validators bind those bytes, and `observationStatus: not_run` remains true. The decision record maps each contract ID to its decision and is the current source for candidate behavior.
- The experimental `internal/compat/externalid` package stays an experiment. The product function lives in `internal/platform`.

## Acceptance

- Go race tests and vet pass against PostgreSQL 17.
- The offline lab check and candidate OpenAPI tests pass.
- A new required gate, `candidate/foundation/decisions`, uses the real JSforce client and raw HTTP against the running candidate.
- The full `scripts/check-candidate-acceptance.sh` passes on the final commit, with results recorded.

## Calls made for the maintainer to check

1. Trimming ASCII spaces in the equivalence rule, which goes further than the roadmap's "whitespace stays a gap" wording: lookup follows the observation, while creation stays a gap.
2. Serializing the upsert race rather than copying the observed `DUPLICATE_VALUE`.
3. Moving all pagination work to Plan 29, with its decisions recorded here.
4. Using provisional captures as a basis even though their pair was never accepted.

## Left to the builder

SQL form of the backfill, helper names not fixed in the plan, exact error message wording (independently written, per `CLEAN_ROOM.md`), and test fixture layout.
