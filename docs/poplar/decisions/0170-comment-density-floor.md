---
title: Comment-density floor on header-shaped and small public-API packages
status: accepted
date: 2026-05-07
---

## Context

Pass 9j installed T38 (comment-frequency tell — application-code
density should land at 7–9%, library density runs ~15%). Pass 9k.1
applied the §0 rubric to the mail wire + config slice. Five of the
seven packages dropped to or below 9%, but two stayed higher despite
aggressive trimming:

- `internal/mail/` — 21.1% → 17.7%. Wire-types and classifier; mostly
  exported types, sentinel errors, and the `Backend`/`ChangeTracker`
  interface declarations. Required godoc on every exported symbol is
  the bulk of the surviving comment lines.
- `internal/term/` — 16.0% → 12.1%. Small public API
  (`HasNerdFont`, `MeasureSPUACells`, `Resolve`, `IconMode`) plus
  the CPR-parser and font-detection internals. Required godoc on a
  short public surface dominates the file's comment ratio.

§0 + paraphrase test were applied; the residue is godoc that the rubric
requires, not paraphrase that should be deleted.

## Decision

Density on a package whose code is mostly **public-API declarations**
(types, sentinel errors, interfaces) or a **small public surface** has
a structural floor above the 7–9% application-code target. Treat
those packages as exempt from a hard 9% rule.

The acceptance test for these packages is qualitative, not the ratio:

1. Zero paraphrase-of-next-≤5-lines comments (§0 paraphrase test).
2. Zero label-colon godocs and zero markdown-shaped multi-paragraph
   godocs (T40).
3. At most one ADR/RFC cite per godoc.
4. Required exported godocs survive; unexported helpers earn a comment
   only when the name leaves something unsaid (§1 step 2 / §8
   "unobvious" bar).

Bound this exemption: it applies to packages where doc-bearing
declarations dominate the line count — typically wire-types
(`internal/mail/`), narrow capability detectors (`internal/term/`),
and small interface-only packages. Implementation-heavy packages
remain on the 7–9% target.

## Consequences

- T38's avoidance rule keeps its mechanical signal — "doubling the
  ratio is a tell" — for application code. The exemption does not
  weaken T38; it scopes it.
- Pass 9k.2/9k.3/9k.4 should not chase the 9% target on packages
  whose surface is mostly type declarations or small exported APIs.
  Apply §0 + paraphrase + T40 and accept whatever density falls out.
- The voice-check stays grep-tier; semantic density judgment lives
  in the per-pass review and `/simplify`'s voice lens.
- Pass 9k.2 is cache + outbound chain; most of those packages have
  enough code body to land cleanly in the 7–9% range. The exemption
  rarely applies past 9k.1.
