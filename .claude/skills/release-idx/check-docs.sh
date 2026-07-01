#!/usr/bin/env bash
# Collects changes since the last release tag so the agent can analyse
# documentation gaps. Outputs raw data — the analysis is done by the agent.
set -euo pipefail

NO_RESULTS="(none)"

LAST_TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]' | head -1)

if [[ -z "$LAST_TAG" ]]; then
  echo "No previous tag found — skipping gap analysis."
  exit 0
fi

echo "=== Changes since ${LAST_TAG} ==="
echo ""

echo "--- Commits ---"
git log "${LAST_TAG}..HEAD" --oneline
echo ""

echo "--- Go source files changed (non-test) ---"
git diff "${LAST_TAG}..HEAD" --name-only -- '*.go' | grep -v '_test\.go' || echo "$NO_RESULTS"
echo ""

echo "--- CLI command files changed ---"
git diff "${LAST_TAG}..HEAD" --name-only -- 'internal/app/cli/*.go' | grep -v '_test\.go' || echo "$NO_RESULTS"
echo ""

echo "--- ADR files changed or added ---"
git diff "${LAST_TAG}..HEAD" --name-only -- 'docs/adr/*.md' || echo "$NO_RESULTS"
echo ""

echo "--- Feature docs changed ---"
git diff "${LAST_TAG}..HEAD" --name-only -- 'docs/features/*.md' || echo "$NO_RESULTS"
echo ""

echo "--- CLAUDE.md changed (new ADR decisions?) ---"
git diff "${LAST_TAG}..HEAD" -- CLAUDE.md | grep '^[+-].*ADR' | grep -v '^---\|^+++' || echo "(no ADR changes)"
echo ""

echo "--- Diff stat summary ---"
git diff "${LAST_TAG}..HEAD" --stat -- \
  'internal/app/cli/' \
  'internal/features/' \
  'internal/shared/' \
  'cmd/' \
  'docs/'
