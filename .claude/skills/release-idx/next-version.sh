#!/usr/bin/env bash
# Determines the next semantic version from conventional commits since the last tag.
# Bump rules:
#   feat!: / BREAKING CHANGE  → major
#   feat(…):                  → minor
#   fix / chore / docs / …    → patch
set -euo pipefail

LAST_TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]' | head -1)

if [ -z "$LAST_TAG" ]; then
  echo "v0.1.0"
  exit 0
fi

VERSION="${LAST_TAG#v}"
MAJOR=$(echo "$VERSION" | cut -d. -f1)
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)

BUMP="patch"

while IFS= read -r commit; do
  [ -z "$commit" ] && continue
  if echo "$commit" | grep -qE '^(feat|fix)!:|BREAKING CHANGE'; then
    BUMP="major"
    break
  fi
  if [ "$BUMP" != "major" ] && echo "$commit" | grep -qE '^feat(\(.+\))?:'; then
    BUMP="minor"
  fi
done < <(git log "${LAST_TAG}..HEAD" --format="%s")

case "$BUMP" in
  major) NEXT="v$((MAJOR + 1)).0.0" ;;
  minor) NEXT="v${MAJOR}.$((MINOR + 1)).0" ;;
  patch) NEXT="v${MAJOR}.${MINOR}.$((PATCH + 1))" ;;
esac

echo "$NEXT"
