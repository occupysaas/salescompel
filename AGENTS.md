# Agent instructions for SalesCompel

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [CLEAN_ROOM.md](CLEAN_ROOM.md)
before changing anything. Several people work on this project at once. Who is
doing what is tracked in [docs/paths/README.md](docs/paths/README.md).

The maintainer is the GitHub user `thomasmeadows`. Only the maintainer
confirms claims (adds `claimed`) and works on `maintainer-only` items. The
person is the maintainer when the authenticated `gh` user
(`gh api user --jq .login`) is `thomasmeadows`.

## Session protocol

### 1. At session start

If the person has already named a task, do that task and skip this step.

Otherwise, list the roadmap items and ask which one to work on:

```bash
gh issue list -R occupysaas/salescompel --label path --state open --limit 50 \
  --json number,title,labels,assignees,comments
```

Show every item with one status, using the rules in
[docs/paths/README.md](docs/paths/README.md#free-items):

- **free:** it can be claimed now
- **claimed by @user:** it has the `claimed` label
- **claim requested by @user:** it has a comment whose text starts with
  `claiming` (any case), from someone other than the maintainer, and no
  `claimed` label. This counts as taken for the item and its track.
- **blocked by #N, #M:** a plan it needs is still open, or another item in its
  track is claimed or claim requested. List every direct blocker by issue
  number. The README's table maps plan numbers to issues.
- **maintainer-only:** it has that label. Add what it still needs, e.g.
  "maintainer-only (needs #23)". Offer it only when the person is the
  maintainer.

Independent items ignore the track rule and don't count toward it: Plan 31,
every `track:c` item and Plan 47 (which is labeled `track:d`, like 44–46).

Then ask which item to work on. List the free items first.

If `gh` is missing or there is no network, list the briefs in `docs/paths/`
(`plan-*.md`), say that claim status is unknown, and ask the person to check
the issues before starting.

### 2. On the chosen item

1. Claim it: comment `claiming` on its issue. If the person is the
   maintainer, also add the `claimed` label and assign them. Otherwise, don't
   add labels even if you can. Say that the claim stays "claim requested"
   until the maintainer confirms it, and suggest mentioning it in Discord
   `#contributing-coordination`.
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
