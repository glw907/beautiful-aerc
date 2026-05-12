#!/usr/bin/env bash
# check-deep: mutation testing across logic-heavy packages.
#
# Runs gremlins (github.com/go-gremlins/gremlins) per package, prints
# the efficacy + mutant-coverage score, and exits 1 if any package
# falls below its configured threshold.
#
# Mutation testing answers "would any test fail if the code were
# wrong?" — the question coverage cannot answer. ADR-0230 (Audit G)
# surfaced 21 useless-test findings dominated by silent-success fakes;
# this gate prevents the regression class from re-entering.
#
# Slow by design (recompile + retest per mutant). Not part of `make
# check`; run at pass-end before consolidation, or nightly.
#
# Scope: pure-logic packages where mutation has high signal. UI is
# excluded — golden-file tests fight the mutation pattern. Network
# backends (mailimap, mailjmap) are excluded for runtime; their
# fakes are the Audit G focus and gremlins-on-fakes is circular.

set -euo pipefail

ROOT=${1:-.}
cd "$ROOT"

if ! command -v gremlins >/dev/null 2>&1; then
    if [ -x "$HOME/go/bin/gremlins" ]; then
        GREMLINS="$HOME/go/bin/gremlins"
    else
        echo "gremlins not installed."
        echo "install: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest"
        exit 2
    fi
else
    GREMLINS=gremlins
fi

# Curated logic-heavy packages. Add as new pure-logic packages land.
# Thresholds are per-package efficacy floors. 0 = informational only
# (calibration phase). Tighten as the suite hardens.
declare -A PKGS=(
    [internal/mailcompose]=0
    [internal/mail]=0
    [internal/cache]=0
    [internal/content]=0
    [internal/filter]=0
    [internal/tidytext]=0
    [internal/mailauth]=0
    [internal/config]=0
)

fail=0
for pkg in "${!PKGS[@]}"; do
    threshold=${PKGS[$pkg]}
    echo
    echo "=== $pkg (threshold: ${threshold}%) ==="
    if ! "$GREMLINS" unleash -t dev \
        --threshold-efficacy "$threshold" \
        --output-statuses lk \
        "./$pkg"; then
        fail=1
    fi
done

exit "$fail"
