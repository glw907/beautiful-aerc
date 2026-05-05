---
title: Catkin annotation pipeline + spellcheck (Pass 9d)
status: draft
date: 2026-05-05
---

## Goal

Add an annotation layer to Catkin: a generic, range-based pipeline
that overlays decorations on the existing styled render, plus
spellcheck as the first consumer. Squiggles flag misspellings;
`Ctrl+;` opens a suggestion popover.

Scope is `internal/catkin/` only. Library purity holds: no new
non-charm dependencies. Wordlists embed via `//go:embed`; an
optional user wordlist overlays from `~/.config/poplar/wordlist.txt`.

## Non-goals

- Grammar / lint annotators. The interface accommodates them
  (reserved kinds), but no implementation lands in 9d.
- Hunspell subprocess (Track 2 in the starter prompt). Defer.
- Reload-on-mtime for the user wordlist. Loaded once at host
  construction.
- Per-language detection or non-en_US dictionaries.

## Architecture

Two new files in `internal/catkin/`:

- `annotate.go` (+ `annotate_test.go`) — the generic pipeline.
- `spellcheck.go` (+ `spellcheck_test.go`) — the first consumer,
  plus the SymSpell engine and wordlist loader.

Embedded data:

- `internal/catkin/spellcheck/en_US.txt` — frequency-sorted top
  ~50k entries from a permissively-licensed source (SCOWL or
  aspell). Frequency matters for SymSpell ranking.
- `internal/catkin/spellcheck/project.txt` — hand-curated poplar
  terms (Catkin, JMAP, IMAP, lipgloss, bubbletea, viewport, …).
  Additive to en_US.

Two seams exit the package:

- `(*Model).RegisterAnnotator(a Annotator)` — host calls at
  construction.
- `LoadUserWordlist(path string) ([]string, error)` — host calls
  with the resolved XDG path; result is passed into
  `NewSpeller(extra)`. Missing file is not an error (returns nil).

Catkin owns rendering, overlay positioning, idle scheduling, and
the popover. The host wires construction and supplies the user
wordlist path.

## Annotation interface

```go
// Range is a half-open byte-offset range over the raw source.
// Stored as offsets (not row/col) so annotators don't re-derive
// from a moving cursor; Render maps offsets to row/col once.
type Range struct{ Start, End int }

type AnnotationKind int

const (
    KindMisspelling AnnotationKind = iota // reserved: KindGrammar, KindLint
)

type Annotation struct {
    Range   Range
    Kind    AnnotationKind
    Style   lipgloss.Style // visual decoration
    Payload any            // typed per kind
}

type MisspellingPayload struct {
    Word        string
    Suggestions []string // up to 5, frequency-ordered
}

type Annotator interface {
    Name() string                       // registry-order id, popover header
    Annotate(src string) []Annotation   // pure; sorted by Range.Start
}

type AnnotationSet struct {
    All     []Annotation       // sorted by Range.Start
    byRow   map[int]int        // first index of an annotation starting on row
}
```

Composition rule: per-cell decorations stack via lipgloss
inheritance in registry order. For 9d only spellcheck is
registered; the rule is contract, not exercised. Two annotators
flagging an identical range layer their styles (later wins on
attributes that conflict, both apply where they don't).

## Idle scheduling

A debounced 350 ms idle timer drives recomputation. Implementation:

- `Model` carries `srcGen uint64` (incremented on every
  source-mutating Update branch) and `annoGen uint64` (the
  generation the current `AnnotationSet` was computed against).
- Source-mutating updates issue `scheduleAnnotateCmd(srcGen, src,
  annotators)` — a `tea.Tick(350ms)` that fires
  `annotateRequestMsg{gen: srcGen}`.
- On `annotateRequestMsg`, if `msg.gen != m.srcGen` the request
  is stale (more keystrokes arrived) → drop. Otherwise issue a
  `tea.Cmd` that runs all `Annotator.Annotate` sequentially over
  the snapshot and returns `annotationsReadyMsg{gen, set}`.
- On `annotationsReadyMsg`, if `msg.gen == m.srcGen` swap in the
  set; else drop.

The pattern matches bubbletea's standard tick + generation guard.
No new tickers, no goroutines outside `tea.Cmd`.

## Spellcheck

### Engine

Hand-rolled SymSpell (~150 LOC; spike both hand-roll and
`github.com/sajari/fuzzy` at plan time, prefer hand-roll if
quality is comparable to keep dep graph clean).

```go
type Speller struct { /* deletion-distance index, frequency map */ }

func NewSpeller(extra []string) (*Speller, error) // embedded + extra
func (s *Speller) Check(word string) bool         // true = known
func (s *Speller) Suggest(word string, n int) []string
```

`extra` accepts user wordlist entries (already de-duped against
embedded). Case folding: lowercase comparison; suggestions
preserve embedded casing.

### Annotator

`spellcheckAnnotator` walks `src` once, identifying word ranges
via the `wordnav.go` rune classifier. For each candidate word:

- **Skip** if inside a fenced code block, an inline code span, or
  a link URL. The mask is computed with Catkin's existing
  `Classify` (block kinds) and `walkSpans` (inline kinds) — the
  spellcheck pass walks both and unions their ranges.
- **Skip** if all-caps and ≤4 runes (heuristic; covers HTTP, JSON,
  IMAP-not-on-list cases without false positives on prose).
- **Skip** if `Speller.Check(word)` returns true.
- Otherwise emit `Annotation{Range, KindMisspelling,
  Style: styles.Squiggle, Payload: MisspellingPayload{Word,
  Suggestions: Suggest(word, 5)}}`.

### Wordlists

- `en_US.txt` and `project.txt` embed via `//go:embed` directives
  in `spellcheck.go`. Format: one word per line, `#` comments,
  blank lines ignored. en_US is frequency-ordered (rank = order);
  project.txt is unordered (treated as max frequency to ensure
  project terms beat similar dictionary words in suggestions).
- User overlay: `~/.config/poplar/wordlist.txt`, one word per
  line, `#` comments, additive only. Loaded once at host
  construction via `LoadUserWordlist`. Missing file → nil, no
  error. Malformed lines (whitespace, empty after comment strip)
  are skipped silently.

## Popover UX

### Trigger and lifecycle

- Open: cursor on a `KindMisspelling` range + `Ctrl+;`.
- Close: `Esc`, selection, or any cursor move that leaves the
  range. Find/replace and the popover are mutually exclusive —
  opening the popover while find is active closes find first.

### Content

- Header line shows the misspelled word in quotes.
- Up to 5 suggestions, frequency-ordered, exactly N rows (no
  blank padding when fewer are returned).
- A single blank separator row sits between the suggestion list
  and the actions row when at least one suggestion is shown.
- 0-suggestion fallback: skip the suggestion list and the
  separator; show only header + actions row.
- Actions row: `a add  i ignore  ␛ close`.

```
┌─ "tradeof" ───────────┐
│ › tradeoff            │
│   trade-off           │
│   tradeoffs           │
│                       │
│ a add  i ignore  ␛ x  │
└───────────────────────┘
```

### Keys

`key.Binding` declarations in `popover.go`:

- `OpenSuggestions`     — `Ctrl+;`
- `CloseSuggestions`    — `Esc`
- `NextSuggestion`      — `Down`, `Ctrl+N`
- `PrevSuggestion`      — `Up`, `Ctrl+P`
- `ApplySuggestion`     — `Enter`
- `JumpApply1..5`       — digits `1` through `5` (jump-and-apply)
- `AddToWordlist`       — `a`
- `IgnoreInSession`     — `i`

`Apply` replaces the misspelled range in source and pushes an
undo entry via `undo.Push`. `AddToWordlist` appends the word to
`~/.config/poplar/wordlist.txt` (creating the file if needed,
0o600), updates the in-memory `Speller`, and re-runs annotators
on the current source. `IgnoreInSession` adds the word to a
per-Model in-memory set consulted by the spellcheck annotator
before `Speller.Check`; the set clears at next `Catkin` rebuild.

### Positioning

- Vertical: below the cursor row if `cursorRow + popoverHeight ≤
  viewportHeight - 1`, else above.
- Horizontal: anchored at the word's left edge, clamped to
  `viewportWidth - popoverWidth`.
- Implementation: row-by-row overlay onto `Render` output, ANSI-
  aware via `ansi.Truncate` + concatenation. Same family as
  `overlayMatch`'s single-row substitution, generalized.

### Style

New `Styles` fields (zero values render unstyled but
positionally correct):

- `Squiggle      lipgloss.Style` — single underline, `Styles.Error`-ish red.
- `Popover       lipgloss.Style` — border + bg.
- `PopoverSelected lipgloss.Style` — highlighted suggestion row.

Host (poplar's compose) maps theme onto these at the boundary.

## Testing

**Unit tests**, table-driven, no assertion libraries:

- `annotate_test.go` — registry order; generation-counter
  stale-drop; merge ordering across multiple annotators; empty
  source no-op; AnnotationSet.byRow index correctness.
- `spellcheck_test.go` — known-word pass-through; misspelling
  detection; code-fence / inline-code / URL skip masks; all-caps
  ≤4 acronym skip; project allowlist hits; user overlay hits;
  suggestion frequency ordering. Engine `Speller.Check`/`Suggest`
  tested over a small fixture wordlist; embedded en_US gets a
  smoke test (size > 40k, sentinel first/last entries).
- `popover_test.go` — positioning above/below flip at viewport
  edge; horizontal clamp at right edge; key dispatch (digits,
  arrows, `a`, `i`, `Esc`, `Enter`); apply replaces range and
  pushes undo; dynamic height for 1–5 suggestions; 0-suggestion
  fallback shape; close-on-cursor-leave.

**Live verification** (mandatory per pass-end checklist) — 80×24
and 120×40 tmux captures of:

- Idle squiggle render after typing "tradeof".
- Popover below cursor (cursor in upper half).
- Popover above cursor (cursor near bottom).
- Popover with 2 suggestions (sized to content).
- Popover with 0 suggestions (header + actions only).
- Code-fence skip — no squiggle on `func` inside a fenced Go block.
- `a` round-trip — add to user wordlist, squiggle disappears.

## Bubbletea conformance (§1b)

- `View()` self-clipped via existing `clipPane`; popover overlay
  never exceeds viewport. Width math via `lipgloss.Width` /
  `displayCells`, never `len()`.
- All annotation work in `tea.Cmd`; no I/O in `View`. Wordlist
  load is bounded one-shot, may run synchronously at host
  construction.
- Keys declared as `key.Binding`, dispatched via `key.Matches`.
- No state mutation in `View()` or in `tea.Cmd` closures.
- Children-to-parent signals via `tea.Msg` types
  (`annotateRequestMsg`, `annotationsReadyMsg`); no callbacks.

## ADRs to be written at pass end

- **0149** — Catkin annotation pipeline (range-based, registry-
  ordered, debounced-tick + generation-counter idle).
- **0150** — Spellcheck (SymSpell, embedded en_US + project,
  optional user overlay, code-aware skip masks, popover UX with
  add/ignore actions).

## Risk and rollback

- SymSpell hand-roll quality risk: spike both implementations at
  plan time over a fixture corpus; pick the better. Fallback to
  `sajari/fuzzy` is a one-import swap if the hand-roll regresses.
- Embedded en_US size adds ~500 KB to the binary. Acceptable
  trade for zero-friction first run.
- The popover is the largest unknown. Keep it isolated to
  `popover.go` so it can be reverted without disturbing the
  pipeline if the UX needs rework.
