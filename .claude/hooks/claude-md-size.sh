#!/usr/bin/env bash
# Hook: block if CLAUDE.md exceeds 200 lines after edit
# PostToolUse on Edit/Write targeting CLAUDE.md

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

if [[ "$file" == *"/CLAUDE.md" ]]; then
    lines=$(wc -l < "$file" 2>/dev/null || echo 0)
    if (( lines > 200 )); then
        echo "CLAUDE.md is $lines lines (limit: 200). Refactor into .claude/rules/ files." >&2
        exit 2
    fi
fi
exit 0
