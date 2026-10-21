# Plan 19 — Apex execution feasibility

Status: proposed for maintainer review; specification only, no implementation or reference collection performed.
Milestone: bounded M7 feasibility work brought forward under the existing roadmap; full M7 remains open.

## Intent and selection

Test whether an independently implemented, deliberately small Apex execution path can preserve source input while controlling record access, transaction effects and execution budgets. Deliver an executable experiment and a concrete continue/revise/blocked decision before investing in a general runtime. A syntax demo alone cannot answer the architectural question.

The executor recommends this package after [Plan 18](../../compat/plan-18-foundation-release-reconciliation.md). Its pinned historical sources remain unavailable; repeating the same failed retrievals is not a new package. Plans [10](../../compat/plan-10-foundation-decision.md) and [14](../../compat/plan-14-external-id-normalization-contract.md) resume under their existing numbers when their reference prerequisites exist. [Plan 17](../../compat/plan-17-target-inventory.md) supplies an inventory format but no real target inventory. The [remaining roadmap](../../plans/2026-10-20-progress-and-remaining-roadmap.md#recommended-sequence) explicitly calls for early customization feasibility; the [original roadmap](../../plans/2026-10-14-salesforce-compatibility-roadmap.md) identifies M7 as a major uncertainty.

Selection of Apex first, an offline interpreter experiment and the narrow language boundary below are executor recommendations, not previously accepted customer requirements. Flow, imported UI and packages need separate feasibility packages. A missing real inventory limits relevance to actual migration targets; it does not prevent testing this synthetic architectural hypothesis.

## Question and success condition

Can a small source-to-execution pipeline run independently authored Apex-shaped fixtures without per-fixture translation, route every record operation through one explicit host boundary, discard all staged effects on failure and terminate runaway execution predictably?

Success means reproducible answers with failing cases and limitations exposed. A working experiment establishes only feasibility for its stated subset and synthetic host semantics. It does not establish Salesforce parity, a safe production sandbox, sufficient customer coverage or an approved architecture for all Apex. A negative result is a valid completed study when the minimum experiments were attempted and the blocker is reproducible.

## Existing assets and constraints

Read [CLEAN_ROOM.md](../../../CLEAN_ROOM.md), the roadmap's M7 requirements, the Plan 17 automation inventory model, and the current candidate's `internal/platform/context.go`, `store.go` and `http.go` before planning. Record the existing transaction/authentication boundaries relevant to eventual integration; do not refactor or import them into this experiment. Existing REST/query and external-ID decisions remain independent.

Use Go and the repository's pinned toolchain, standard library only, in a new isolated `internal/compat/apex` package. No product package may import it. Work directly on main under repository convention. Do not change dependencies, database/schema/catalog, product endpoints, UI, accepted profiles, readiness attestations, observations or historical captures. Do not change the clean-room policy or copy implementation code, grammars or Salesforce examples.

During execution, consult public official language documentation for the exact constructs selected, recording URL, section, access date and version applicability or explicit unknowns. Independently author all fixture source and summaries. This specification makes no new assertion about Salesforce language/security/limit semantics. Unreadable documentation remains a source gap; do not infer reference behavior from the experiment or a search snippet. No live org calls, credential discovery, customer source, managed-package inspection or external communications belong to this package.

## Deliverables

1. An isolated source parser, evaluator and synthetic transactional host under `internal/compat/apex`, exercised by ordinary Go tests. A public API/server, general CLI and plugin system are unnecessary.
2. Small independently authored source fixtures and table-driven scenarios under that package's `testdata`, with exact source bytes, entry point, inputs, expected synthetic outcome, actual outcome and basis.
3. `docs/compat/plan-19-apex-execution-feasibility.md`: source/subset ledger, reproducible results, architecture decision, candidate integration gaps and concrete follow-up gates.

Prefer a hand-written parser and interpreted syntax tree for the bounded experiment. Compare this choice briefly against transpilation and a bytecode VM in the decision report, using the observed host/control needs. No second implementation or broad compiler survey is required. The builder chooses internal APIs and syntax-tree representation; execution must consume source, not manually constructed trees or fixture-name dispatch.

## Language and fixture boundary

Define and publish the exact accepted grammar before implementing the evaluator. Target one class containing one static entry method, local variables, parameter/return values, assignment, integer/Boolean/string/null literals, comparison and basic arithmetic, conditional branches, a bounded normal loop and a deliberately nonterminating loop. Include the smallest record/list syntax needed for the host scenarios below: a static Account query, iteration, assignment to a synthetic record's Name and a bulk update statement. Use literal query structure with bind values; do not build a general SOQL parser.

The builder chooses a documented common syntax for these constructs and records the exact method signature, supported types/operators and query form in the guide. Treat integer overflow, null operations, string comparison and evaluation order as explicit experiment decisions wherever no reviewed source settles them. Do not silently use Go behavior as a compatibility claim. Deterministic rejection is preferable to accepting syntax whose semantics the experiment does not implement.

Unsupported syntax fails before any host operation, with a stable experiment error category and useful source location. Test at least inheritance, an unimplemented platform call, dynamic query construction and malformed source. Full language support, triggers, sharing declarations, production CRUD/FLS semantics, exception handlers/savepoints, async jobs, callouts, reflection, packaging, deployment and Salesforce test coverage are excluded. Their exclusion is a feasibility gap, never evidence that real targets do not need them.

Use a synthetic Account field for selection and Name for mutation; avoid external-ID equality and the candidate's unresolved general query semantics. No source rewriting per scenario is permitted. Entry arguments and host data may vary while source bytes stay identical. Record which inputs are document-informed and which expectations are purely synthetic.

## Host, transaction and resource model

Each invocation receives explicit tenant/principal context, input parameters, a private synthetic store snapshot and resource budgets. Query/update operations pass through a narrow host interface. The host controls tenant filtering, allowed operations/fields and record handles; the evaluator cannot access a global store or another invocation's context. The synthetic policy is intentionally explicit and does not emulate an Apex execution security mode.

Stage mutations per invocation and publish them only after successful completion. Any parse, type, host-policy, execution or budget error discards every staged mutation. Scope the harness to sequential commits; concurrent isolated invocations must use separate stores. Shared-store concurrency, isolation levels and real database integration are deferred and cannot receive a passing verdict from this experiment.

Budget at least source size/parser nesting, evaluation steps, call depth, returned rows, staged writes and string/collection sizes. Set and document small finite defaults and count before performing the bounded action/allocation. Use explicit interpreter checkpoints and deterministic operation counters; a test timeout is only a backstop. Recursive calls may be rejected as unsupported, with the call-depth budget guarding supported entry/host transitions. Support cancellation between evaluation/host steps. Synthetic limits and error categories are not Salesforce governor limits or wire errors.

No evaluated program may access filesystem, network, process execution, wall-clock time or environment variables. The experiment is an in-process interpreter with a limited host surface, not proof against hostile code or unbounded memory use. Document any remaining allocation/control gap as a production blocker.

## Required experiments

| Scenario | Required assertion and interpretation |
| --- | --- |
| Source execution | Parse and execute one ordinary conditional/loop computation; varied parameters change results through the same source bytes. This tests the pipeline, not Salesforce parity. |
| Record workflow | Query a synthetic batch, iterate, change Names and stage one bulk update; commit exactly the intended rows once on success. Run the same program with empty, one-row and multirow inputs. |
| Failed transaction | Stage a write, then encounter a denied operation or deterministic runtime error; compare the entire store before/after and prove zero committed changes. |
| Tenant and principal separation | Reuse local record IDs across tenants; queries cannot return another tenant's records and forged/cross-tenant update handles fail. Run the same source under allowed and denied synthetic principals. |
| Resource termination | Infinite loop, too many rows/writes, oversized value and excessive parser nesting produce their stated controlled failures and discard staged changes. Include a budget exhaustion after a staged write. |
| Cancellation | Cancel before execution and during a multi-step run using deterministic synchronization; no partial commit. |
| Unsupported input | Reject each excluded construct above before host activity; malformed input reports a bounded diagnostic without a panic. |
| Repeatability and isolation | Repeat each deterministic scenario with fresh state; concurrent invocations on separate stores do not share variables, context or budgets. |

Use host-call traces or equivalent assertions to distinguish parser rejection, evaluator failure, host denial and commit/discard. Do not log private identifiers or assume test success proves production isolation. Fixtures are synthetic and may be public.

## Decision record and promotion gates

Report each scenario as passed, failed, or blocked with command, fixture, assertion, actual result and reason. Separate documentary support, synthetic expectation, reference observation (none in this package), implementation and verification. Include all supported and rejected constructs; a happy-path subset must not conceal failed or unavailable cases.

Conclude with one of these bounded decisions:

- **Continue:** the complete minimum experiment passes; this architecture merits the next evidence-backed slice for this subset only.
- **Revise:** an attempted scenario exposes a reproducible architectural problem; propose a specific alternative and the discriminating next experiment.
- **Blocked:** a required question cannot be tested or a source prerequisite cannot be resolved; identify the missing input, owner and next action. Unattempted work without such a blocker is incomplete.

Document proposed attachment points to the candidate and the work they would need: transaction ownership, metadata/type resolution, permission/security context, query execution, errors and metering. Do not wire these into production. A further design must establish real target requirements, class/API versions, authorized reference observations, transaction/trigger ordering, limits and security before promoting any semantics. Plan 10/14 evidence cannot be replaced by these synthetic results.

Identify separate follow-ups for Flow execution, imported component hosting and required packages without inventing their feasibility or assigning them implemented coverage. Leave M0/M1R/M7 gates and program estimates unchanged; the experiment supplies inputs to a later re-estimate.

## Verification and acceptance

At execution run focused tests, `go test -race ./internal/compat/apex`, `go test ./...`, `go vet ./...` and `git diff --check`. Record actual failures, skips and environmental limitations. No database or live client run is needed for the isolated host; do not claim those were verified. Confirm the experiment is absent from product dependency graphs and protected profiles, readiness/evidence files and dependency manifests are byte-identical to execution base.

Obtain a fresh whole-change review of source execution, host bypass opportunities, rollback, resource termination and the honesty of conclusions. A completed positive experiment requires all mandatory scenarios; a completed negative study requires reproducible attempted failures/blockers and an actionable decision. Neither outcome certifies compatibility or authorizes product promotion.

The current request delivers this specification only. An implementation plan and execution follow after design review.

## Builder check

A fresh software-engineer reviewer read this specification and relevant repository context and reported “nothing important left.” Package selection, the interpreter approach and the synthetic scope remain executor recommendations for maintainer review.
