# Tasks: plan-31-composite-collections

Implementation order matches `docs/superpowers/plans/2026-11-15-plan-31-composite-collections.md`
Tasks 1-11. Each step is TDD and commits with DCO sign-off.

## 1. Client probe
- [x] 1.1 Record JSforce Collections request shapes (candidate-side, black box)

## 2. Routes out of http.go; ReadFields
- [x] 2.1 Pure move of record GET and `/query`
- [x] 2.2 `ReadFields` and standalone `fields`

## 3. Reference grammar
- [x] 3.1 Parse, resolve, substitute

## 4. Envelope and routing
- [x] 4.1 Decoder, limits, routes, prevalidation, re-routing

## 5. Executor
- [x] 5.1 Error classes, savepoints, dispatch

## 6. Collections writes
- [x] 6.1 Create, update, upsert, delete

## 7. Collections retrieve
- [x] 7.1 GET and POST retrieve, chunking

## 8. Collections inside Composite
- [x] 8.1 Outer/inner allOrNone

## 9. OpenAPI
- [x] 9.1 Operations, guards, traffic

## 10. Acceptance
- [x] 10.1 JSforce Collections and raw HTTP Composite, wired into the gates

## 11. Records
- [x] 11.1 Reference record, sources, roadmap line, PR
