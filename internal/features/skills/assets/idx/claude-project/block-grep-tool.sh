#!/usr/bin/env bash
# PreToolCall hook: blocks Claude's built-in Grep tool.
# Redirects to idx search.
# Exit 2 = block the tool call and send feedback to the model.
printf '[idx-enforce] Use idx search instead of the built-in Grep tool.\n' >&2
printf '  → idx search "<keywords>" --compact --limit 2 --ext ".<ext>"\n' >&2
exit 2
