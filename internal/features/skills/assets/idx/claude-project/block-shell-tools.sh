#!/usr/bin/env bash
# idx-search-hook: PreToolCall hook for Claude Code.
# Blocks grep/rg/cat/head/tail as the primary command and redirects to idx.
# Exit 2 = block the tool call and show the message to Claude as feedback.
# Exit 0 = allow the call to proceed.
set -uo pipefail

input=$(cat)

# Extract tool_input.command from the JSON payload.
if command -v jq >/dev/null 2>&1; then
  cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null)
else
  # Fallback without jq: extract the first "command": "..." value.
  cmd=$(printf '%s' "$input" | grep -oE '"command"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"command"[[:space:]]*:[[:space:]]*"\(.*\)"/\1/')
fi

[[ -z "$cmd" ]] && exit 0

# Only inspect the segment before the first pipe — if the prohibited command
# appears AFTER a pipe it is filtering shell output (allowed), not reading files.
first_segment=$(printf '%s' "$cmd" | awk -F'|' '{print $1}')
first_word=$(printf '%s' "$first_segment" | awk '{print $1}')

case "$first_word" in
  grep|egrep|fgrep|rg|ag|ack|pt|ugrep)
    printf '[idx-enforce] Use idx search instead of %s.\n' "$first_word" >&2
    printf '  → idx search "<keywords>" --compact --limit 2 --ext ".<ext>"\n' >&2
    exit 2
    ;;
  git)
    second=$(printf '%s' "$first_segment" | awk '{print $2}')
    if [[ "$second" == "grep" ]]; then
      printf '[idx-enforce] Use idx search instead of git grep.\n' >&2
      printf '  → idx search "<keywords>" --compact --limit 2 --ext ".<ext>"\n' >&2
      exit 2
    fi
    ;;
  cat|head|tail)
    printf '[idx-enforce] Use idx read instead of %s.\n' "$first_word" >&2
    printf '  → idx read --compact <path>\n' >&2
    exit 2
    ;;
  *)
    # All other commands are allowed to proceed.
    ;;
esac

exit 0
