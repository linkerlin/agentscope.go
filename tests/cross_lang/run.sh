#!/usr/bin/env bash
# Cross-language contract test runner
set -euo pipefail

cd "$(dirname "$0")/../.."

echo "=== Step 1: Generate Go fixtures ==="
go run tests/cross_lang/generate_go.go

echo "=== Step 2: Validate Go fixtures with Python ==="
python tests/cross_lang/validate_py.py

echo "=== Step 3: Generate Python fixtures ==="
python tests/cross_lang/generate_py.py

echo "=== Step 4: Validate Python fixtures with Go ==="
go run tests/cross_lang/validate_go.go

echo ""
echo "=== All cross-language contract tests passed ==="
