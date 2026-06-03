#!/usr/bin/env bash
# Audits documentation completeness before a release.
# Exits 1 if any gap is found; exits 0 if everything is covered.
set -euo pipefail

FEATURES_DIR="docs/features"
ADR_DIR="docs/adr"
RELEASES_DIR="docs/releases"
ERRORS=0

echo "=== Feature doc coverage ==="
EXPECTED=(init sync status search read inspect destroy version skills server config errors)
for feature in "${EXPECTED[@]}"; do
  if [ -f "${FEATURES_DIR}/${feature}.md" ]; then
    echo "  ✅ ${feature}.md"
  else
    echo "  ❌ MISSING: ${FEATURES_DIR}/${feature}.md"
    ERRORS=$((ERRORS + 1))
  fi
done

echo ""
echo "=== ADR coverage (from CLAUDE.md) ==="
while IFS= read -r line; do
  if echo "$line" | grep -qE '^\- ADR [0-9]+:'; then
    ADR_NUM=$(echo "$line" | grep -oE 'ADR [0-9]+' | grep -oE '[0-9]+' | sed 's/^0*//')
    ADR_PADDED=$(printf '%04d' "${ADR_NUM:-0}")
    ADR_FILE=$(ls "${ADR_DIR}/${ADR_PADDED}-"*.md 2>/dev/null | head -1 || true)
    if [ -n "$ADR_FILE" ]; then
      echo "  ✅ ADR ${ADR_NUM}: $(basename "$ADR_FILE")"
    else
      echo "  ❌ MISSING: ${ADR_DIR}/${ADR_PADDED}-*.md"
      ERRORS=$((ERRORS + 1))
    fi
  fi
done < CLAUDE.md

echo ""
echo "=== Release docs coverage ==="
LAST_TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]' | head -1)
if [ -n "$LAST_TAG" ]; then
  if grep -q "${LAST_TAG}" "${RELEASES_DIR}/index.md"; then
    echo "  ✅ ${LAST_TAG} in releases/index.md"
  else
    echo "  ❌ ${LAST_TAG} not in releases/index.md"
    ERRORS=$((ERRORS + 1))
  fi
  if [ -f "${RELEASES_DIR}/${LAST_TAG}.md" ]; then
    echo "  ✅ ${RELEASES_DIR}/${LAST_TAG}.md"
  else
    echo "  ❌ MISSING: ${RELEASES_DIR}/${LAST_TAG}.md"
    ERRORS=$((ERRORS + 1))
  fi
fi

echo ""
if [ "$ERRORS" -eq 0 ]; then
  echo "✅ All documentation checks passed."
else
  echo ""
  echo "❌ ${ERRORS} gap(s) found — fix before releasing."
  exit 1
fi
