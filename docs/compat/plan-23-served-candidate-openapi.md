# Plan 23 — Served candidate OpenAPI contract

Status: implementation in progress, 2026-11-04. No acceptance/parity decision yet.

[Design](../superpowers/specs/2026-11-04-plan-23-served-candidate-openapi-design.md)
and [implementation plan](../superpowers/plans/2026-11-04-plan-23-served-candidate-openapi.md).

## Source ledger

Public OpenAPI 3.2.1 paths/security/media/reference/schema sections, Ajv JSON Schema
2020-12 version guidance and Go 1.25.0 embed documentation were re-read on
2026-11-04. URLs and read boundaries are appended to sources.yaml. Candidate HTTP,
catalog, select and column metadata source were reviewed locally at base 272883b;
the public repository locators were not fetched as an independent oracle. No
Salesforce observations, examples or third-party implementation were copied.

The canonical YAML was independently authored; its bytes are the single checked-in
source. There is no generated JSON twin or regeneration dependency. Existing
OpenAPI 3.2 minor tooling accepted the minimal 3.2.1 planning probe and the full
candidate document. Request business rules and authorization remain server checks.

Task 1: three document/inventory/provenance/schema tests pass; typecheck and offline
repository validation pass. Authenticated route, traffic validation, full candidate
acceptance and independent review remain pending.
