# Plan 36 — Live application shell

Issue: —

**Maintainer-only.** This item is reserved for the maintainer.

## Track and dependencies

- **Track:** A — Application
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** Plan 26 (merged), Plan 28 (merged), [Plan 33](plan-33-users-profiles-permissions.md)
- **Roadmap:** [§3, Plan 36](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Turns the exploratory preview's navigation and visual direction into a connected, metadata-driven app.
- Sign-in, lists, record detail and edit, related lists and layouts.
- RTL and keyboard access.
- The Plan 13 visual acceptance left open.
- The preview's "Planned" destinations stay disabled until their plan ships.
- *E2E:* Playwright, with each language in the catalog.

## Out of scope

- The sales objects themselves (Plans 37–41).
- List views and search (Plan 42).
- Reports (Plan 43).

## Code areas

Likely to touch:

- `web/src`
- UI-facing APIs in `internal/platform`

## Decisions needed

- Reserved for the maintainer, who takes it right after Plan 33 lands. Plans 37–43 wait on it.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-36-application-shell-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-36-application-shell.md`
5. OpenSpec change: `openspec/changes/plan-36-application-shell/`
6. Branch on your fork: `plan-36-application-shell`. The pull request says `Closes #<issue>`.
