#!/usr/bin/env bash
# check_release.sh — release consistency gate (22.5).
#
# Verifies that the version declared in version.go, the newest CHANGELOG
# entry, the release notes file, and (in release mode) the git tag all
# agree. Usage:
#
#   scripts/check_release.sh              # CI mode: version.go == CHANGELOG head,
#                                         #           release notes exist, README mentions
#   scripts/check_release.sh --tag v2.6.0 # release mode: additionally require the
#                                         # tag to match the declared version
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "release-consistency: $1" >&2; exit 1; }

# --- declared version (version.go) ---
declared="$(sed -n 's/^const Version = "\([^"]*\)".*$/\1/p' version.go)"
[ -n "$declared" ] || fail "cannot parse Version from version.go"

# --- newest CHANGELOG entry ---
changelog_head="$(grep -m1 -E '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md \
  | sed -E 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\].*/\1/')"
[ -n "$changelog_head" ] || fail "no '## [x.y.z]' entry found in CHANGELOG.md"

[ "$declared" = "$changelog_head" ] \
  || fail "version.go ($declared) != newest CHANGELOG entry ($changelog_head)"

# --- release notes file ---
[ -f "RELEASE_NOTES_v${declared}.md" ] \
  || fail "missing RELEASE_NOTES_v${declared}.md"

# --- README mentions the current version ---
grep -q "$declared" README.md \
  || fail "README.md does not mention $declared"

# --- release mode: the tag must exist and match ---
if [ "${1:-}" = "--tag" ]; then
  [ -n "${2:-}" ] || fail "--tag requires the tag name (e.g. --tag v2.6.0)"
  want="v${declared}"
  [ "$2" = "$want" ] || fail "tag $2 does not match declared version (expected $want)"
  git rev-parse -q --verify "refs/tags/$2" >/dev/null \
    || fail "tag $2 does not exist in this repository"
fi

echo "release-consistency: OK (version.go=$declared, CHANGELOG=$changelog_head, notes=RELEASE_NOTES_v${declared}.md)"
