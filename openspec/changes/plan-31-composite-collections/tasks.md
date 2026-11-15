# Tasks: plan-31-composite-collections

Implementation order matches `docs/superpowers/plans/2026-11-15-plan-31-composite-collections.md`
Tasks 1-11. Each step is TDD and commits with DCO sign-off.

## 1. Client probe
- [ ] 1.1 Record JSforce Collections request shapes (candidate-side, black box)

## 2. Routes out of http.go; ReadFields
- [ ] 2.1 Pure move of record GET and `/query`
- [ ] 2.2 `ReadFields` and standalone `fields`

## 3. Reference grammar
- [ ] 3.1 Parse, resolve, substitute

## 4. Envelope and routing
- [ ] 4.1 Decoder, limits, routes, prevalidation, re-routing

## 5. Executor
- [ ] 5.1 Error classes, savepoints, dispatch

## 6. Collections writes
- [ ] 6.1 Create, update, upsert, delete

## 7. Collections retrieve
- [ ] 7.1 GET and POST retrieve, chunking

## 8. Collections inside Composite
- [ ] 8.1 Outer/inner allOrNone

## 9. OpenAPI
- [ ] 9.1 Operations, guards, traffic

## 10. Acceptance
- [ ] 10.1 JSforce Collections and raw HTTP Composite, wired into the gates

## 11. Records
- [ ] 11.1 Reference record, sources, roadmap line, PR
