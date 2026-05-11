#!/usr/bin/env bash
# modern-go-check: commit-gate scan for pre-1.21 Go idioms.
#
# Catalogue: ~/.claude/skills/go-conventions/SKILL.md "Modern
# Stdlib Idioms" section, ADR-0196.
# Scope: idioms with reliable mechanical signatures and zero
# false-positives on the current poplar tree. Semantic candidates
# (sync.Once + package var, push-callback iterators, slog
# adoption, errors.Join, cmp.Or) go through /simplify Agent 3.
#
# Soft-warn through the 16-series: exits 0 even on findings so
# `make check` stays green while 16b (mechanical sweep), 16c
# (iter.Seq), and 16d (slog) apply the modern forms. 16d flips
# this to hard-fail (exit 1) after the tree is clean. Output:
# file:line: M## rule. Set MODERN_GO_STRICT=1 to flip locally.
#
# To extend: add a `scan` call. Calibrate first; only land patterns
# that come back clean today after the corresponding apply pass.

set -euo pipefail

ROOT=${1:-.}
hits=0

SCAN_ARGS=(-rEn --include='*.go' --exclude-dir=.git --exclude-dir=vendor)

# scan TAG "regex" "human rule"
scan() {
    local tag=$1 pattern=$2 rule=$3
    local out
    out=$(grep "${SCAN_ARGS[@]}" "$pattern" "$ROOT" 2>/dev/null || true)
    if [[ -n "$out" ]]; then
        while IFS= read -r line; do
            echo "$line: $tag — $rule"
            hits=$((hits + 1))
        done <<< "$out"
    fi
}

# M1: sort.SliceStable / sort.Slice — use slices.SortStableFunc /
# slices.SortFunc with cmp.Compare / cmp.Or. Multi-key sorts
# collapse cleanly.
scan "M1" \
    '\bsort\.Slice(Stable)?\(' \
    'sort.Slice(Stable) — slices.Sort(Stable)Func with cmp.Compare/cmp.Or'

# M2: sort.Strings / sort.Ints / sort.Float64s — slices.Sort works
# on any ordered slice and reads cleaner.
scan "M2" \
    '\bsort\.(Strings|Ints|Float64s)\(' \
    'sort.Strings/Ints/Float64s — slices.Sort'

# M3: for i := 0; i < N; i++ — flag the header; the apply-phase
# agent confirms `i` is unused inside the body before rewriting
# to `for range N`. False positives (i is read) are cheap to skip.
scan "M3" \
    '^[[:space:]]*for[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*:=[[:space:]]*0[[:space:]]*;[[:space:]]*[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]*<' \
    'three-clause for loop — if index is unused, use for range N'

# M4: package-scope `sync.Once`. Tightly anchored to top-level var
# blocks and bare `var x sync.Once`. Struct-field `sync.Once` is
# still legitimate when the once-value lives with its owner.
scan "M4" \
    '^(var[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]+sync\.Once\b|[[:space:]]+[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]+sync\.Once$)' \
    'package-scope sync.Once — sync.OnceValue / sync.OnceFunc'

if [[ $hits -gt 0 ]]; then
    echo
    echo "modern-go-check: $hits finding(s). Catalogue: ~/.claude/skills/go-conventions/SKILL.md (Modern Stdlib Idioms)" >&2
    if [[ "${MODERN_GO_STRICT:-0}" = "1" ]]; then
        exit 1
    fi
fi

exit 0
