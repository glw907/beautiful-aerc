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
# Invocation uses --timeout-coefficient 10 --workers 1 (ADR-0236).
# Default gremlins timeout (3× base test runtime) is too tight for
# packages whose tests have intrinsic wall-clock waits (mailauth's
# RFC 8628 slow_down test, etc); timed-out mutants are excluded
# from the efficacy denominator and silently inflate the score.
# Coefficient 10 + single worker eliminate the timeout channel so
# each mutant lands as KILLED / LIVED / NOT_COVERED.
#
# Baselines (per-package observed efficacy, floor = observed − 5pp).
# All eight packages measured under the calibrated flag set
# (--timeout-coefficient 10 --workers 1) in Pass 40.5b (ADR-0237):
#   mailauth     99.07%  filter      82.14%
#   content      77.05%  mail        94.44%
#   cache       100.00%  tidytext    79.39%
#   mailcompose  83.20%  config      83.86%
#
# Documented equivalent mutants (cannot be killed without
# functional changes):
#   mail/mock.go:117    `end > total` ↔ `end >= total` (ADR-0235).
#   mailauth/oauth.go:213  `time.Until > 5*time.Minute` boundary
#       — equivalent under any non-clock-controlled test; the
#       5-minute refresh buffer is conservative, not load-bearing
#       (ADR-0237).
declare -A PKGS=(
    [internal/mailauth]=94
    [internal/content]=72
    [internal/filter]=77
    [internal/mail]=89
    [internal/cache]=95
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
        --timeout-coefficient 10 \
        --workers 1 \
        --threshold-efficacy "$threshold" \
        --output-statuses lk \
        "./$pkg"; then
        fail=1
    fi
done

exit "$fail"
