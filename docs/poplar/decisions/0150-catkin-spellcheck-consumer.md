---
title: Catkin spellcheck — first annotation consumer
status: accepted
date: 2026-05-05
---

## Context

Catkin needs spellcheck for compose. The constraint set: library-
pure (no subprocess, no service), embedded wordlist (no first-run
fetch), millisecond-class lookups on hand-edited buffers, simple
suggestion algorithm whose code we can read. SymSpell's deletion-
distance index meets all four; hunspell is deferred to a follow-up
pass for users who need richer dictionaries.

## Decision

`internal/catkin/spellcheck.go` ships a hand-rolled
`Speller`:

- Wordlists embedded via `//go:embed`:
  `internal/catkin/spellcheck/en_US.txt` (frequency-sorted top
  ~50k from google-10000-english-no-swears + dwyl/english-words
  fill; reproducible via `scripts/build-wordlist.sh`) and
  `project.txt` (hand-curated poplar terms).
- `LoadUserWordlist(path)` reads optional per-user words; missing
  file is `(nil, nil)` (not an error).
- `NewSpeller(extra []string)` loads embedded + extra into a
  `known map[string]int` (lowercased word → frequency rank).
- `Check(word) bool` — case-insensitive lookup against `known` +
  `ignored`.
- `Suggest(word) []string` — SymSpell deletion-distance index
  (`delIdx map[string][]string`, edit distance ≤2) built lazily
  under `sync.Once`; up to 5 frequency-ordered suggestions.
- `IgnoreInSession(word)` — annotator-side session-only ignore.

Annotator (`spellcheckAnnotator`):

- `NewSpellcheckAnnotator(speller, styles Styles)` — captures
  `Styles.Squiggle` so emitted annotations carry the decoration
  style without the renderer routing by kind.
- `Annotate(src)` walks rune-by-rune via existing `isWordRune`
  classification, builds a skip mask for fenced code blocks
  (including marker lines), inline code spans, and link URL
  portions, skips short all-caps words (acronyms), runs `Check`
  on each remaining word, calls `Suggest` for misses, emits
  `Annotation{Kind: KindMisspelling, Style: Squiggle, Payload:
  MisspellingPayload{Word, Suggestions}}`.

Popover (`internal/catkin/popover.go`):

- Opens on `;` when the cursor is on a misspelling annotation.
  Plain `;` rather than `Ctrl+;` because `Ctrl+;` is not
  deliverable on most terminals. Outside a misspelling, `;` types
  literally.
- Up to 5 suggestions, digit-jump (`1`–`9`) applies; `Tab`/`↑`/`↓`
  + `Enter` selects; `i` ignores in session; `a` adds to user
  wordlist (mutates live `Speller.known`, appends to file at
  `Model.userWordlistPath` if set, swallows I/O errors — the
  in-memory addition is the recoverable fallback).
- Cursor leaving the misspelling range auto-closes (in
  `afterEdit`).
- ANSI-aware row-by-row overlay; positioning clamps to the
  editor's right edge and flips above when below would overflow.

Render (`render.go`):

- `RenderAnnotated` is the primary entry; `Render` is a thin
  back-compat shim. `applyAnnotationsToLine` splices each
  annotation via `ansiSpliceAtCol` over the styled line. Column
  math runs against the unmodified `plain` line; the cursor row
  passes a `cursorByteCol` adjustment so splices at-or-after the
  cursor account for the inserted `█`.

## Consequences

- Spellcheck adds zero deps (SymSpell is hand-rolled). Wordlist
  rebuild is a script run by hand, not by the build.
- `Speller.delIdx` is built once on first `Suggest` (not in
  `NewSpeller`) so accounts that never spell-check pay nothing.
  `sync.Once` guards the build.
- Squiggle decoration is `Styles.Squiggle` (single-underline in
  `Error` foreground) — minimal, iA-Writer-restraint matching the
  rest of Catkin's live styling.
- The spellcheck annotator is the proof point for ADR-0149's
  pipeline. A second consumer (grammar, lint) can land without
  pipeline changes; if a second consumer needs different
  composition rules, that's a pipeline-side decision logged then,
  not now.
- Hunspell-via-subprocess is deferred. The bundled SymSpell+50k
  index covers nominal English compose; users with specialized
  vocabularies extend via `~/.config/poplar/wordlist.txt`.
