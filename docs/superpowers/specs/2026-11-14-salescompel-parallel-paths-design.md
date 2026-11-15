# SalesCompel — Design: parallel work paths for multiple contributors

Status: draft for maintainer approval (2026-11-14).

## Intent

Several people now contribute to SalesCompel: TheArchitectit (Plan 29),
anthonypaddison (#18, and #19 as unrequested Plan 30 groundwork), aither-wzns
(#17), with more offering help in issue #2. The first-release roadmap
(`specs/2026-11-04-salescompel-first-release-roadmap-design.md`, Plans 30–50
remaining) is one ordered queue, so contributors can't tell what is safe to
pick up. Two people have already built the same plan without knowing about
each other (#17). CONTRIBUTING (from #17) tells people to claim "on the plan's
issue", but no plan issues exist.

This design breaks the remaining roadmap into claimable items, each with a
short brief and a GitHub issue. Every agent session starts by asking which
item to work on. It ports zeroauthy's parallel-paths design
(`occupysaas/zeroauthy`, PR #13), adjusted for SalesCompel.

## Goals

- Every remaining roadmap plan (30–50) has a short brief that a contributor
  can claim and turn into a full spec and plan.
- Two contributors never work on colliding plans at the same time, and nobody
  starts a plan before its dependencies have merged.
- Claims are visible on GitHub to the maintainer and every agent.
- An agent session starts by showing the free items and asking which one to
  work on. Finishing an item ends with "what is next?".
- Parallel branches can't merge a migration out of order.

## Non-goals

- Full specs or implementation plans for the items. Contributors write them
  with the existing brainstorming → spec → plan workflow, and each plan's
  "Task 1: observation" rule stays.
- Re-deciding roadmap scope, order or dependencies. The briefs restate the
  roadmap's §3 section and its "Needs" column, nothing more.
- A project board, milestones or other GitHub tooling beyond issues and labels.
- Changing the maintainer's own workflow. Maintainer sessions keep working
  directly on `main` until Plan 49 adds the release branch. Contributors keep
  working from forks, as CONTRIBUTING already says.

## Anti-goals

- A claim system that is so heavy that contributors skip it and go back to
  building in silence.
- Briefs that quietly change roadmap scope. Any scope question goes in the
  brief's "decisions needed" list, not into its scope.

## Approaches considered

1. **One free item per plan:** the most parallel option. But nothing stops two
   people reworking the record write path at once (Plans 30 and 32), or
   someone starting Plan 34 before Plan 33 lands.
2. **About six code-area tracks, one person per track:** prevents collisions,
   but each track becomes a queue behind one holder. With three plans free
   today, a stalled Security track would block 34, 35, 36 and all of Stage C.
3. **Hybrid (chosen, as in zeroauthy):** one brief and one issue per plan.
   Each plan belongs to a track that fixes its order and code area. A plan is
   claimable when its dependencies have merged and no other plan in its track
   is in progress. Plans marked *independent* are limited only by their
   dependencies.

Where claims live: a GitHub issue (chosen) rather than Discord or a status
table in the repo. Agents can read and update issues but can't read Discord.
A repo table would let two people claim in the gap before a PR merges, and it
would conflict constantly. Discord stays for discussion and announcements.

How much each brief says: a short brief (chosen) rather than a full spec up
front. Full specs would need many maintainer rulings today, and the roadmap
already marks which plans need a full design.

## Design

### Tracks and items

Items are named by their existing plan number, which is already unique. Track
letters only group them.

| Track | Items, in order | Code area |
|---|---|---|
| R — Records API | Plan 30 Record lifecycle → Plan 32 Bulk API 2.0 | record write path, `/limits`, headers, bulk jobs and worker |
| W — Composite (independent) | Plan 31 Composite II and sObject Collections | composite and collections endpoints |
| S — Security model | Plan 33 Users, profiles, permission sets → Plan 34 Sharing | users, permissions, access checks in every API path |
| A — Application | Plan 36 Live application shell (**maintainer-only**) → Plan 42 List views and search → Plan 43 Reports and dashboards | `web/`, UI-facing APIs, search |
| C — CRM objects (each item independent) | Plan 37 Leads and conversion; Plan 38 Opportunities; Plan 39 Products and price books; Plan 40 Activities and notes; Plan 41 Campaigns | standard sales objects and their pages |
| D — Data in and out | Plan 35 Metadata subset → Plan 44 Dry-run report → Plan 45 Data load → Plan 46 Reconciliation and export; Plan 47 Connected extraction (independent) | Metadata API, bundles, import and export |
| E — Release (**maintainer-only**) | Plan 48 Security review → Plan 49 Release pipeline → Plan 50 First-run docs | whole repository, release tooling |

Dependencies come from the roadmap's "Needs" column and cross tracks. Each
brief and issue lists its own. The cross-track ones:

- Plan 35 needs 33.
- Plan 36 needs 26, 28 and 33.
- Plans 37 and 38 need 34 and 36. Plan 39 needs 38. Plan 40 needs 36. Plan 41
  needs 37 and 38.
- Plan 42 needs 29 and 36. Plan 43 needs 29, 34 and 42.
- Plan 45 needs 32 and 44. Plan 47 needs 44.
- Plan 48 needs every other plan.

Free on the day this lands: **Plan 30, Plan 31, Plan 33.**

Plan 30's issue links #19 (anthonypaddison's offline limits checks) as
existing groundwork. It does not record a claim for them. They claim like
anyone else.

### Files

- `docs/paths/README.md`: the track table, the claim rules, the migration
  rule and the session protocol. `AGENTS.md`, `CONTRIBUTING.md` and the
  roadmap all link to it.
- `docs/paths/plan-<NN>-<slug>.md`: one brief per item (21 in all), about a
  page each:
  - track, size (Short or Full, from the roadmap) and dependencies
  - scope, restating the roadmap's §3 section for the plan
  - out of scope, from the roadmap's text and §5
  - code areas it is likely to touch
  - decisions it needs from the maintainer (the roadmap's "Full design
    decides" items, plus anything the restated scope leaves open)
  - how to start: spec path
    `docs/superpowers/specs/YYYY-MM-DD-plan-<NN>-<slug>-design.md`, plan path
    `docs/superpowers/plans/YYYY-MM-DD-plan-<NN>-<slug>.md`, OpenSpec change
    `openspec/changes/plan-<NN>-<slug>/`, branch `plan-<NN>-<slug>` on the
    contributor's fork
- `AGENTS.md` (new): the session protocol, plus pointers to CONTRIBUTING,
  CLEAN_ROOM and `docs/paths/`.
- `CLAUDE.md` (new): one line, `@AGENTS.md`, so Claude Code sessions load the
  same protocol.
- `CONTRIBUTING.md`: the claim bullet from #17 changes to "comment `claiming`
  on the plan's issue (listed in `docs/paths/`), then mention it in Discord
  `#contributing-coordination`". A short paragraph on migration numbering goes
  under the workflow steps.
- The roadmap gets one line in §2 pointing to `docs/paths/` for who is doing
  what. Its order and history stay unchanged.

### GitHub

- Labels: `path`, `track:r`, `track:w`, `track:s`, `track:a`, `track:c`,
  `track:d`, `track:e`, `claimed`, `maintainer-only`.
- One open issue per item, titled `Plan <NN> — <name>`. Its body links the
  brief, lists dependencies as issue references (merged plans 25–29 shown as
  done) and states the claim rule in one line.
- **Claiming:** comment `claiming` on the issue. Only the maintainer has write
  access today, so a contributor's claim shows as **claim requested** until
  the maintainer adds `claimed` and assigns them. An agent running with write
  access (the maintainer's) adds the label itself.
- **Releasing:** remove `claimed` and comment. When a claim goes quiet, the
  maintainer asks first, then releases it (as CONTRIBUTING already says).
- **Finishing:** the merging PR says `Closes #<issue>`.
- **A claim request blocks like a claim.** An open issue with a `claiming`
  comment from someone other than the maintainer, and no `claimed` label,
  counts as taken for that item and for its track. Without this, two people
  could claim the same item before the maintainer acts, which is exactly the
  #17 collision. So agents read issue comments as well as labels. A request
  the maintainer turns down is answered with a comment, and the item is free
  again.
- **Free item:** open, not claimed or claim-requested, every dependency issue
  closed (or the dependency is Plan 29 or earlier), no other claimed or
  claim-requested item in the same track, and not `maintainer-only` unless the
  maintainer is the one asking. Exempt items (W, every C item and Plan 47)
  ignore the track rule, and they don't count toward it: a claimed Plan 47
  doesn't block 44, 45 or 46.

### Agent session protocol (in `AGENTS.md`)

- **At session start, unless the person already named a task,** the agent
  runs `gh issue list --label path --state open`. It shows each item as free,
  claimed (by whom), claim requested, or blocked (by what), and asks which one
  to work on. Without `gh` or network access, it lists the briefs in
  `docs/paths/` and says claim status is unknown.
- **On a chosen item,** the agent claims it, reads its brief and the roadmap
  section, then runs brainstorming → spec → plan → execution as today.
- **When the item's PR merges or work stops,** the agent updates the issue and
  asks what is next, showing the free items again.

### Migration numbering

The runner (`internal/migration/migrate.go`) embeds an ordered manifest and
requires the database history to match it exactly by version, name and
checksum. A misnumbered migration therefore fails loudly rather than being
silently skipped. Two branches that both add `0007` also collide in the
manifest. Even so, the collision is found late, so:

- **Rule:** a branch's migration takes its number when the branch merges: the
  next number after `main`'s latest. Rename it before merging. Numbers are
  never reserved.
- **Check:** `scripts/check-migrations.sh <base> <head>` fails, with
  `::error::` lines naming each file, when a PR adds a file under
  `internal/migration/migrations/` numbered at or below the base branch's
  highest number. It runs as a second job, `migrations`, in
  `.github/workflows/dco.yml`. It is not a required check at first. The
  maintainer may add it to the ruleset later.
- **When `main` moves after a PR's last check:** a PR checked green can go
  stale if `main` gains a migration afterwards. In practice, both PRs add an
  entry at the same place in `manifest()` in `migrate.go`. The second one then
  has a merge conflict, has to be rebased, and the check runs again. As a
  backstop, the workflow also runs the check on pushes to `main` (base = the
  previous commit), so a clash that slips through shows up at once. The README
  words the rule as "the check catches this", not as a guarantee.
- **Renaming** a migration means renaming the file and its entry in
  `migrate.go`'s hand-written manifest. Migration 1 is `v1.go`, so the check
  only compares the four-digit prefixes under `internal/migration/migrations/`.
- **Tests** (`scripts/check-migrations_test.sh`, run by the same job): no
  migration → pass; next number → pass; stale number → fail and name it; two
  new files, one stale → fail and name only the stale one.

## Order of work

1. Docs: README, 21 briefs, `AGENTS.md`, `CLAUDE.md`, CONTRIBUTING and roadmap
   pointers.
2. The migration check, its tests and the workflow job.
3. After those are on `main` so links resolve: labels, then the 21 issues.
   Then fill the issue numbers into the briefs and the README table, and
   cross-link dependencies. Steps 1–3 are done by the maintainer's session
   with write access, committing straight to `main` under the current
   workflow. The issue-number backfill is one follow-up commit.
4. Dry run: an agent session with no named task lists Plans 30, 31 and 33 as
   free and everything else as blocked or maintainer-only.

## Left to the builder

- Brief wording, within the restating-only rule.
- Label colours.
- Exact text of `AGENTS.md`, keeping the protocol steps above.

## Calls made in this design (check while reading)

- Plan 36 and Plans 48–50 are maintainer-only. The maintainer confirmed
  this on 2026-11-14. Because 37–43 all wait on 36, the maintainer takes
  Plan 36 as a priority right after Plan 33 lands. Contributors meanwhile have
  30, 31, 33, then 32, 34 and 35. The Plan 36 brief and the README say so.
- Plan 35 sits in the Data track rather than Security, so it can run alongside
  Plan 34 once Plan 33 lands.
- Track C items are each independent, so 37, 38 and 40 can run at once once
  their dependencies land.
- No decision-ID blocks. SalesCompel records decisions in each plan's own
  compat docs.
- `CLAUDE.md` imports `AGENTS.md`, so Claude Code reads the protocol.
