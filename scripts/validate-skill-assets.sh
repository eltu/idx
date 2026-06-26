#!/usr/bin/env bash
# Validates that every idx subcommand and flag referenced in the skill asset
# files exists in the actual CLI. Fails the build if the skill has drifted.
set -euo pipefail

BINARY="${IDX_BINARY:-./bin/idx}"
SKILL_DIR="./internal/features/skills/assets/idx"
SKILL_MD="${SKILL_DIR}/SKILL.md"
COMMANDS_MD="${SKILL_DIR}/references/idx-commands.md"

errors=0

die() { printf "❌  %s\n" "$*" >&2; errors=$((errors + 1)); }
ok()  { printf "✓  %s\n" "$*"; }

# Extract flags between two ## headings.
# Usage: extract_flags_between <file> <start-heading-pattern>
extract_flags_between() {
  local file="$1" start="$2"
  awk "/^## ${start}/{found=1; next} found && /^## /{found=0} found" "$file" \
    | grep -oE '`--[a-z][a-z-]*`' \
    | tr -d '`' \
    | sort -u
}

# Build the binary if it doesn't exist.
if [[ ! -x "$BINARY" ]]; then
  printf "Building %s...\n" "$BINARY"
  make build >/dev/null
fi

# -- 1. Validate subcommands ----------------------------------------------
# Extract all "idx <word>" occurrences from code blocks in both asset files.
idx_help=$("$BINARY" --help 2>&1 || true)

while IFS= read -r subcmd; do
  # Skip meta-words that appear with idx but are not subcommands.
  case "$subcmd" in
    "" | "server") continue ;;
    *) ;;
  esac
  if ! echo "$idx_help" | grep -qE "^\s+${subcmd}\b"; then
    die "subcommand 'idx ${subcmd}' found in skill assets but not in 'idx --help'"
  fi
done < <(
  grep -hE '^\s*idx [a-z]' "$SKILL_MD" "$COMMANDS_MD" \
    | grep -oE 'idx [a-z]+' \
    | awk '{print $2}' \
    | sort -u
)

[[ $errors -eq 0 ]] && ok "all subcommands validated"

# -- 2. Validate idx search flags -----------------------------------------
search_help=$("$BINARY" search --help 2>&1 || true)

while IFS= read -r flag; do
  if ! echo "$search_help" | grep -qE -- "${flag}(\b|=)"; then
    die "search flag '${flag}' found in skill assets but not in 'idx search --help'"
  fi
done < <(extract_flags_between "$COMMANDS_MD" "Search Flags Reference")

[[ $errors -eq 0 ]] && ok "all idx search flags validated"

# -- 3. Validate idx read flags -------------------------------------------
read_help=$("$BINARY" read --help 2>&1 || true)

while IFS= read -r flag; do
  if ! echo "$read_help" | grep -qE -- "${flag}(\b|=)"; then
    die "read flag '${flag}' found in skill assets but not in 'idx read --help'"
  fi
done < <(extract_flags_between "$COMMANDS_MD" "Read Flags Reference")

[[ $errors -eq 0 ]] && ok "all idx read flags validated"

# -- Final result ---------------------------------------------------------
if [[ $errors -gt 0 ]]; then
  printf "\n%d drift error(s) found. Update the skill assets to match the current CLI.\n" "$errors" >&2
  exit 1
fi

printf "\n✅  Skill assets are in sync with the CLI.\n"
