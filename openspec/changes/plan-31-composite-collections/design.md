# Design: plan-31-composite-collections

## Context

The narrative source is the Plan 31 Superpowers spec
(`docs/superpowers/specs/2026-11-15-plan-31-composite-collections-design.md`).
This design records the decisions the requirements bind to. Superpowers plan
Tasks 1-11 are the implementation order.

## Goals / Non-Goals

**Goals:**

- One write path: every write through `records.Tx`; one PostgreSQL transaction
  per HTTP request; partial success through pgx nested transactions.
- Three error classes (request, item, storage) with fixed outer effects.
- References resolve against earlier subrequest bodies, appear only in bounded
  URL positions or body strings, and are re-routed after substitution.
- Collections results in the documented `{id, success, errors[{statusCode,
  message, fields}]}` shape; all-or-none uses `ALL_OR_NONE_OPERATION_ROLLED_BACK`.
- Plan 24 acceptance stays green (assertions Plan 31 lifts were updated); `httpHeaders` stays `{}`.

**Non-Goals:**

- Composite Graph and Batch, QueryAll, subrequest headers, general external ID
  upsert, the `Location` header, live Salesforce observation.

## Decisions

1. Standalone record GET `fields` comes from Plan 33 (D33-03); Composite reads share `records.Tx.Read`.
2. Bad `referenceId` is `JSON_PARSER_ERROR`.
3. Omitted Composite `allOrNone` is `false`.
4. Upsert is limited to `Account.Compat_Key__c`.
5. No `Location` header.
6. Retrieve reads in chunks of at most 150 IDs (the SOQL WHERE length limit).
7. Collections upserts lock keys in sorted order.
8. Wrong method on a supported path is item 405; `PUT` and other unsupported
   methods are request errors.
9. References are substituted into the parsed URL position, never re-routed;
   a null, empty or non-string URL value is item 400.
10. An upsert record with `Id`, unknown or repeated parameters, extra body keys
    and trailing JSON are request errors.
11. Embedded null renders as `null`; retrieve rows always include `Id`.
12. Plan 33 visibility applies: hidden objects answer as unknown, hidden fields
    are `INVALID_FIELD`.

## Risks

- Substitution into `q` is raw text, as documented. Queries still run as the
  caller, so the risk is limited to the caller's own data; a test pins it.
- All behavior beyond the Plan 24 pilot observation is documented or candidate
  behavior, not observed Salesforce behavior.
