## Which plan or bug

<!-- One plan (or one task of a plan) or one bug per pull request. -->

Plan: <!-- e.g. Plan 31, Task 2, with a link to its file under docs/superpowers/plans/ -->
Claimed in: <!-- link to your claim in #contributing-coordination, or the plan's issue -->

## Summary

<!-- What does this change and why? -->

## Checks I ran

<!-- Paste each command and the last line it printed. -->

- [ ] `go test ./...`
- [ ] `sh scripts/check-file-size.sh`
- [ ] `scripts/check-candidate-acceptance.sh` (if the change affects the candidate)
- [ ] Anything else the plan names (compatibility laboratory, web tests):

## Clean-room checklist

- [ ] I followed [CLEAN_ROOM.md](../CLEAN_ROOM.md): compatibility comes from public interfaces, never from proprietary Salesforce internals.
- [ ] New or changed compatibility behavior cites its public source or recorded evidence.
- [ ] No Salesforce code, templates, styles, images, or product copy were copied.

## Licence

- [ ] Every commit is signed off (`git commit -s`) under the Developer Certificate of Origin; see [CONTRIBUTING.md](../CONTRIBUTING.md).
