---
title: Catkin annotation/spellcheck cleanup — voice + dead code
status: accepted
date: 2026-05-05
---

## Context

Pass 9d landed the Catkin annotation pipeline + spellcheck consumer
(ADR-0149, 0150) in 14 tasks across two coupled subsystems. The
post-hoc audit (ADR-0105 pre-beta posture; CLAUDE.md "pass size
budget") flagged the pass as oversized and queued 9d.1–9d.4 to
re-review the diff under independent lenses. Pass 9d.1 is the
voice + quality lens.

Per-task review during 9d let several anti-patterns through that
the conventions skill catches when applied to a fresh diff: T2
restate-comments on unexported helpers, T5 task-framing in
production doc strings ("Pass 9", "this pass deliberately"),
defensive checks on internal callers, dead code with no current
consumer, hand-rolled helpers that duplicate stdlib, and
duplicated state between `AnnotationSet.rowStarts` and
`render.computeRowOffsets`.

## Decision

The Catkin annotation/spellcheck surface settles on these binding
shapes:

- `Annotator` is a single-method interface: `Annotate(src string)
  []Annotation`. The `Name() string` method is dropped — no
  production caller invoked it. If a future consumer needs
  per-annotator diagnostics it can introduce a typed field on a
  concrete annotator, not on the interface.
- `AnnotationKind` documents only the kinds it actually defines
  (`KindMisspelling`). Speculative future kinds (grammar, lint)
  are not preserved as comment-only placeholders.
- `AnnotationSet.rowStarts` is the canonical per-render
  row-offset table. The `render.computeRowOffsets` duplicate is
  removed; render reads `ann.rowStarts` directly (intra-package).
- `byteOffsetToRune` is removed in favor of inlined
  `utf8.RuneCountInString(src[:byteOff])` at its single call
  site. `len([]rune(s))` patterns in `applySelectedSuggestion`
  are likewise replaced with `utf8.RuneCountInString` to avoid
  per-call slice allocation.
- `min3` is removed in favor of the Go 1.21+ builtin `min`.
- `popoverState.width` no longer shadows `cap` and `max`
  builtins; it uses `maxWidth` as the local constant and `max(...)`
  for the running maximum.
- `applySelectedSuggestion` drops defensive
  `!m.popover.open || r.Start < 0 || r.End > len(src)` checks —
  these guard against caller shapes that don't exist; the popover
  key dispatcher only enters this path when the state is open and
  the wordRange came from a validated annotation.
- Comments default to none on unexported symbols; remaining doc
  comments lead with the why (the cap rationale on
  `popoverState.width`, the SymSpell algorithm summary on
  `buildIndex`) rather than restating the name.

## Consequences

The Catkin diff shrinks by ~420 lines (mostly the dropped 344-line
unreferenced testdata file `spellcheck/testdata/small_words.txt`
and the `fixtureExtra` const). `make check` stays green; tests
were updated in step (`fakeAnnotator` lost its `name` field;
`TestAnnotationSetByRow` was table-driven).

Pass 9d.2 (render-path adversarial review), 9d.3 (golangci-lint),
and 9d.4 (live tmux at edge sizes) remain queued — they audit
different lenses and are kept separate to honor the pass-size
budget.

The pass-size lesson from 9d itself is now codified inline in
`CLAUDE.md` (the 8–12 task / one-ADR ceiling); 9d.1 is small
enough to ship without amendment.
