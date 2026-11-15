#!/usr/bin/env bash
# Usage: scripts/check-migrations.sh <base> <head>
# Fails if a migration added in base..head is numbered at or below the highest
# migration at <base>. Migrations take their number when the branch merges
# (CONTRIBUTING.md); a stale number would not match the database history.
set -euo pipefail

base=${1:?usage: check-migrations.sh <base> <head>}
head=${2:?usage: check-migrations.sh <base> <head>}
dir=internal/migration/migrations

latest=0
while IFS= read -r path; do
	name=${path##*/}
	[[ $name =~ ^([0-9]{4})_ ]] || continue
	n=$((10#${BASH_REMATCH[1]}))
	((n > latest)) && latest=$n
done < <(git ls-tree -r --name-only "$base" -- "$dir/")

bad=0
added=0
while IFS= read -r path; do
	[ -n "$path" ] || continue
	name=${path##*/}
	[[ $name =~ ^([0-9]{4})_ ]] || continue
	added=$((added + 1))
	n=$((10#${BASH_REMATCH[1]}))
	if ((n <= latest)); then
		printf '::error file=%s::migration %s is numbered at or below the base branch latest (%04d); renumber it to %04d or higher and update manifest() in internal/migration/migrate.go\n' \
			"$path" "$name" "$latest" "$((latest + 1))"
		bad=$((bad + 1))
	fi
done < <(git diff --diff-filter=A --name-only "$base" "$head" -- "$dir/")

if [ "$bad" -gt 0 ]; then
	echo "$bad of $added new migration(s) are out of order. See CONTRIBUTING.md, \"Workflow\"."
	exit 1
fi
echo "Migration order ok: $added new migration(s), base latest $(printf '%04d' "$latest")."
