#!/usr/bin/env bash
# scripts/build-wordlist.sh — one-shot. Output is committed; this
# script exists for reproducibility, not for the build pipeline.
#
# Sources:
#   - github.com/first20hours/google-10000-english (top 10k by frequency).
#   - github.com/dwyl/english-words/words_alpha.txt (~370k alphabetical fill).
#
# Output is frequency-sorted: ranks 1-10000 from the google list,
# remainder appended alphabetically (treated as floor frequency by
# the SymSpell engine).

set -euo pipefail

OUT="$(dirname "$0")/../internal/catkin/spellcheck/en_US.txt"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL https://raw.githubusercontent.com/first20hours/google-10000-english/master/google-10000-english-no-swears.txt > "$TMP/google.txt"
curl -fsSL https://raw.githubusercontent.com/dwyl/english-words/master/words_alpha.txt > "$TMP/alpha.txt"

# Lowercase + strip non-ASCII-alpha; google list keeps order.
awk '{print tolower($0)}' "$TMP/google.txt" | grep -E '^[a-z]+$' | awk '!seen[$0]++' > "$TMP/freq.txt"

# Alphabetical fill, excluding entries already in freq list.
awk '{print tolower($0)}' "$TMP/alpha.txt" | grep -E '^[a-z]+$' \
	| awk 'NR==FNR{seen[$0]=1; next} !seen[$0]' "$TMP/freq.txt" - \
	| sort -u > "$TMP/fill.txt"

# Cap output at ~50k to bound binary size. 10k freq + first 40k alpha.
head -n 40000 "$TMP/fill.txt" > "$TMP/fill_capped.txt"
cat "$TMP/freq.txt" "$TMP/fill_capped.txt" > "$OUT"

echo "wrote $OUT ($(wc -l < "$OUT") lines)"
