#!/usr/bin/env bash
# Hook: remind to verify filter changes against live email
# PostToolUse on Edit/Write targeting internal/filter/

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

if [[ "$file" == *"/internal/filter/"* && "$file" != *"_test.go" ]]; then
    echo "REMINDER: Verify this filter change against a live email in aerc before committing." >&2
fi
exit 0
