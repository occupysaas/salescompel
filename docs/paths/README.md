# Parallel work paths

Several people work on SalesCompel at once. This page splits the rest of the
[first-release roadmap](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md)
(Plans 30–50) into items you can claim, so two people never build the same
thing or start something before what it needs has merged.

Each item has a one-page brief in this folder and an open GitHub issue labeled
`path`. The design behind these rules is
[2026-11-14-salescompel-parallel-paths-design.md](../superpowers/specs/2026-11-14-salescompel-parallel-paths-design.md).

## Free today

Plan 33 has landed. Plans **30**, **31**, **34** and **35** are free to claim.
Plan 36 is reserved for the maintainer, who takes it next, because Plans 37–43
all wait on it.

The live status is always the issue list:
`gh issue list --label path --state open`.

## Tracks

Every item belongs to a track. A track fixes the order of its items and the
part of the code they touch. Only one item per track is in progress at a time,
except for items marked *independent*.

| Track | Items, in order | Code area |
|---|---|---|
| R — Records API | Plan 30 Record lifecycle → Plan 32 Bulk API 2.0 | record write path, `/limits`, headers, bulk jobs and worker |
| W — Composite (independent) | Plan 31 Composite II and sObject Collections | composite and collections endpoints |
| S — Security model | Plan 33 Users, profiles, permission sets → Plan 34 Sharing | users, permissions, access checks in every API path |
| A — Application | Plan 36 Live application shell (maintainer-only) → Plan 42 List views and search → Plan 43 Reports and dashboards | `web/`, UI-facing APIs, search |
| C — CRM objects (each item independent) | Plan 37 Leads and conversion; Plan 38 Opportunities; Plan 39 Products and price books; Plan 40 Activities and notes; Plan 41 Campaigns | standard sales objects and their pages |
| D — Data in and out | Plan 35 Metadata subset → Plan 44 Dry-run report → Plan 45 Data load → Plan 46 Reconciliation and export; Plan 47 Connected extraction (independent) | Metadata API, bundles, import and export |
| E — Release (maintainer-only) | Plan 48 Security review → Plan 49 Release pipeline → Plan 50 First-run docs | whole repository, release tooling |

| Plan | Track | Needs | Brief | Issue |
|---|---|---|---|---|
| 30 Record lifecycle | R | 27, 29 | [brief](plan-30-record-lifecycle.md) | [#20](https://github.com/occupysaas/salescompel/issues/20) |
| 31 Composite II and Collections | W | 29 | [brief](plan-31-composite-collections.md) | [#21](https://github.com/occupysaas/salescompel/issues/21) |
| 32 Bulk API 2.0 | R | 29, 30 | [brief](plan-32-bulk-api.md) | [#22](https://github.com/occupysaas/salescompel/issues/22) |
| 33 Users, profiles, permission sets | S | 26, 27, 29 | [brief](plan-33-users-profiles-permissions.md) | [#23](https://github.com/occupysaas/salescompel/issues/23) |
| 34 Sharing | S | 33 | [brief](plan-34-sharing.md) | [#24](https://github.com/occupysaas/salescompel/issues/24) |
| 35 Metadata subset | D | 28, 33 | [brief](plan-35-metadata-subset.md) | [#25](https://github.com/occupysaas/salescompel/issues/25) |
| 36 Live application shell | A (maintainer-only) | 26, 28, 33 | [brief](plan-36-application-shell.md) | [#26](https://github.com/occupysaas/salescompel/issues/26) |
| 37 Leads and conversion | C | 34, 36 | [brief](plan-37-leads-conversion.md) | [#27](https://github.com/occupysaas/salescompel/issues/27) |
| 38 Opportunities | C | 34, 36 | [brief](plan-38-opportunities.md) | [#28](https://github.com/occupysaas/salescompel/issues/28) |
| 39 Products and price books | C | 38 | [brief](plan-39-products-price-books.md) | [#29](https://github.com/occupysaas/salescompel/issues/29) |
| 40 Activities and notes | C | 36 | [brief](plan-40-activities-notes.md) | [#30](https://github.com/occupysaas/salescompel/issues/30) |
| 41 Campaigns | C | 37, 38 | [brief](plan-41-campaigns.md) | [#31](https://github.com/occupysaas/salescompel/issues/31) |
| 42 List views and search | A | 29, 36 | [brief](plan-42-list-views-search.md) | [#32](https://github.com/occupysaas/salescompel/issues/32) |
| 43 Reports and dashboards | A | 29, 34, 42 | [brief](plan-43-reports-dashboards.md) | [#33](https://github.com/occupysaas/salescompel/issues/33) |
| 44 Dry-run report | D | 35 | [brief](plan-44-dry-run-report.md) | [#34](https://github.com/occupysaas/salescompel/issues/34) |
| 45 Data load | D | 32, 44 | [brief](plan-45-data-load.md) | [#35](https://github.com/occupysaas/salescompel/issues/35) |
| 46 Reconciliation and export | D | 45 | [brief](plan-46-reconciliation-export.md) | [#36](https://github.com/occupysaas/salescompel/issues/36) |
| 47 Connected extraction | D (independent) | 44 | [brief](plan-47-connected-extraction.md) | [#37](https://github.com/occupysaas/salescompel/issues/37) |
| 48 Security review and hardening | E (maintainer-only) | all above | [brief](plan-48-security-review.md) | [#38](https://github.com/occupysaas/salescompel/issues/38) |
| 49 Release pipeline | E (maintainer-only) | 48 | [brief](plan-49-release-pipeline.md) | [#39](https://github.com/occupysaas/salescompel/issues/39) |
| 50 First-run docs | E (maintainer-only) | 49 | [brief](plan-50-first-run-docs.md) | [#40](https://github.com/occupysaas/salescompel/issues/40) |

Plans 25–29 have merged, so a dependency on them is already met.

## Claiming

- **Claim:** comment `claiming` on the item's issue, then mention it in
  Discord `#contributing-coordination`. Only the maintainer can add labels
  today, so the maintainer confirms your claim by adding `claimed` and
  assigning you.
- **A claim request blocks like a claim.** An open issue with a comment
  starting with `claiming`, from someone other than the maintainer
  (`thomasmeadows`), and no `claimed` label is **claim requested**. It counts
  as taken, for that item and for its track, until the maintainer confirms it
  or declines it. This stops two people claiming the same thing before anyone
  has looked. If two people comment `claiming`, the first comment wins.
- **Only the latest claim comment counts.** A later comment starting with
  `released` (from the claimant or the maintainer) or `declined` (from the
  maintainer) cancels every earlier `claiming` comment, and the item is free
  again unless it still has the `claimed` label.
- **Release:** comment `released`. The maintainer also removes `claimed` and
  the assignee. When a claim goes quiet, the maintainer asks first, then
  comments `released` and removes the label.
- **Maintainer-only items** can't be claimed by contributors. A `claiming`
  comment on one is ignored.
- **Finish:** the pull request that completes the item says
  `Closes #<issue>`, so merging it closes the issue.

## Free items

An item is **free** when all of these hold:

- its issue is open, not `claimed` and not claim requested
- every plan it needs has merged (its issue is closed, or it is Plan 29 or
  earlier)
- no other item in its track is claimed or claim requested
- it is not `maintainer-only`, unless the maintainer is the one asking

Independent items (Plan 31, every track C item and Plan 47) ignore the track
rule, and they don't count toward it. A claimed Plan 47 does not block 44, 45
or 46.

## Migrations

Several plans add database migrations. A new migration takes its number when
its branch merges: the next one after the latest on `main`. Never reserve
numbers. The `migrations` check in `.github/workflows/dco.yml` catches a pull
request whose new migration is numbered at or below `main`'s latest. It also
runs on every push to `main`, in case a pull request went stale after its
last check. See [CONTRIBUTING.md](../../CONTRIBUTING.md#workflow).

## Session protocol

Agent sessions follow [AGENTS.md](../../AGENTS.md):

1. Unless you name a task, the agent lists the items with their status (free,
   claimed, claim requested, blocked or maintainer-only) and asks which one to
   work on.
2. On the item you choose, it claims the item, reads the brief, then runs
   brainstorming → spec → plan → implementation.
3. When the pull request merges or work stops, it updates the issue and asks
   what is next.
