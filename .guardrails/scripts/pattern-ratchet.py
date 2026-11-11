#!/usr/bin/env python3
"""Scan ratchet for the PR gate (pattern scan and semantic scan).

DevGate's guardrails-scan.mjs and semantic-scan.mjs have no baseline: they
fail on ANY finding, including the pre-existing ones on this tree. A
permanently-red gate gets ignored, so this wrapper keeps each gate honest
while the debt is paid down:

  - run the full scan once over a pristine export of the BASE_REF and once on
    the working tree,
  - a finding is NEW only when its occurrence count for (severity, rule, file,
    message) exceeds the base count. NEW findings fail; pre-existing findings
    are counted and reported, never hidden.

This adopts the gates on day one and blocks regressions. It does not waive a
finding in a file this PR edits unless the same finding already existed on the
base ref. Fix the debt forward; the pre-existing count is the backlog.

Usage:
  python3 .guardrails/scripts/pattern-ratchet.py --base-ref origin/main \
      --scanner .devgate/scripts/guardrails-scan.mjs

Environment: NODE_PATH must point at a tree with typescript@5 for the
semantic scan; the pattern scan itself needs only node.
"""
from __future__ import annotations

import argparse
import collections
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

# Two finding formats share the header. Pattern scan:
#   [GUARDRAILS][error] PREVENT-011 path/file.ts:12 — message
# Semantic scan (no severity token; the rule id sits in the first bracket):
#   [GUARDRAILS][SEMANTIC-001] path/file.ts:109 — message
FINDING_RE = re.compile(
    r"^\[GUARDRAILS\]\[(?P<first>[A-Za-z][\w-]*)\] (?:(?P<rule2>[\w-]+) )?"
    r"(?P<file>[^\s:]+):(?P<line>\d+) [—-] (?P<msg>.*)$"
)


def _rule(m: re.Match[str]) -> str:
    return m["rule2"] or m["first"]


def _key(m: re.Match[str]) -> str:
    # (rule, file, message) WITHOUT the line: a line number shifts whenever
    # code above it changes, so keying on it would call every shifted finding
    # "new". Occurrence COUNT per key is what detects a genuinely added finding
    # (a second `any` in an already-flagged file raises that file's count for
    # that message).
    return f"{_rule(m)} {m['file']} {m['msg']}"


def run_scan(scanner: Path, root: Path, argv: list[str],
             env: dict[str, str] | None = None) -> tuple[int, collections.Counter[str], list[str], str]:
    run_env = dict(os.environ)
    if env:
        run_env.update(env)
    proc = subprocess.run([*argv, str(scanner)], cwd=root, capture_output=True,
                          text=True, env=run_env)
    counts: collections.Counter[str] = collections.Counter()
    raw: list[str] = []
    combined = proc.stdout + proc.stderr
    for line in combined.splitlines():
        m = FINDING_RE.match(line)
        if not m:
            continue
        raw.append(line)
        counts[_key(m)] += 1
    return proc.returncode, counts, raw, combined


def scan_at_ref(scanner_rel: str, root: Path, ref: str, argv: list[str],
                env: dict[str, str] | None = None) -> collections.Counter[str]:
    """Scan a pristine export of `ref` so base findings are measured, not assumed."""
    with tempfile.TemporaryDirectory() as td:
        tar = subprocess.run(["git", "-C", str(root), "archive", ref],
                             capture_output=True)
        if tar.returncode != 0:
            raise SystemExit(
                f"pattern-ratchet: cannot archive {ref}: "
                f"{tar.stderr.decode(errors='replace')}")
        subprocess.run(["tar", "-x", "-C", td], input=tar.stdout, check=True)
        # The scanner resolves its project root as the PARENT of the .devgate
        # it runs from (projectRootFor), and overlay/scope come from that root.
        # A symlink would resolve back to the real repo and scan the working
        # tree, so .devgate must be a real COPY inside the export. .guardrails
        # comes from the ref's own archived content when present; the working
        # copy is a fallback only for a ref that predates the overlay.
        devgate = root / ".devgate"
        if devgate.is_dir() and not (Path(td) / ".devgate").exists():
            shutil.copytree(devgate, Path(td) / ".devgate", symlinks=True,
                            ignore=shutil.ignore_patterns("node_modules", ".git"))
        if not (Path(td) / ".guardrails").is_dir():
            shutil.copytree(root / ".guardrails", Path(td) / ".guardrails",
                            symlinks=True)
        # The semantic scanner does `await import("typescript")`, which resolves
        # from the scanner file upward (ESM does not honor NODE_PATH). Give the
        # export the typescript package so the base scan can actually run.
        src_ts = root / "node_modules" / "typescript"
        if src_ts.is_dir() and not (Path(td) / "node_modules" / "typescript").exists():
            (Path(td) / "node_modules").mkdir(exist_ok=True)
            (Path(td) / "node_modules" / "typescript").symlink_to(src_ts)
        export_scanner = Path(td) / scanner_rel
        if not export_scanner.exists():
            raise SystemExit(
                f"pattern-ratchet: scanner missing in export: {scanner_rel}")
        _, counts, _, _ = run_scan(export_scanner, Path(td), argv, env)
        return counts


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base-ref", required=True)
    ap.add_argument("--scanner", default=".devgate/scripts/guardrails-scan.mjs")
    ap.add_argument("--interpreter", default="node",
                    help="program that runs the scanner (node, for either scan)")
    ap.add_argument("--env", action="append", default=[], metavar="K=V",
                    help="extra env for the scanner, e.g. NODE_PATH=./node_modules")
    args = ap.parse_args()
    root = Path(__file__).resolve().parents[2]
    argv = [args.interpreter]
    env = dict(kv.split("=", 1) for kv in args.env)

    rc, _counts, raw, head_out = run_scan((root / args.scanner).resolve(), root, argv, env)
    if "evaluated NOTHING" in head_out or "SKIPPED" in head_out:
        print("pattern-ratchet: FAIL - the scanner did not evaluate the tree "
              "(missing compiler API or skipped); a scan that ran nothing is not "
              "green", file=sys.stderr)
        print(head_out, file=sys.stderr)
        return 1
    if rc == 0 and not raw:
        print("pattern-ratchet: OK - scanner clean")
        return 0
    if not raw:
        print("pattern-ratchet: FAIL - scanner exited non-zero with no parsed "
              "finding; treat as a tooling error, not a pass", file=sys.stderr)
        return 1

    try:
        base = scan_at_ref(args.scanner, root, args.base_ref, argv, env)
    except SystemExit as exc:
        print(f"pattern-ratchet: FAIL - {exc}", file=sys.stderr)
        return 1

    new: list[str] = []
    seen: collections.Counter[str] = collections.Counter()
    for line in raw:
        m = FINDING_RE.match(line)
        if not m:
            continue
        key = _key(m)
        seen[key] += 1
        # The occurrence beyond what the base ref already had is NEW.
        if seen[key] > base.get(key, 0):
            new.append(line)

    pre_existing = len(raw) - len(new)
    if new:
        print(f"pattern-ratchet: FAIL - {len(new)} NEW finding(s) not present on "
              f"{args.base_ref} ({pre_existing} pre-existing, grandfathered):",
              file=sys.stderr)
        for line in new:
            print(f"  {line}", file=sys.stderr)
        return 1
    print(f"pattern-ratchet: OK - {pre_existing} pre-existing finding(s) on "
          f"{args.base_ref}, 0 new")
    return 0


if __name__ == "__main__":
    sys.exit(main())
