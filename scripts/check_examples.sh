#!/usr/bin/env bash
# check_examples.sh — example rot gate (23.6).
#
# 1. every example directory must be listed in docs/examples-index.md
#    (no orphan examples),
# 2. every path referenced by the index must exist (no dead links),
# 3. every example compiles, with a per-example time limit.
#
# Usage:
#   scripts/check_examples.sh                 # full gate
#   scripts/check_examples.sh --build-only    # skip index cross-check
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

INDEX="docs/examples-index.md"
BUILD_TIMEOUT="${EXAMPLE_BUILD_TIMEOUT:-60}"
fail=0

# Portable timeout: GNU coreutils on Linux CI; fall back to a watch-dog
# subshell on macOS (no timeout binary by default).
run_with_timeout() { # run_with_timeout <secs> <cmd...>
  local secs="$1"; shift
  if command -v timeout > /dev/null 2>&1; then
    timeout "$secs" "$@"
    return
  fi
  if command -v gtimeout > /dev/null 2>&1; then
    gtimeout "$secs" "$@"
    return
  fi
  "$@" &
  local pid=$!
  ( sleep "$secs"; kill -9 "$pid" 2> /dev/null ) &
  local watchdog=$!
  wait "$pid"
  local rc=$?
  kill "$watchdog" 2> /dev/null
  wait "$watchdog" 2> /dev/null
  return "$rc"
}

# --- 1. no orphans: every examples/* dir is indexed ---
indexed_dirs=$(grep -oE 'examples/[a-zA-Z0-9_]+' "$INDEX" 2>/dev/null | sed 's|^examples/||' | sort -u || true)
for d in examples/*/; do
  name="$(basename "$d")"
  if ! grep -qE "(^|[^a-zA-Z0-9_])${name}([^a-zA-Z0-9_]|$)" <<< "$indexed_dirs" && ! grep -q "examples/$name" "$INDEX"; then
    echo "examples-gate: ORPHAN examples/$name not listed in $INDEX" >&2
    fail=1
  fi
done

# --- 2. no dead links: every indexed example dir exists ---
while IFS= read -r dir; do
  [ -z "$dir" ] && continue
  if [ ! -d "$dir" ]; then
    echo "examples-gate: DEAD LINK $dir referenced in $INDEX" >&2
    fail=1
  fi
done < <(grep -oE 'examples/[a-zA-Z0-9_]+' "$INDEX" 2>/dev/null | sort -u)

# --- 3. every example compiles (time-limited) ---
while IFS= read -r dir; do
  [ -z "$dir" ] && continue
  [ -d "$dir" ] || continue
  if ! run_with_timeout "$BUILD_TIMEOUT" go build -o /dev/null "./$dir/..." > /tmp/example-build.err 2>&1; then
    echo "examples-gate: BUILD FAIL (or timeout>${BUILD_TIMEOUT}s) $dir:" >&2
    head -5 /tmp/example-build.err >&2
    fail=1
  fi
done < <(grep -oE 'examples/[a-zA-Z0-9_]+' "$INDEX" 2>/dev/null | sort -u)

if [ "$fail" -ne 0 ]; then
  echo "examples-gate: FAILED" >&2
  exit 1
fi
echo "examples-gate: OK (indexed examples exist and compile)"
