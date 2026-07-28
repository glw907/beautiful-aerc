# ADR-0013: one error seam, structural redaction, slog to a rotated file

Date 2026-07-27. Status: accepted (Phase 4).

## Context

C7: every user-visible error reaches the log through one seam;
every action produces a trace; silent no-ops are bugs. ER-1
requires unexported presentation constructors with a vet-class
check; ER-4 requires bounded logs with a stated redaction policy.

## Decision

`internal/uerr` holds the presentation types with unexported
constructors. The one exported constructor takes (operation, ids,
typed reason, wrapped error), writes the structured log line, and
returns the view value; a custom analyzer in the Phase 5 gate
fails on user-facing error construction anywhere else. Logging is
stdlib `slog` with a JSON handler to
`$XDG_STATE_HOME/poplar/poplar.log`, size-based rotation
hand-rolled (rename at threshold, keep N files; a dependency is
not justified for twenty lines). ER-2's action trace logs every
user intent and outcome at debug. Redaction is structural: log
value types carry no body field at all; address and subject
fields exist only on debug-level types; credentials never enter
a log value type. ER-3's honesty states are UI presentations
backed by the same seam.

## Alternatives considered

- **Errors as plain wrapped errors surfaced ad hoc**: exactly the
  silent-fallback pattern the legacy client shipped and C7 now
  forbids; the seam makes the log line and the toast the same
  event.
- **lumberjack for rotation**: maintained-but-quiet dependency
  for a trivial mechanism; the hand-rolled rotation is smaller
  than its go.mod entry.
- **Policy-based redaction (scrub at write time)**: scrubbing
  fails open on the field someone forgot; absent fields fail
  closed. Types that cannot represent a secret cannot leak it.

## Consequences

The SY-4 failure-class test asserts exactly one log line per
class with operation, ids, and reason. A scripted session's log
reconstructs the action sequence (ER-2's oracle). Debug logging
stays on in the dogfood config, matching current practice.

## Revision 2 (2026-07-27, post-review)

Construction is the surfacing event, stated as a rule: retry
loops do not construct per attempt; a failure surfaces on state
transitions (first failure, class change, recovery). This keeps
ER-1's one-line-per-outcome oracle true and stops a backoff loop
from flooding the log the trace depends on.
