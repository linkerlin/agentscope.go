#!/usr/bin/env bash
# check_api_diff.sh — exported-API compatibility gate (23.7).
#
# Diffs the exported Go API surface (packages/symbols/signatures via
# internal/apidump) of HEAD against BASE (default: the most recent v* tag;
# CI PRs pass origin/main's merge-base). Removed or changed exports are
# BREAKING unless each offending line appears in the allowlist
# (quality/api_breaking_allowlist.txt), which must accompany a migration
# note in the PR/CHANGELOG. Added exports are safe and merely reported.
#
# Usage: scripts/check_api_diff.sh [base-ref]   # default: latest v* tag
#        scripts/check_api_diff.sh --selftest    # verify the gate itself
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# --- selftest: prove breaking detection and allowlist exemption work ---
if [ "${1:-}" = "--selftest" ]; then
  tmpd="$(mktemp -d)"; trap 'rm -rf "$tmpd"' EXIT
  printf 'mod/pkg\tfunc Kept int\nmod/pkg\tfunc Old string\n' > "$tmpd/base"
  printf 'mod/pkg\tfunc Kept int\nmod/pkg\tfunc New bool\n' > "$tmpd/head"
  breaking="$(comm -23 <(sort -u "$tmpd/base") <(sort -u "$tmpd/head"))"
  [ "$breaking" = "mod/pkg	func Old string" ] || { echo "selftest: breaking detection broken" >&2; exit 1; }
  printf 'mod/pkg\tfunc Old string\n' > "$tmpd/allow"
  grep -Fxq -- "$breaking" "$tmpd/allow" || { echo "selftest: allowlist match broken" >&2; exit 1; }
  grep -Fxq -- "mod/pkg	func Different" "$tmpd/allow" && { echo "selftest: allowlist over-matching" >&2; exit 1; } || true
  echo "api-diff selftest: OK (breaking detected, allowlist exact-match)"
  exit 0
fi

BASE="${1:-}"
if [ -z "$BASE" ]; then
  BASE="$(git describe --tags --match 'v*' --abbrev=0 2>/dev/null || true)"
  if [ -z "$BASE" ]; then
    echo "api-diff: no v* tag found and no base given; skipping" >&2
    exit 0
  fi
fi

ALLOWLIST="quality/api_breaking_allowlist.txt"
WORKTREE="$(mktemp -d /tmp/agentscope-apidiff.XXXXXX)"
trap 'git worktree remove --force "$WORKTREE" > /dev/null 2>&1 || rm -rf "$WORKTREE"' EXIT

git worktree add --quiet --detach "$WORKTREE" "$BASE"

# apidump itself may not exist at BASE (it is newer) — copy the HEAD copy in
# so both sides are measured by the same ruler.
mkdir -p "$WORKTREE/internal/apidump"
cp internal/apidump/*.go "$WORKTREE/internal/apidump/"

dump_head="$(mktemp)"; dump_base="$(mktemp)"
go run ./internal/apidump > "$dump_head"
( cd "$WORKTREE" && go run ./internal/apidump > "$dump_base" ) || {
  echo "api-diff: cannot build apidump at base $BASE (deps fetch?)" >&2
  exit 1
}

# Breaking: lines present at base but absent at HEAD (removed or re-signatured).
breaking="$(comm -23 <(sort -u "$dump_base") <(sort -u "$dump_head"))"
# Safe: lines only at HEAD (additions).
added="$(comm -13 <(sort -u "$dump_base") <(sort -u "$dump_head"))"

allow() { # allow <line> — exact allowlist match (comments/blank ignored)
  [ -f "$ALLOWLIST" ] || return 1
  grep -Fxq -- "$1" "$ALLOWLIST"
}

fail=0
if [ -n "$breaking" ]; then
  while IFS= read -r line; do
    if allow "$line"; then
      echo "api-diff: ALLOWED break: $line"
    else
      echo "api-diff: BREAKING: $line" >&2
      fail=1
    fi
  done <<< "$breaking"
else
  echo "api-diff: no breaking changes vs $BASE"
fi
added_count="$(wc -l <<< "$added" | tr -d ' ')"
echo "api-diff: $added_count added exports (safe) vs $BASE"

if [ "$fail" -ne 0 ]; then
  echo "api-diff: FAILED — breaking exported-API changes must be consciously allowed:" >&2
  echo "  1. add the exact line to $ALLOWLIST" >&2
  echo "  2. document the migration in the PR and CHANGELOG" >&2
  exit 1
fi
echo "api-diff: OK (base $BASE)"
