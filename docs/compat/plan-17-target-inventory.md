# Plan 17 — Target inventory contract

Status: implemented, independently reviewed and corrected; final verification passed, 2026-10-21. The checked-in
inventory is a synthetic demonstration with open questions, not a collected
customer inventory. M0 remains open. See the [design](../superpowers/specs/2026-10-21-plan-17-target-inventory-contract-design.md)
and [implementation plan](../superpowers/plans/2026-10-21-plan-17-target-inventory-contract.md).

## Run offline validation

From `compat/clients/node`, using the existing installed dependencies:

```sh
node --import tsx src/cli.ts validate-target-inventory \
  --input ../../../docs/compat/inventories/synthetic-account-contact.yaml

# For a privately authored real inventory; paths refer to existing private files/directories.
node --import tsx src/cli.ts validate-target-inventory \
  --input "$INVENTORY_INPUT" --out "$INVENTORY_REPORT"
```

Only `--input` and optional `--out` are accepted. No credentials, COMPAT
environment, reference session, database, browser or network are used. Source
locators are inert provenance text: the command never opens or fetches them.
The command checks repository-reference syntax; the ordinary lab `validate`
command also resolves the checked-in example's approved repository links.
The ordinary `npm run check` includes this validation.

| Exit | Meaning |
| --- | --- |
| 0 | Structurally and semantically valid, even if requirements remain unknown |
| 1 | Invalid document, unsupported schema version, malformed input or arguments |
| 2 | Input/output could not be safely read or written |

| Readiness | Meaning |
| --- | --- |
| `synthetic-only` | Synthetic classification, including internally complete examples; never customer readiness |
| `incomplete` | Real inventory has gaps or invalidity, or classification cannot be established |
| `ready-for-review` | Valid real inventory with complete sections, evidence and resolved required information; human review still needed |

Validity, readiness, observed behavior, implementation, verification and milestone
acceptance are separate facts. An implemented candidate workflow cannot establish
customer demand or Salesforce parity. A `ready-for-review` report does not pass M0,
authorize product scope or certify compatibility.

## Author the inventory

Use [the synthetic example](inventories/synthetic-account-contact.yaml) as a
format reference and the [closed schema](../../compat/schemas/target-inventory.schema.json)
as the field contract. Copying the example does not inventory a real organization.
Schema version is 1; revision is a positive integer, and dates are calendar-valid
`YYYY-MM-DD`. Top-level metadata names the inventory, classification, target and
accountable owner. The source register has IDs, kinds, dates, descriptions and
nonsecret locators; `evidenceRefs` and `provenanceRefs` refer only to these local
source IDs. Source kinds distinguish repository documents, operators, source
exports, pipelines and observations.

Every section is present with `status` (`unknown`, `partial`, `complete`),
`ownerId`, `evidenceRefs`, `questions`, `absence` and `items`:

| Section key | Item categories |
| --- | --- |
| `sourceOrg` | `source`, `org-feature` |
| `dataMetadata` | `object`, `field`, `relationship`, `record-type`, `layout`, `data-volume`, `export-area` |
| `workflowsUsers` | `workflow`, `persona` |
| `clientsIntegrations` | `client`, `integration` |
| `cliPlugins` | `cli`, `plugin`, `cli-command` |
| `automationCustomization` | `apex`, `flow`, `formula`, `validation`, `legacy-rule`, `imported-ui`, `package` |
| `security` | `security-policy` |
| `localization` | `locale-requirement` |
| `migrationOperations` | `migration-path`, `operating-objective` |

Item details are typed and closed by category. Record actual edition/release/API
versions and enabled features, data/metadata families and export limitations,
user journeys and outcomes, exact clients/authentication/dependencies, actual
CLI/plugin commands/flags/output formats, automation and package/class versions,
security grants/denials, locale/support/fallback requirements, and migration and
operating objectives. Do not infer the latest version or mark an empty list as
feature absence. Source API/class API versions use the wire `digits.0` format;
other version fields record exact nonempty source strings. Non-org tools are
recorded as commands with target `non-org`, with explicit requirement disposition.

Unknown values use a tagged representation rather than placeholder strings,
nulls or invented defaults. For example, these are three different facts:

```yaml
# An operating objective not decided yet:
value:
  status: unknown
  question: What maximum cutover downtime can the operator accept?
  nextAction: collect
# A measured zero (for example data-volume count):
count:
  status: known
  value: 0
# A feature explicitly known to be disabled:
enabled:
  status: known
  value: false
```

Knowledge values use `status: known` with `value`, or `status: unknown` with
`question` and a next-action code. Codes are `collect`, `clarify`, `assign-owner`,
`define-acceptance`, `reconcile` and `review-scope`. Questions also have unique IDs
and owners. An item's `behavior`, `acceptance` and `milestone` use Knowledge too.
A required item must have known behavior, acceptance and one primary milestone
(M0, M1, M1R, M2–M10), plus owner and provenance. Other items still carry those
fields but can explicitly mark them unknown. Dependencies reference item IDs;
dangling references, self-dependencies and cycles are rejected.

Disposition is `required`, `optional`, `out-of-scope` or `unknown`. Every item has
a rationale. An unknown disposition also needs a `decision` with a question and
next action; an out-of-scope rationale records why exclusion is deliberate.
`facts.observation`, `facts.implementation` and `facts.verification` each carry
an independent `unknown`/`documented` status and evidence references. Documented
facts require evidence; unknown facts cannot carry evidence as if documented.
Unknown runtime facts do not by themselves prevent collection completeness.
Any non-null item decision remains a pending owned question, including for
required, optional or out-of-scope items; it prevents complete readiness.
Known command `pluginId` values must resolve to an inventory item of category
`plugin`. A known null value explicitly records a command supplied without a
plugin (for example a non-org tool); unknown plugin provenance stays unknown.

A complete section cannot contain unresolved questions, unknown dispositions or
unknown Knowledge values. A complete empty section needs an `absence` attestation
with owner, evidence references and rationale. A nonempty section cannot also
attest absence. Complete collection requires section-level evidence too; missing
evidence remains an owned gap and prevents real readiness. These rules validate
the recorded statements; they cannot establish that a human attestation is true.

## Reports, limits and privacy

Without `--out`, stdout is one JSON report. With `--out`, stdout is a static
outcome/readiness summary without the path or inventory contents, and JSON is
written exclusively to the new output. Existing outputs are never overwritten.
Write/sync failures remove only the output created by the command. No parent
directory is created automatically.

Real input files must be mode 0600 and owned by the operator, in an existing
owner-only directory outside the checkout. Synthetic input may be checked in.
All output parents must be existing owner-only directories outside the checkout.
Symlink inputs, symlink path components, nonregular files and symlink outputs
are rejected. Filesystem commands require Linux with `/proc/self/fd` available;
other environments fail closed with exit 2. Every directory component is opened
with no-follow flags and pinned by a descriptor. Final read/create operations
and failed-output cleanup use that verified parent descriptor, so a concurrent
directory replacement cannot redirect them to a private symlink target. Real
input permissions are checked against the actual read handles' metadata. The
in-memory assessor does not require Linux. Keep source files, real inventories and reports private; nothing is
automatically published. Use nonsecret endpoint descriptions and never store
tokens/passwords in inventory fields.

Reports allowlist schema version, ID/revision/classification, SHA-256 of exact
input bytes, validation/readiness, section states, stable issue codes and owned
actionable gaps. No labels, provenance locators, questions, rationale, arbitrary
property names, parser prose or OS error messages are printed. Inventory, item
and owner IDs are deliberately included as diagnostic identifiers and must be
nonsecret lowercase slugs (`[a-z][a-z0-9-]{0,63}`). Do not put private names or
credentials in IDs; this is not a general-purpose anonymizer. Structurally
invalid metadata is included only when its individual type/format is safe.

Limits are 1 MiB input, YAML alias expansion ceiling 50, AST depth 32 and 10,000
visited AST nodes including expansion. UTF-8 decoding is strict; duplicate mapping
keys (including JSON), multiple documents, alias cycles and malformed values
are rejected. Collection/non-string mapping keys are rejected before conversion;
YAML warning logging is disabled for input and referenced YAML, so warnings cannot
echo private keys to stderr. File reads are bounded. `inputDigest: ""` means unavailable when
a file was refused before reading (oversize or unsafe I/O). In-memory assessment
hashes every supplied byte, even for invalid input. Issues and gaps have stable
ordering by section, item, code, owner and next action; reports have no volatile
timestamps.

Local `repositoryRefs` must use approved `docs/compat/`, `docs/plans/`,
`docs/superpowers/`, `locales/` or `api/` prefixes, with no traversal, URL, drive,
percent escapes, backslashes or symlink components. Repository validation checks
existence and optional YAML contract/profile IDs or Markdown heading IDs
(lowercase, whitespace replaced by hyphens). Other formats accept paths only.
It does not resolve source locators or change accepted profiles.

## Verification

On 2026-10-21, the post-review frozen-source full lab check (`npm --prefix
compat/clients/node run check`) passed typechecking, formatting, tests and
repository validation: **321 tests, 318 passed, zero failures, three skips**.
The skips are the existing optional `candidate/jsforce/core`,
`candidate/jsforce/policy` and `candidate/jsforce/isolation` live-candidate tests.
All **57 target-inventory tests** ran without skips. The public example reported
valid / synthetic-only with 29 owned gaps. `git diff --check` and guide link
checks passed; 13 protected profile/evidence/dependency files were byte-identical
to execution base `8712c46`.

The initial full run exposed `direct reference capture blocks pending
authorization before clients or requests` and `CLI preflight reports reference
blockers but candidate needs no reference decision`: inventory imports eagerly
required a schema in the minimal pending-reference fixture. Validation now loads
inventory modules only when needed; both regressions passed afterward. An
intermediate run overlapped further CLI edits and failed `matching malformed
create success errors block assessment rather than becoming repeatable evidence`
with `NORMALIZATION_BINDING_MISMATCH`; it is not counted as verification. The
final successful run used unchanged source for its entire duration.

Independent whole-change review found four Important issues and no Critical or
Minor findings: YAML collection-key warnings bypassed report filtering;
directory replacement could race path checks; pending item decisions could be
ignored; and known command plugin IDs could dangle. Each finding was reproduced
by failing tests and corrected in one fix pass. The warning tests include the
real CLI's stderr and referenced YAML. Directory-swap tests verify actual opened
paths and output creation; readiness tests cover all resolved dispositions and
missing/wrong-category plugins. A plugin-free command uses explicit known null.
No behaviors were declined by the reviewer. The full post-fix lab check passed with the counts above; no second review was dispatched. The reviewer declined no behaviors and deferred no Minor findings.

No real inventory or new Salesforce evidence has been collected, and no
database/browser/live acceptance is claimed by this package.

## Implementation decisions

- Work stayed directly on main under the approved repository convention; changes
  share the existing checkout rather than an isolated worktree.
- A file refused before bounded reading has an unavailable (empty) digest rather
  than a fabricated digest of truncated bytes. Report consumers must handle it.
- Filesystem access uses Linux descriptor paths because Node has no openat API
  and this package cannot add dependencies. Other platforms need a reviewed
  adapter; the commands currently fail closed there.
- Known null command plugin IDs mean explicitly plugin-free, preserving non-org
  tools without fabricated plugin records. Schema consumers must accept this
  alternative; non-null IDs must resolve to plugin items.

## Operator handoff

1. Nominate the named target, inventory owner and contract owners.
2. Collect authorized source, workflow, pipeline and operating information privately;
   record dates and provenance without credentials.
3. Validate, resolve contradictions and assign an owner/next action to every gap.
4. Review required customization feasibility, denied-access behavior, localization,
   export exclusions and operating objectives with accountable people.
5. Review the real inventory, then propose separate target profile/contract changes
   and bounded feasibility packages. Preserve historical evidence and profile hashes.

Plans 10/14 resume under their own prerequisites, Plan 13 visual acceptance stays
open, and full M0 sign-off, product behavior and certification remain separate work.
