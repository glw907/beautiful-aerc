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
# Per-package efficacy floor = observed - 5pp buffer. Buffer absorbs
# the 1–3pp run-to-run drift gremlins shows from mutant ordering and
# per-run timing variance. Raise a floor only by writing new tests
# that lift the observed number — never by closing the buffer.
#
# Baselines (per-package observed efficacy, floor = observed − 5pp):
#   mailauth     76.00% (Pass 40.3)   filter     82.14% (Pass 40.3)
#   content      78.75% (Pass 40.3)   mail       94.44% (Pass 40.4)
#   cache        77.54% (Pass 40.3)   tidytext   79.07% (Pass 40.3)
#   mailcompose  83.76% (Pass 40.3)   config     83.93% (Pass 40.3)
#
# mail's lone surviving mutant (mock.go:117 `end > total` ↔
# `end >= total`) is the documented equivalent mutant from
# ADR-0235 — cannot be killed without rewriting the clamp.
declare -A PKGS=(
    [internal/mailauth]=71
    [internal/content]=73
    [internal/filter]=77
    [internal/mail]=89
    [internal/cache]=72
    [internal/tidytext]=74
    [internal/mailcompose]=78
    [internal/config]=78
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
