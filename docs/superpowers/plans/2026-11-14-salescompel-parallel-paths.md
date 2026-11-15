# Parallel work paths Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split Plans 30–50 into claimable items with briefs and GitHub issues, guard migration order across parallel branches, and make agent sessions start by asking which item to work on.

**Architecture:**
- **Docs:** `docs/paths/` holds the rules (`README.md`) and one brief per plan.
- **Session rules:** a new `AGENTS.md` holds the session protocol, and `CLAUDE.md` imports it.
- **Migration check:** a new script runs as a second job in the existing `dco` workflow.
- **GitHub:** labels and issues are created after the docs reach `main`, so their links resolve.

**Tech Stack:** Markdown, bash, GitHub Actions, `gh` CLI.

**Spec:** `docs/superpowers/specs/2026-11-14-salescompel-parallel-paths-design.md`

## Global Constraints

- Work directly on `main`. Maintainer sessions don't use feature branches until Plan 49. Every commit is signed off (`git commit -s`) with the session's Co-Authored-By trailer.
- Brief file names: `docs/paths/plan-<NN>-<slug>.md`, one per plan 30–50 (21 files). Slugs are lowercase-hyphenated forms of the roadmap's plan name, e.g. `plan-30-record-lifecycle.md`, `plan-31-composite-collections.md`.
- Tracks, items and order exactly as in the spec's track table: R (30→32), W (31, independent), S (33→34), A (36→42→43), C (37, 38, 39, 40, 41, each independent), D (35→44→45→46; 47 independent), E (48→49→50).
- Maintainer-only: Plans 36, 48, 49, 50. The Plan 36 brief and the README say the maintainer takes Plan 36 right after Plan 33 lands.
- Dependencies come from the roadmap's §2 "Needs" column (`docs/superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md`). Briefs never change scope. Open questions go under "Decisions needed".
- Labels: `path`, `track:r`, `track:w`, `track:s`, `track:a`, `track:c`, `track:d`, `track:e`, `claimed`, `maintainer-only`.
- Issue titles: `Plan <NN> — <roadmap name>`.
- Migrations live in `internal/migration/migrations/NNNN_*.sql`. They are numbered at merge time and never reserved.
- Plain English in all docs, and nothing from Salesforce internals (CLEAN_ROOM.md).

## Review Focus

- **A claim request without a label:** an issue with a `claiming` comment from a non-maintainer and no `claimed` label must show as "claim requested" and block its track (README and AGENTS.md text, checked in the Task 4 dry run).
- **An exempt item claimed:** a claimed Plan 47 must not make 44, 45 or 46 look blocked (README rule, checked in the dry run).
- **Two new migrations, one stale:** the check fails and names only the stale file (Task 1 test `two_files_one_stale`).
- **A push to `main` with no new migration:** the check passes. Also, the `sign-off` job must not run on push events, since there is no PR base there (Task 1 workflow `if:` and test `no_migrations`).
- **No `gh` or no network:** the session protocol still lists the briefs and says claim status is unknown (AGENTS.md text).

---

### Task 1: Migration-order check

**Files:**
- Create: `scripts/check-migrations.sh`
- Create: `scripts/check-migrations_test.sh`
- Modify: `.github/workflows/dco.yml`
- Modify: `CONTRIBUTING.md` (one paragraph after workflow step 3)

**Interfaces:**
- Produces: `scripts/check-migrations.sh <base> <head>`. It exits 0 when every migration file added in `base..head` has a four-digit prefix greater than the highest prefix at `<base>`. Otherwise it exits 1 and prints one `::error::` line per offending file, naming the path.

- [ ] **Step 1: Write the failing test** `scripts/check-migrations_test.sh` (bash, `set -euo pipefail`).
  - It builds a throwaway repo in `mktemp -d` with `internal/migration/migrations/0006_x.sql` on a base commit. It runs each case on its own branch from base and calls the script by absolute path:
    - `no_migrations`: only a README change → exit 0.
    - `next_number`: adds `0007_y.sql` → exit 0.
    - `stale_number`: adds `0006_z.sql` → exit 1, and the output contains `0006_z.sql`.
    - `two_files_one_stale`: adds `0007_y.sql` and `0005_w.sql` → exit 1, and the output contains `0005_w.sql` and not `0007_y.sql`.
  - It prints `ok <case>` or `FAIL <case>` for each case and exits non-zero if any case fails.
- [ ] **Step 2: Run it to make sure it fails.** Run `bash scripts/check-migrations_test.sh`. Expected: FAIL, because the script doesn't exist.
- [ ] **Step 3: Implement `scripts/check-migrations.sh`.**
  - `latest` is the highest prefix from `git ls-tree -r --name-only <base> internal/migration/migrations/`. Treat it as 0 when there are none, and compare as base-10 (strip leading zeros).
  - Added files come from `git diff --diff-filter=A --name-only <base> <head> -- internal/migration/migrations/`.
- [ ] **Step 4: Run the test.** `bash scripts/check-migrations_test.sh` prints four `ok` lines and exits 0. `scripts/check-migrations.sh HEAD~1 HEAD` exits 0 on `main`.
- [ ] **Step 5: Add the `migrations` job to `.github/workflows/dco.yml`.**
  - Triggers become `pull_request:` plus `push: branches: [main]`. The existing `sign-off` job gets `if: github.event_name == 'pull_request'`.
  - The new job runs on `ubuntu-24.04` with `actions/checkout@v4` and `fetch-depth: 0`. It runs the test script first.
  - It then runs the check with `BASE` and `HEAD` taken as follows:
    - on `pull_request`: `github.event.pull_request.base.sha` and `head.sha`
    - on `push`: `github.event.before` and `github.sha`
- [ ] **Step 6: Add the CONTRIBUTING paragraph.** It says: a new migration takes the next number after `main`'s latest when the branch merges. Rename the file and its entry in `internal/migration/migrate.go`'s `manifest()` before merging. Never reserve numbers. `scripts/check-migrations.sh main HEAD` checks this locally.
- [ ] **Step 7: Commit.** `git add scripts/check-migrations.sh scripts/check-migrations_test.sh .github/workflows/dco.yml CONTRIBUTING.md && git commit -s -m "ci: check migration order on pull requests and main"`

### Task 2: Rules page, session protocol and pointers

**Files:**
- Create: `docs/paths/README.md`, `AGENTS.md`, `CLAUDE.md`
- Modify: `CONTRIBUTING.md` (the "Claim the plan before you start" bullet from #17), `docs/superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md` (one line under §2's intro)

**Interfaces:**
- Produces: the anchors `#tracks`, `#claiming` and `#free-items` in `docs/paths/README.md`. Briefs and issues link to them.

- [ ] **Step 1: Write `docs/paths/README.md`** with these sections:
  - **Tracks:** the spec's table. The issue column reads `—` until Task 4.
  - **Claiming:** the spec's claiming, release, finishing and claim-request rules.
  - **Free items:** the spec's rule, including that exempt items ignore the track rule and don't count toward it.
  - **Migrations:** the merge-time numbering rule. It says the check catches clashes, not that it guarantees against them.
  - **Session protocol:** a short form, linking `AGENTS.md`.
  - **Free today:** 30, 31 and 33, plus the Plan 36 priority note.
- [ ] **Step 2: Write `AGENTS.md`.**
  - The three protocol steps from the spec, with the exact command `gh issue list --label path --state open --json number,title,labels,assignees,comments`.
  - The four statuses (free, claimed, claim requested, blocked by #N).
  - The no-`gh` fallback: list `docs/paths/plan-*.md` and say claim status is unknown.
  - Pointers to CONTRIBUTING.md, CLEAN_ROOM.md and `docs/paths/README.md`.
- [ ] **Step 3: Write `CLAUDE.md`** with the single line `@AGENTS.md`.
- [ ] **Step 4: Update CONTRIBUTING and the roadmap.**
  - The claim bullet becomes: comment `claiming` on the plan's issue, listed in `docs/paths/README.md`, then mention it in Discord `#contributing-coordination`. Check that nobody else has claimed it.
  - The roadmap's §2 gains: "Who is working on what, and what is free to claim: [docs/paths](../../paths/README.md)."
- [ ] **Step 5: Verify.** `grep -c 'docs/paths' AGENTS.md CONTRIBUTING.md docs/superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md` reports at least 1 per file. Every relative link in the new files resolves: `grep -o '](\.[^)]*)' docs/paths/README.md AGENTS.md`, then check each target exists.
- [ ] **Step 6: Commit.** `git commit -s -m "docs: add parallel work paths rules and agent session protocol"`

### Task 3: The 21 briefs

**Files:**
- Create: `docs/paths/plan-30-*.md` … `docs/paths/plan-50-*.md`
- Modify: `docs/paths/README.md` (link each track-table entry to its brief)

**Interfaces:**
- Consumes: the README anchors from Task 2.
- Produces: one brief per plan, with these headings exactly, in order: `## Track and dependencies`, `## Scope`, `## Out of scope`, `## Code areas`, `## Decisions needed`, `## How to start`. The first line under the H1 is `Issue: —`; Task 4 replaces it.

- [ ] **Step 1: Write the briefs, one per plan.**
  - Scope restates the roadmap's §3 entry for the plan, without widening it.
  - "Decisions needed" lists the roadmap's "Full design decides" items, and "None known" otherwise.
  - "How to start" gives the spec, plan, OpenSpec change and fork branch paths from the spec's Files section.
  - Maintainer-only briefs say so on the line under `Issue:`.
  - Plan 30's brief names PR #19 as existing groundwork, not as a claim.
- [ ] **Step 2: Verify.**
  - `ls docs/paths/plan-*.md | wc -l` → 21.
  - For each brief, `grep -c '^## ' ` → 6.
  - `grep -L 'Issue: ' docs/paths/plan-*.md` → no output.
  - Spot-check that the dependencies of 37, 43, 45 and 47 match the roadmap's Needs column.
- [ ] **Step 3: Commit and push.** `git commit -s -m "docs: add briefs for Plans 30-50"` then `git push origin main`.

### Task 4: Labels, issues, backfill and dry run

**Files:**
- Modify: `docs/paths/README.md` and all 21 briefs (issue numbers)

**Interfaces:**
- Consumes: the pushed briefs from Task 3.

- [ ] **Step 1: Create the labels** with `gh label create <name> --color <hex> --description <text>` for the 10 labels in Global Constraints. Skip any that already exist.
- [ ] **Step 2: Create the 21 issues** in plan-number order with `gh issue create`.
  - Each gets the title format from Global Constraints and the labels `path`, `track:<x>`, plus `maintainer-only` for 36 and 48–50.
  - Each body links the brief on `main`, lists dependencies (Plans 25–29 as "done", later plans as `#<issue>`; create in plan order so lower numbers exist first), and links `docs/paths/README.md#claiming`.
  - Plan 30's body also links #19.
  - Record the plan → issue number map.
- [ ] **Step 3: Backfill.**
  - Replace `Issue: —` in each brief with `Issue: #<n>`.
  - Fill the README's issue column.
  - Edit issue bodies whose dependencies referenced plans by number (where the plan's issue didn't exist yet) to use issue links.
  - Commit with `git commit -s -m "docs: link parallel work paths to their issues"` and push.
- [ ] **Step 4: Dry run.** In a fresh session with no named task, follow `AGENTS.md`. Expected result:
  - Plans 30, 31 and 33 show as free.
  - 32, 34, 35 and 37–47 show as blocked, naming their open dependencies.
  - 36 and 48–50 show as maintainer-only.
  - Record the output in the final summary. If it differs, fix the README or `AGENTS.md` wording and commit.
