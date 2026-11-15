# Plan 49 — Release pipeline

Issue: [#39](https://github.com/occupysaas/salescompel/issues/39)

**Maintainer-only.** This item is reserved for the maintainer.

## Track and dependencies

- **Track:** E — Release
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 48](plan-48-security-review.md)
- **Roadmap:** [§3, Plan 49](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Semantic versioning starting at `0.1.0`.
- Reproducible builds with checksums and a container image.
- Upgrade and rollback notes, a changelog and `RELEASING.md`.
- Builds publish only from a release branch, never from `main`, with maintainer approval. Direct work on `main` continues until this plan adds the release branch.

## Out of scope

- Installer and Docker CLI channels (roadmap §5).

## Code areas

Likely to touch:

- release tooling
- `.github/workflows`

## Decisions needed

- Reserved for the maintainer.

## How to start

1. Reserved for the maintainer. Contributors can't claim it (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-49-release-pipeline-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-49-release-pipeline.md`
5. OpenSpec change: `openspec/changes/plan-49-release-pipeline/`
6. Branch on your fork: `plan-49-release-pipeline`. The pull request says `Closes #<issue>`.
