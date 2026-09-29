#!/usr/bin/env bash
# check_docs.sh — documentation consistency gate (23.5).
#
# 1. Mirror sync: files that exist in both docs/ and docs-site/docs/ must be
#    byte-identical (single source of truth; docs/ is the upstream copy).
# 2. Constructor existence: every `pkg.NewXxx(` referenced in a docs/*.md Go
#    snippet must exist as `func NewXxx` in that package's directory — this
#    is what catches "documentation calling APIs that do not exist".
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail=0

# --- 1. mirror sync (docs/ ↔ docs-site/docs/) ---
mirrors=(
  "docs/api-reference.md:docs-site/docs/api-reference.md"
  "docs/deployment.md:docs-site/docs/deployment.md"
  "docs/MIGRATION.md:docs-site/docs/MIGRATION.md"
  "docs/A2A.md:docs-site/docs/advanced/a2a.md"
)
for pair in "${mirrors[@]}"; do
  src="${pair%%:*}"; dst="${pair##*:}"
  if [ ! -f "$src" ]; then
    echo "docs-consistency: missing upstream $src" >&2; fail=1; continue
  fi
  if [ ! -f "$dst" ]; then
    echo "docs-consistency: missing mirror $dst (run: cp $src $dst)" >&2; fail=1; continue
  fi
  if ! cmp -s "$src" "$dst"; then
    echo "docs-consistency: $src and $dst diverged (run: cp $src $dst)" >&2; fail=1
  fi
done

# --- 2. constructor existence in Go snippets ---
# pkg dir map: package name used in docs → repository directory.
pkg_dir() {
  case "$1" in
    a2a) echo "a2a" ;;
    gateway) echo "gateway" ;;
    workspace) echo "workspace" ;;
    service) echo "service" ;;
    memory) echo "memory" ;;
    messagebus) echo "messagebus" ;;
    controlplane) echo "controlplane" ;;
    evolver) echo "evolver" ;;
    hub) echo "hub" ;;
    toolkit) echo "toolkit" ;;
    plugin) echo "plugin" ;;
    embedding) echo "embedding" ;;
    tts) echo "tts" ;;
    rag) echo "rag" ;;
    schedule) echo "schedule" ;;
    plan) echo "plan" ;;
    skill) echo "skill" ;;
    config) echo "config" ;;
    *) echo "" ;;
  esac
}

# Extract `pkg.NewXxx(` references from fenced go blocks only.
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
awk '/^```go$/{inblock=1; next} /^```$/{inblock=0} inblock' docs/*.md 2>/dev/null \
  | grep -oE '[a-zA-Z_][a-zA-Z0-9_]*\.New[A-Za-z0-9_]*\(' | sort -u > "$tmp" || true

checked=0
while IFS= read -r ref; do
  pkg="${ref%%.*}"
  fn="${ref#*.}"; fn="${fn%%(*}"      # NewXxx
  dir="$(pkg_dir "$pkg")"
  [ -n "$dir" ] && [ -d "$dir" ] || continue
  checked=$((checked + 1))
  if ! grep -rqE "func ${fn}\(" "$dir" --include="*.go" 2>/dev/null; then
    echo "docs-consistency: $ref referenced in docs but func $fn does not exist in $dir/" >&2
    fail=1
  fi
done < "$tmp"
echo "docs-consistency: checked $checked constructor references"

if [ "$fail" -ne 0 ]; then
  echo "docs-consistency: FAILED" >&2
  exit 1
fi
echo "docs-consistency: OK"
