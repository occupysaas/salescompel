# SalesCompel

An open-source, free-to-use Salesforce replacement project, with an initial tenant-isolated Go/PostgreSQL candidate and Vue record workflow.

See the [compatibility roadmap](docs/plans/2026-10-14-salesforce-compatibility-roadmap.md) for the proposed architecture, migration strategy, and milestones.

## License

SalesCompel is licensed under the [Apache License, Version 2.0](LICENSE) (SPDX: Apache-2.0).

Contributions follow the [clean-room policy](CLEAN_ROOM.md).

## Compatibility laboratory

[Plan 1](docs/plans/2026-10-15-plan-1-compatibility-laboratory.md) is implemented as an offline specification and test laboratory. See [setup, live-test prerequisites, and recovery](docs/compat/README.md).

```sh
npm ci --prefix compat/clients/node --ignore-scripts
npm --prefix compat/clients/node run check
```

The [OpenAPI 3.2 REST contract](api/openapi.yaml) is a proposed broader subset; the [Plan 2 implementation guide](docs/compat/plan-2-implementation.md) lists the routes served by the candidate. Official Salesforce schema fixtures retain their own licenses; see [provenance](compat/fixtures/openapi/salesforce/provenance.json). The [Plan 4 pilot baseline](docs/compat/plan-4-reference-baseline.md) records two repeatable live reference captures; no candidate Salesforce compatibility certification has been performed.

## Platform walking slice

[Plan 2](docs/superpowers/plans/2026-10-15-plan-2-platform-walking-slice.md) implements persistent Account/Contact CRUD, describe, limited queries, external-ID upsert, provisioned bearer authentication and a localized Vue editor. See the [setup and verification guide](docs/compat/plan-2-implementation.md). The roadmap also tracks [M1R release-history review](docs/plans/2026-10-14-salesforce-compatibility-roadmap.md#m1r--salesforce-release-notes-and-changelog-review).

The candidate is not production-ready or certified Salesforce-compatible. The eleven-scenario reference baseline is recorded; broader M1 obligations and the wider protocol/security/migration surfaces remain pending.

## Catalog upgrades

The candidate now uses catalog version 2 with 80-character bounds for both synthetic
Account custom fields. Existing databases upgrade transactionally at startup;
oversized values block the upgrade without changing data. Stop old instances and
back up before upgrading. See the [Plan 8 upgrade and rollback guide](docs/compat/plan-8-active-field-metadata.md).
