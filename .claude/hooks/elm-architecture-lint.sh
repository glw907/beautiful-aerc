#!/usr/bin/env bash
# Hook: lint internal/ui/ files for Elm Architecture violations
# PostToolUse on Edit/Write targeting internal/ui/**/*.go

input=$(cat)
file=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only check internal/ui/ Go files, skip tests
if [[ "$file" != *"/internal/ui/"* ]] || [[ "$file" == *"_test.go" ]]; then
    exit 0
fi

if [[ ! -f "$file" ]]; then
    exit 0
fi

warnings=""

# Rule 1: No package-level mutable state (var with slice, map, or struct)
# Only flag top-level declarations — lines with no leading whitespace.
if grep -nE '^var\s+\w+\s+((\[\]|\*|map\[)|\w+\{)' "$file" | grep -vq '^//'; then
    warnings+="  Rule 1 violation: Package-level mutable var — move to model struct\n"
fi

# Rule 1: No sync.Mutex in UI code
if grep -nq 'sync\.\(RW\)\?Mutex' "$file"; then
    warnings+="  Rule 2 violation: sync.Mutex in UI code — trust the tea event loop\n"
fi

# Rule 3: No blocking calls in Update or View functions
# Extract function bodies of Update and View, check for blocking patterns
if awk '/^func.*\) Update\(/,/^}/' "$file" | grep -qE '\b(backend|adapter)\.\w+\('; then
    warnings+="  Rule 3 violation: Blocking call in Update — move to tea.Cmd\n"
fi

if awk '/^func.*\) View\(/,/^}/' "$file" | grep -qE '\b(backend|adapter|os\.Open|os\.Read|http\.)\w*\('; then
    warnings+="  Rule 3 violation: Blocking call in View — move to tea.Cmd\n"
fi

# Rule 2/3: No goroutine launches in Update
if awk '/^func.*\) Update\(/,/^}/' "$file" | grep -qE '\bgo\s+(func\b|\w+\()'; then
    warnings+="  Rule 2/3 violation: Goroutine in Update — use tea.Cmd instead\n"
fi

if [[ -n "$warnings" ]]; then
    echo "ELM ARCHITECTURE: $(basename "$file") has violations:" >&2
    echo -e "$warnings" >&2
    echo "  Invoke the elm-conventions skill for correct patterns." >&2
    exit 2
fi

exit 0
