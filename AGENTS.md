# Agent instructions for SalesCompel

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [CLEAN_ROOM.md](CLEAN_ROOM.md)
before changing anything. Several people work on this project at once. Who is
doing what is tracked in [docs/paths/README.md](docs/paths/README.md).

## Session protocol

### 1. At session start

If the person has already named a task, do that task and skip this step.

Otherwise, list the roadmap items and ask which one to work on:

```bash
gh issue list --label path --state open --limit 50 \
  --json number,title,labels,assignees,comments
```

Show every item with one status, using the rules in
[docs/paths/README.md](docs/paths/README.md#free-items):

- **free:** it can be claimed now
- **claimed by @user:** it has the `claimed` label
- **claim requested by @user:** it has a `claiming` comment but no `claimed`
  label. This counts as taken for the item and its track.
- **blocked by #N:** a plan it needs is still open, or another item in its
  track is claimed or claim requested. Name what blocks it.
- **maintainer-only:** it has that label. Offer it only when the person is the
  maintainer.

Then ask which item to work on. List the free items first.

If `gh` is missing or there is no network, list the briefs in `docs/paths/`
(`plan-*.md`), say that claim status is unknown, and ask the person to check
the issues before starting.

### 2. On the chosen item

1. Claim it: comment `claiming` on its issue. With write access, also add the
   `claimed` label and assign the person. Without write access, say that the
   claim stays "claim requested" until the maintainer confirms it, and suggest
   mentioning it in Discord `#contributing-coordination`.
2. Read its brief and the roadmap section it restates.
3. Work through brainstorming → spec → plan → implementation as usual. The
   brief's "How to start" section gives the file paths and branch name.

### 3. When the work merges or stops

- **If it merged:** the pull request's `Closes #<issue>` closes the issue.
- **If it stopped:** comment on the issue with where it stands, and release
  the claim if the person is not coming back to it.

Then ask what is next, showing the free items again.

## Other rules

- Every commit is signed off (`git commit -s`), see CONTRIBUTING.md.
- New migrations take their number at merge time, see CONTRIBUTING.md,
  "Migration numbers".
- No source file over 500 lines (`sh scripts/check-file-size.sh`).
