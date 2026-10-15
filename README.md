# SalesCompel

An open-source, free-to-use Salesforce replacement project, currently in the planning stage.

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

The [OpenAPI 3.0 REST contract](api/openapi.yaml) is a proposed initial subset, not a running server. Official Salesforce schema fixtures retain their own licenses; see [provenance](compat/fixtures/openapi/salesforce/provenance.json). No live Salesforce compatibility certification has been performed.
