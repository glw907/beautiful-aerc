---
description: Binding facts for poplar's Catkin markdown editor
paths:
  - "internal/catkin/**/*.go"
  - "docs/superpowers/plans/**/*.md"
  - "docs/superpowers/specs/**/*.md"
---

# Poplar Catkin Invariants

Binding facts for `internal/catkin/`. Loaded when editing Catkin
sources or planning passes that touch the editor. The decision
index in `docs/poplar/invariants.md` maps each fact back to its
ADR(s).

## Catkin

- Catkin (`internal/catkin/`) is poplar's markdown-first
  bubbletea editor, library-pure on the charm.land/v2 substrate
  (`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`,
  `charm.land/lipgloss/v2`, `charmbracelet/x/ansi`,
  `alecthomas/chroma/v2`). The reflow primitives are the
  vendored algorithm from `muesli/reflow` — provenance-noted in
  `catkin.go` and `uicore/overlay.go`, not a module dep. Wraps
  `bubbles/textarea` as
  buffer/cursor but owns `View()`. Pure `Classify` + `Reflow`
  (non-whitespace rune count for cursor remap); word nav
  intercepts before textarea; 3-row scroll-off. Live styling
  via Catkin-owned `Styles` (zero = plain): inline priority
  code > bold-italic > bold > italic > link in iA-Writer
  literal-delimiter shape; fences via chroma + viewport-bounded
  `sync.Map`; soft-wrap via `ansi.Hardwrap`. `handleCommand`
  runs ahead of word-nav (smart Enter, list indent,
  `Ctrl+B/I/K/L/Q/Space`, counts). QoL: 50-step undo/redo with
  intra-word coalescing, find/replace, markdown auto-pair
  (suppressed in code), smart URL paste, bracket-match via
  `walkSpans`, `DisplayMode` cycle on `Ctrl+\`.
  ADR-0144, 0145, 0146, 0147.
- Annotation pipeline: pure `Annotator` over raw-source byte
  ranges; 350 ms idle tick + `srcGen` counter drops stale
  results. `RenderAnnotated` splices via `ansiSpliceAtCol`
  against unmodified `plain`. `insertCursorBlock` replaces the
  rune at the cursor column, so column math matches styled
  directly with no cursor-aware shift; an annotation enclosing
  the cursor byte splits its splice around the cursor cell so
  `█` survives. Spellcheck (first consumer): hand-rolled
  `Speller` with `//go:embed`'d `en_US.txt` (~50k) +
  `project.txt`; SymSpell delete-index built lazily under
  `sync.Once`; `LoadUserWordlist` is missing-file-tolerant.
  `spellcheckAnnotator` reuses `isWordRune`, masks fences
  (marker lines too), inline code, link URLs, skips short
  all-caps. Popover on plain `;` over a misspelling
  (`Ctrl+;` is not deliverable); digit/arrow apply, `i`
  session-ignores, `a` appends to `Model.userWordlistPath`;
  cursor leaving auto-closes. ADR-0149, 0150, 0152.
