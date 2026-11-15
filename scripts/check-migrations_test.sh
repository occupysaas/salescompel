#!/usr/bin/env bash
# Tests for scripts/check-migrations.sh. Builds a throwaway repo and runs the
# check against one branch per case.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/check-migrations.sh"
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cd "$dir"
git init -q -b main .
git config user.email t@example.invalid
git config user.name t
git config commit.gpgsign false
mig=internal/migration/migrations
mkdir -p "$mig"
echo "-- base" > "$mig/0006_x.sql"
echo base > README.md
git add -A && git commit -qm base

fails=0
# run_case <name> <want-exit> <must-contain> <must-not-contain> -- <files to add...>
run_case() {
	local name=$1 want=$2 has=$3 hasnt=$4
	shift 5
	git checkout -q -b "$name" main
	for f in "$@"; do
		mkdir -p "$(dirname "$f")"
		echo "-- $name" > "$f"
	done
	git add -A && git commit -qm "$name"
	local out got=0
	out=$("$script" main "$name" 2>&1) || got=$?
	git checkout -q main
	if [ "$got" -ne "$want" ]; then
		echo "FAIL $name (exit $got, want $want)"; echo "$out" | sed 's/^/  | /'; fails=$((fails + 1)); return
	fi
	if [ -n "$has" ] && ! grep -qF "$has" <<<"$out"; then
		echo "FAIL $name (output lacks $has)"; echo "$out" | sed 's/^/  | /'; fails=$((fails + 1)); return
	fi
	if [ -n "$hasnt" ] && grep -qF "$hasnt" <<<"$out"; then
		echo "FAIL $name (output names $hasnt)"; echo "$out" | sed 's/^/  | /'; fails=$((fails + 1)); return
	fi
	echo "ok $name"
}

run_case no_migrations 0 "" "" -- docs/note.md
run_case next_number 0 "" "" -- "$mig/0007_y.sql"
run_case stale_number 1 "0006_z.sql" "" -- "$mig/0006_z.sql"
run_case two_files_one_stale 1 "0005_w.sql" "0007_y.sql" -- "$mig/0007_y.sql" "$mig/0005_w.sql"

# bad_base: an unknown base ref must not pass silently.
got=0
out=$("$script" 0000000000000000000000000000000000000000 main 2>&1) || got=$?
if [ "$got" -eq 2 ] && grep -qF "not a commit" <<<"$out"; then
	echo "ok bad_base"
else
	echo "FAIL bad_base (exit $got, want 2)"; echo "$out" | sed 's/^/  | /'; fails=$((fails + 1))
fi

if [ "$fails" -gt 0 ]; then
	echo "$fails case(s) failed"
	exit 1
fi
echo "check-migrations tests: all passed"
