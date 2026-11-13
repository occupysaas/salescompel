# Contributing to SalesCompel

Thanks for your interest in SalesCompel. This guide covers how to propose
changes and what every contribution must agree to.

## Before you start

1. **Read [`CLEAN_ROOM.md`](CLEAN_ROOM.md).** It is SalesCompel's black-box
   testing policy: compatibility comes from exercising public interfaces,
   never from proprietary Salesforce internals. Every contribution must
   follow it.
2. **Ask before testing against live Salesforce organizations.** Live
   reference tests run only against organizations the maintainer owns or has
   authorized. Offline tests and the compatibility laboratory need no
   approval.
3. **Agree on the work first.** Open an issue, or ask the maintainer, before
   starting anything substantial, so effort isn't spent on changes that
   can't be merged. Reviews may take a while.

## Rules for every contribution

These apply to everyone. The sections after them add what changes with
experience.

- **Claim the plan before you start.** Say which plan (or bug) you are taking
  in the Discord `#contributing-coordination` channel, or comment on the
  plan's issue, and check nobody else has claimed it. Two people building the
  same plan wastes both of their work.
- **One plan or one bug per pull request.** A plan may land as one pull
  request with a commit per task; do not mix plans or add unrelated fixes.
- **No source file over 500 lines.** `scripts/check-file-size.sh` (run in CI
  by `.github/workflows/file-size.yml`) fails when a new or grown `.go`,
  `.ts`, `.vue`, `.py` or `.sql` file passes 500 lines. Files that were
  already larger are listed in `.filesize-allowlist`; each may stay the same
  or shrink, never grow, and the list itself may only shrink. See
  [How to split a big file](#how-to-split-a-big-file).
- **Tests are required.** New behavior comes with a test that fails without
  it. Put the commands you ran and what they printed in the pull request.
- **Sign off every commit** (`git commit -s`, see below).

### Your first contribution

- Pick a small bug or one task of a plan, claim it, and ask in Discord if
  anything is unclear.
- Fork, branch from `main`, push to your fork and open the pull request.
- A one-file fix with a test is a complete contribution.

### Regular contributors

- Take roadmap items in order and follow the spec, plan, implement steps
  below.
- Keep one pull request open per plan and push each finished task to it.
- Split a file before it reaches the limit, in its own pull request, rather
  than inside a feature.

### Maintainers

- Make the `File size` check required for `main` (Settings, Branches), or it
  is advice rather than a rule.
- Review changes to `scripts/`, `.github/` and `.filesize-allowlist` with
  care: a pull request must not weaken a check or raise its own allowlist
  line.
- When a claim goes quiet, ask the contributor first, then release it.

### How review works

- CI must be green: the platform candidate, the compatibility laboratory and
  the file-size check.
- Post the pull request link in Discord so reviewers see it.
- A maintainer reviews against this list: one plan or bug, a claim, tests,
  the clean-room checklist, a public source for compatibility behavior, and
  no file growth. A strong model may review first (see step 5 below).
- Answer each review comment with a commit or a reply. A maintainer merges.

### How to split a big file

- **Split by responsibility, not by line number.** Move one cohesive group
  (the parser, the validators, the handlers for one resource) to a new file
  in the same package, for example `compile.go` becomes `compile.go` and
  `compile_select.go`. Same package means no import or API change.
- **Split tests along the same seams:** `compile_test.go` becomes
  `compile_select_test.go` and so on; shared helpers go in
  `helpers_test.go`.
- **Move first, change later.** Make the pure move its own commit so review
  can check it with `git diff --color-moved`.
- Then run `sh scripts/check-file-size.sh --write-allowlist` to lower or
  delete the file's line, and commit that too.

Generated files (a `Code generated ... DO NOT EDIT.` header) and anything
under `vendor/`, `node_modules/`, `testdata/` or `dist/` are not counted.

## How work is planned and built

SalesCompel is built with AI coding agents, following a roadmap Claude
created:

- [First release roadmap](docs/superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md):
  the current order of work (Plans 25 onward) and what the first `0.x`
  release must do.
- [Compatibility roadmap](docs/plans/2026-10-14-salesforce-compatibility-roadmap.md)
  and [progress roadmap](docs/plans/2026-10-20-progress-and-remaining-roadmap.md):
  the architecture, milestone gates and the record of earlier plans.

Specs and plans that already exist are under `docs/superpowers/specs/` and
`docs/superpowers/plans/` (earlier plans are in `docs/plans/`). Check them
before starting, and pick up the next unfinished item rather than starting a
new one.

### 1. Set up Claude Code with the Superpowers plugin

Install [Claude Code](https://claude.com/claude-code), then install the
Superpowers plugin, which provides the brainstorming and planning skills:

```
/plugin install superpowers@claude-plugins-official
```

### 2. Write a spec and an implementation plan for each roadmap item

For each item, in roadmap order:

1. **Spec:** run the brainstorming skill (`superpowers:brainstorming`) to
   produce a design spec in `docs/superpowers/specs/`.
2. **Implementation plan:** turn the reviewed spec into a step-by-step plan
   in `docs/superpowers/plans/` using the writing-plans skill
   (`superpowers:writing-plans`).

Follow the existing naming pattern
(`YYYY-MM-DD-plan-<n>-<topic>-design.md` for specs,
`YYYY-MM-DD-plan-<n>-<topic>.md` for plans).

### 3. Use a strong model for specs and plans

Write and review every spec and every implementation plan with a strong
model at **medium effort or higher**. Strong models are Claude Opus 5.5 or
later, Claude Fable 5.2 or later, Astra, or newer models. Address the review
findings and commit the result before continuing.

### 4. Implement the reviewed plan

Implement the reviewed plan with Sol or Opus, for example using
`superpowers:subagent-driven-development` or `superpowers:executing-plans`.

### 5. Review the implementation

A strong model should review the code, but this is recommended rather than
required. Either option is fine:

- **Review it now** with a strong model, ideally a different one from the
  model that wrote the code. A second model brings a different perspective.
- **Finish the implementation and open the pull request.** A strong model
  can review it later, before it is merged.

## Workflow

Contributors work from a fork. Nobody pushes directly to this repository.

1. Fork the repository and create a branch from `main`.
2. Claim the plan or bug (see above), then make your change. Keep each pull
   request focused on one plan or fix.
3. Run the relevant tests, for example `go test ./...`, the compatibility
   laboratory checks (see the README) and
   `scripts/check-candidate-acceptance.sh` for changes that affect the
   candidate, and `sh scripts/check-file-size.sh`. CI (`.github/workflows/`) also runs on every pull request.
   Note the commands and results in the pull request.
4. Sign off every commit (see below).
5. Open a pull request against `main`. Say how the change follows
   `CLEAN_ROOM.md`, and cite the public source or recorded evidence for any
   new or changed compatibility behavior.

## Licence and sign-off (DCO)

SalesCompel is licensed under the [Apache License 2.0](LICENSE). Under section
5 of that licence, any contribution you submit is licensed under the same
terms.

Every commit must also carry a `Signed-off-by` line certifying the
[Developer Certificate of Origin 1.1](https://developercertificate.org/):
that you wrote the change, or otherwise have the right to submit it under
the project's licence. Add it with `-s`:

```bash
git commit -s -m "your message"
```

which appends:

```
Signed-off-by: Your Name <you@example.com>
```

Use your real name and an email address you control. Pull requests with
unsigned commits will not be merged. To sign off commits you've already made,
run `git rebase --signoff main`.
