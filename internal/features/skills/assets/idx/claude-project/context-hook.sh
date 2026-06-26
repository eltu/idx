#!/usr/bin/env bash
# UserPromptSubmit hook — injects idx enforcement rules into each session turn.
# Plain stdout is injected by Claude Code as additional context before the model processes the prompt.
cat <<'EOF'
IMPORTANT: This project uses idx for code search and file reading.
- NEVER use grep, rg, ag, ack, git grep or the built-in Grep tool. Always use: idx search "<keywords>" --compact --limit 2
- NEVER use cat, head, tail or the built-in Read tool. Always use: idx read --compact <path>
- idx runs as a daemon. Go directly to idx commands — no pre-flight checks.
EOF
