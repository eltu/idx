#!/usr/bin/env bash
# PreToolCall hook: blocks Claude's built-in Read tool for files inside a git repo.
# Redirects to idx read --compact <path>.
# Exit 2 = block the tool call and send feedback to the model.
# Exit 0 = allow (file is outside a git repository).
set -uo pipefail

input=$(cat)

if command -v jq >/dev/null 2>&1; then
  file_path=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty' 2>/dev/null)
else
  file_path=$(printf '%s' "$input" \
    | grep -oE '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | head -1 \
    | sed 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
fi

[[ -z "$file_path" ]] && exit 0

# Only block reads of files that live inside a git repository.
dir=$(dirname "$file_path")
git -C "$dir" rev-parse --show-toplevel >/dev/null 2>&1 || exit 0

printf '[idx-enforce] Use idx read instead of the Read tool.\n' >&2
printf '  → idx read --compact %s\n' "$file_path" >&2
exit 2
