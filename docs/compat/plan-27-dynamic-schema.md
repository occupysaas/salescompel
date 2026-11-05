# Plan 27 — Dynamic schema decisions and execution record

Status: implementation in progress; no Salesforce compatibility certification.

## Reference review (2026-11-05)

No authorized trial-org session was supplied. Existing observed contracts remain
unchanged; no local candidate test is a Salesforce observation. Reviewed the
[Winter 27 developer overview](https://developer.salesforce.com/blogs/2026/10/developers-guide-to-the-winter-27-release)
and [release notes](https://help.salesforce.com/s/articleView?id=release-notes.salesforce_release_notes.htm&release=264&type=5).
The overview covers headless tooling and metadata generation, with no accessible
claim changing this bounded Text/describe contract. Dynamic Metadata API reference
pages yielded wrapper HTML during planning; their detailed semantics remain
unverified. Custom naming bounds, CLI schema control and current broad custom
object access are candidate defaults, not observed reference behavior.

## Storage measurement and tradeoff review

[Raw measurements](benchmarks/plan-27-storage.txt), command:

```sh
CANDIDATE_DISPOSABLE=yes TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata -run '^$' -bench BenchmarkStorageLayout -benchtime=1x -count=3 -v
```

PostgreSQL 17 in a local disposable container; Go 1.25.0; linux amd64,
12th Gen Intel Core i7-12700H (as reported by Go). Seed 2701, 100,000 records, 10 tenants,
20/100 text fields, balanced/skewed distribution, 1/8 workers. Three repetitions
passed, 96 operation/configuration series, zero SQL errors. Workload source
records schema/index generation; raw output retains plans, size, p50/p95,
throughput, errors and field-add time. Warmup: 10 operations per worker before
100 measured operations. Throughput includes warmup and is descriptive only.

**Decision:** keep typed relational tables/columns. The balanced 20-field workload
showed repeatable write disadvantages that cross the design's review threshold:
median p95 create 27,233 us typed versus 4,226 us JSONB at one worker; update
45,269 versus 7,215 us at eight workers. The maximum median ratio is 6.44x.
Other measured typed configurations were at most 1.28x JSONB p95, and the
100-field workloads did not repeat that large gap. These host-local results
are not an SLA or a general performance claim. Their cause is not yet established;
do not explain them as a proved storage-engine property.

This is the required written tradeoff review: accept that early-preview write
cost in exchange for native per-field bounds, explicit typed indexes and direct
relational constraints, without building expression-index/type coercion machinery
around JSONB before Plan 28. Both layouts retain database uniqueness and composite
foreign keys. This benchmark measures owner SQL, not RLS overhead or complete
API traffic. Direct restricted-role isolation correctness is a separate mandatory
Task 4 gate. Revisit at measured customer scale, especially standard-table column
growth and tenant table counts; do not advertise multi-tenant scale from this run.
No physical migration was committed before these measurements and this decision.

## Verification

Baseline Go suite passed against disposable PostgreSQL 17. Migration, catalog,
role, client and final acceptance evidence will be recorded as those tasks pass.
