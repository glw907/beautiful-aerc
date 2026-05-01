#!/usr/bin/env bash
# Hook: cap auto-loaded context docs.
# CLAUDE.md (200 lines) and docs/poplar/invariants.md (400 lines) are both
# pulled into every conversation; growth here is a context-budget tax.

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

check() {
    local path=$1 limit=$2 label=$3
    local lines
    lines=$(wc -l < "$path" 2>/dev/null || echo 0)
    if (( lines > limit )); then
        echo "$label is $lines lines (limit: $limit). Prune before continuing." >&2
        exit 2
    fi
}

case "$file" in
    */CLAUDE.md)
        check "$file" 200 "CLAUDE.md"
        ;;
    */docs/poplar/invariants.md)
        check "$file" 400 "invariants.md"
        ;;
esac

exit 0
