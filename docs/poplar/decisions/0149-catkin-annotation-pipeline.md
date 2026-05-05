---
title: Catkin annotation pipeline — generic range overlay
status: accepted
date: 2026-05-05
---

## Context

Catkin already owns its render (live markdown styling, chroma
fences). Editor decorations beyond the markdown style — squiggles
for spellcheck, lint marks, comment-author colors, link previews —
all overlay the same rendered output and need a shared mechanism.
Re-tokenizing per producer would couple every feature to Catkin's
internal block/span shape.

## Decision

A range-based annotation pipeline lives in
`internal/catkin/annotate.go`:

- `Range{Start, End int}` is half-open over raw-source byte
  offsets. `Annotation{Range, Kind, Style, Payload}` carries one
  decoration. `AnnotationKind` identifies the producer category
  (`KindMisspelling` first; grammar/lint reserved). `Payload any`
  is the typed per-kind payload (`MisspellingPayload` for
  spellcheck).
- `Annotator` interface: `Name() string`, `Annotate(src string)
  []Annotation`. Implementations are pure — no I/O, no goroutine
  kickoff. Heavy work runs on the idle tick path.
- `AnnotationSet` is the per-frame artifact rendering consults:
  `All` sorted by `Range.Start`, plus `byRow []int` (first
  annotation index whose `End` reaches row `r`) and `rowStarts
  []int` cached at construction so `rangesOnRow` skips the prefix
  in O(1) and never re-walks the source.
- Scheduling: `Model.RegisterAnnotator` builds the registry; every
  edit bumps `srcGen` and emits `scheduleAnnotateCmd(gen)`, a
  350 ms `tea.Tick`. The tick fires `annotateRequestMsg{gen}`,
  which (if `gen == m.srcGen`) launches `runAnnotatorsCmd` — pure
  computation off-Update. The result returns as
  `annotationsReadyMsg{gen, set}`; if `gen != m.srcGen` the result
  is dropped. Both stale-drop guards live on the message type, not
  on the producer.
- `RenderAnnotated(src, w, h, top, cur, styles, mode, *AnnotationSet)`
  takes the optional set; `nil` is pass-through. Annotations
  splice via `ansiSpliceAtCol` (ANSI-safe, column-driven from the
  unmodified `plain` line). Cursor-row offsets account for the
  inserted cursor block (`█`).
- The pipeline runs on raw source. Renderer maps offsets to
  row/col once, never inverted.

## Consequences

- Adding a new annotator is one struct + `Annotate` impl + one
  registration call at host setup. No coupling to Catkin internals.
- Stale results never overwrite fresh ones — the generation
  counter is the only synchronization primitive. No mutex on
  `Model.annotations`.
- Pure annotators compose: emitted annotations are concatenated
  and sorted. Composition rules (precedence when two annotators
  flag the same span) are deferred until a second consumer ships;
  for now last-write-wins on the splice when ranges overlap.
- The 350 ms idle debounce means annotations lag typing; that is
  the point. A keystroke-fast annotator (e.g. matching-bracket)
  would not use this pipeline.
