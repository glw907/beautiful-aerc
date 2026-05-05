#!/usr/bin/env bash
# voice-check: commit-gate scan for AI-tells in Go code.
#
# Catalogue: ~/.claude/docs/go-comment-voice.md §7 (T1..T32).
# Scope: tells with reliable mechanical signatures and zero
# false-positives on the current poplar tree (Pass 8.10
# calibration, 2026-05-04). Semantic tells (uniformity, density,
# chorus across functions, defensive checks, single-impl
# interfaces, etc.) are out of scope here — those go through the
# /simplify voice-lens agent.
#
# Exit 1 on any finding; 0 otherwise. Output: file:line: T## rule.
#
# To extend: add a `scan` call. Calibrate first by running the
# pattern across the tree; only land patterns that come back
# clean today.

set -euo pipefail

ROOT=${1:-.}
hits=0

# Skip vendored / generated trees if any appear later. .git is
# excluded by --include filter implicitly via grep -r on .go files.
SCAN_ARGS=(-rEn --include='*.go' --exclude-dir=.git --exclude-dir=vendor)

# scan TELL "regex" "human rule"
scan() {
    local tell=$1 pattern=$2 rule=$3
    local out
    out=$(grep "${SCAN_ARGS[@]}" "$pattern" "$ROOT" 2>/dev/null || true)
    if [[ -n "$out" ]]; then
        while IFS= read -r line; do
            echo "$line: $tell — $rule"
            hits=$((hits + 1))
        done <<< "$out"
    fi
}

# T10: "failed to" chorus in error strings.
scan "T10" \
    '(errors\.New|fmt\.Errorf)\("failed to\b' \
    'error string starts with "failed to" — use a bare noun or context prefix'

# T14: GetX getter prefix on a method declaration.
# Anchored to receiver-method form, so stdlib calls like os.Getenv
# inside function bodies do not match.
scan "T14" \
    '^func \([^)]+\) Get[A-Z]\w*\(' \
    'getter prefixed with Get — drop the prefix'

# T16: Manager / Helper / Util / Service suffix on a type
# declaration. (Service here means the noun-suffix anti-pattern,
# not a domain-meaningful Service interface — there are none of
# those in poplar today.)
scan "T16" \
    '^type \w+(Manager|Helper|Util|Service) (struct|interface)\b' \
    'role-suffix type name — name what it manages/helps/serves'

# T4a: "for now" hedge in a comment.
scan "T4" \
    '//[^/]*\bfor now\b' \
    '"for now" — replace with a real TODO(owner) or delete'

# T4b: bare or stub TODO. Catches // TODO, // TODO:, and TODOs
# whose only payload is a generic verb. A real TODO has either an
# (owner) or substantive text after the colon.
scan "T4" \
    '//\s*TODO\s*:?\s*(improve|fix this|cleanup|clean up|refactor|later)?\s*\.?\s*$' \
    'bare or stub TODO — name an owner or a real decision, or delete'

# T28: over-explanation of standard Go idioms in comments. Narrow
# phrase set chosen to avoid false-positives on legitimate prose.
scan "T28" \
    '//.*\b(use a goroutine to|close the channel when|iterate over (all|the|each)|loop through (the|all|each))\b' \
    'explains standard Go mechanics — comment on why this use is structured the way it is, not what range/goroutine/close do'

# T27: hedging / apologetic doc phrases.
scan "T27" \
    '//.*\b(may not handle|could be improved|might not work|perhaps we|maybe we)\b' \
    'apologetic/speculative comment — state real limitations precisely or delete'

# T33: em dash inside a // comment. Stdlib density is ~0.02/file; the
# Pass 8.11 cleanup brought poplar to zero. Any new em dash in a
# comment is the AI-prose voice leaking back in.
scan "T33" \
    '//[^/]*—' \
    'em dash in comment. Use a period. Reserve em dashes for short comma-like asides.'

# T34: semicolon as clause-joiner inside a // comment. Matches "; " plus
# one more char to skip stylistic trailing semicolons.
scan "T34" \
    '//[^/]*;[[:space:]][^/]' \
    'semicolon clause-joiner in comment. Use a period or rewrite as one sentence.'

# T35: documentation labels inside // comments. Conservative noun list.
# These have no other natural comment use in Go.
scan "T35" \
    '//[[:space:]]*(Preference|Fallback|Priority|Rationale|Caveat|Otherwise|Constraint|Invariant):' \
    'documentation label in code comment. Write prose instead.'

if [[ $hits -gt 0 ]]; then
    echo
    echo "voice-check: $hits finding(s). Catalogue: ~/.claude/docs/go-comment-voice.md §7" >&2
    exit 1
fi

exit 0
