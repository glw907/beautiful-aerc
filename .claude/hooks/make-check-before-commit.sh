#!/usr/bin/env bash
# Hook: remind to run make check before git commit
# PreToolUse on Bash matching git commit

input=$(cat)
command=$(echo "$input" | jq -r '.tool_input.command // empty')

if echo "$command" | grep -qE '\bgit\s+commit\b'; then
    echo "REMINDER: Run 'make check' (vet + test) before committing." >&2
    exit 0
fi
exit 0
