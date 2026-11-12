# Plan 28 — Field types decisions and execution record

Status: in progress, 2026-11-11; no Salesforce compatibility certification.

## Reference review (2026-11-11)

No authorized trial-org session was supplied, so nothing here is a live
observation. Automated fetches of developer.salesforce.com reference pages
returned HTTP 403 during this review. The pages cited in the
[Plan 28 spec](../superpowers/specs/2026-11-11-plan-28-field-types-design.md)
remain unverified unless a row below names a source reached through search
results. No client or plugin implementation code was inspected.

Release-note review: the Winter '27 notes reviewed for Plan 27 remain the
latest general release. No field-type, describe or REST value change after
2026-11-05 was found in accessible sources.

## Candidate behavior status

`documented` means a public Salesforce source states the behavior.
`candidate` means the spec's default, with no confirming source.
`gap` means a known or likely difference that is recorded and not implemented.

| Item | Candidate rule | Status | Source / note |
| --- | --- | --- | --- |
| Date range | 1700-01-01 to 4000-12-31 | documented | [Data Loader date formats](https://developer.salesforce.com/docs/atlas.en-us.264.0.dataLoader.meta/dataLoader/data_loader_dates.htm) (via search): earliest 1700-01-01T00:00:00Z, latest 4000-12-31T00:00:00Z GMT |
| DateTime input | RFC 3339 with `Z` or offset, optional milliseconds | documented | [Valid date and dateTime formats](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/intro_valid_date_formats.htm) (via search): `yyyy-MM-ddTHH:mm:ss.SSS+/-HH:mm` or `…SSSZ`; older docs show `+/-HHmm` |
| DateTime without offset read as UTC | accept | candidate | not documented |
| DateTime output `…ss.000+0000` | fixed format | candidate | matches commonly reported REST output; not confirmed |
| DateTime precision | whole seconds | candidate | storage precision is not documented in accessible sources |
| Numeric rounding | round half-up to the field scale | **disputed** | Community reports conflict: some say API writes keep extra decimals beyond the declared scale ([Fivetran](https://support.fivetran.com/hc/en-us/community/posts/21755243528727-Connector-Improvement-Cast-Salesforce-Decimals-to-Doubles-to-Maintain-Precision), [Nintex](https://community.nintex.com/skuid-nlx-sfx-exp-89/users-can-enter-more-decimal-places-than-the-ui-displays-in-read-mode-68704)). Apex `Decimal.round()` defaults to half-even. The candidate keeps exact storage at declared scale; observe on the trial org before claiming parity. |
| Numeric range error | `NUMBER_OUTSIDE_VALID_RANGE` | candidate | error name in common use; not confirmed here |
| Numeric JSON string accepted | `"12.5"` accepted | candidate | not documented |
| Checkbox explicit null | stored false | candidate | not documented |
| Email grammar | one `@`, dotted domain, no spaces, ≤80 | candidate | exact Salesforce grammar not documented |
| Multi-select normalization | split, trim, dedupe, definition order | candidate | not documented |
| Restricted picklist error | `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST` | candidate | error name in common use |
| Lookup delete options | clear value or prevent deletion | candidate | help article unverified (403) |
| Address REST shape | `street, city, state, postalCode, country` (+ geocodes omitted) | candidate / gap | geocode keys omitted by design |
| Contact.Name | `FirstName LastName` | candidate / gap | locale-dependent order waits for Plan 33 |
| Auto-number backfill | ascending Id order | candidate | Salesforce offers backfill as an option; order undocumented |
| Required on new field | only for empty objects | candidate | candidate restriction |
| Stored-not-enforced fields absent from describe | yes | candidate | design choice (spec §6) |

## Gaps carried by this plan

- Typed WHERE literals for numeric, date, datetime and checkbox filters (Plan 29).
- Account deletion cascading to Contacts (Plan 30); Contact.AccountId keeps restrict.
- Picklist value edits, field edits and deletion (Plan 35).
- State and country picklists, address geocodes, `OtherAddress`.
- Name order by user locale (Plan 33).
- Time fields; multi-currency.
