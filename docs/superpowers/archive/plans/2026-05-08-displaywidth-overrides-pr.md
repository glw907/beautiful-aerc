# Pass 9w plan — `clipperhouse/displaywidth` `Options.Overrides` PR

Companion to specs:

- `2026-05-08-spua-width-upstream-investigation.md` — why this PR
  is the path forward.
- `2026-05-08-displaywidth-issue-draft.md` — body kernel; adapt
  for the PR description (drop "scope questions for you," tighten
  "Proposal" into "What this PR does," add "Tests" and "Docs").

Pure-implementation pass. No poplar code changes.

## Deliverable

A pull request at `clipperhouse/displaywidth` that adds:

- `Options.Overrides []OverrideRange` and `OverrideRange{Lo, Hi
  rune; Width int}`.
- Consultation of `Overrides` in `graphemeWidth`, ahead of the
  trie lookup. First matching range (by slice order) wins.
- ASCII fast-path gating in `Options.String` / `Options.Bytes`:
  when `Overrides` contains any range that intersects `[0x20,
  0x7E]`, the printable-ASCII shortcut is bypassed so overrides
  in that range fire. Otherwise the fast path is preserved
  bit-for-bit (zero-alloc, current performance).
- Table-driven tests covering SPUA-A, SPUA-B, powerline,
  half-width Kana, multi-range, overlap (first-match), nil/empty
  regression, ASCII-range override, `Width = 0` (zero-width
  override), and Truncate path coverage.
- A new fuzz target that fuzzes input bytes with a small random
  override table; asserts no panic, no infinite loop, and that
  `String(s) == sum(per-grapheme width)` invariant holds.
- README "Overrides" subsection between `EastAsianWidth` and
  "Technical standards." Worked example for the Nerd Font / SPUA
  case. Note about the ASCII fast-path semantics.
- GoDoc on every new exported symbol matching the existing voice
  (terse, declarative, the same shape used for `EastAsianWidth`
  and `ControlSequences`).

## API shape

```go
// Add to Options:
//
//   // Overrides forces the display width of graphemes whose
//   // first rune falls inside one of the listed ranges.
//   // Overrides are consulted before the Unicode trie; the
//   // first matching range wins. A nil or empty slice
//   // preserves the default behavior.
//   //
//   // Overrides apply during grapheme inspection. Inputs that
//   // exit through the printable-ASCII fast path (0x20-0x7E)
//   // skip the per-grapheme path; String and Bytes detect
//   // overlap with that range and disable the fast path when
//   // any override could fire there.
//   Overrides []OverrideRange

// OverrideRange forces a fixed display width for graphemes
// whose first rune falls in [Lo, Hi] (inclusive).
type OverrideRange struct {
    Lo, Hi rune
    Width  int
}
```

## Implementation notes

`graphemeWidth[T ~string | []byte](s T, options Options) int` —
the override consultation is the first step after the empty
guard, sitting before the C0/C1 control checks. We decode the
leading rune via a small generic helper (type-switch over `T` to
choose `utf8.DecodeRuneInString` vs `utf8.DecodeRune`; the switch
disappears under monomorphization). Cost when `Overrides` is nil
is one slice-len check that branch-predicts false.

`Options.String` / `Options.Bytes` — at function entry, compute
`fastPath := !overridesIntersectASCII(options.Overrides)`; pass
`fastPath` to the loop so the printable-ASCII shortcut is
gated. The intersection check is one pass over a slice that is
typically length 0–3.

`Options.Rune` already routes through `graphemeWidth`, so it
inherits the override behavior with no additional change.

`TruncateString` / `TruncateBytes` route every grapheme through
`graphemeWidth`, so overrides apply there transparently. No
fast-path concern (truncate has none).

ASCII fast-path gate trades a tiny scan for correctness when a
caller overrides printable ASCII (an unusual but legitimate case
— e.g. forcing `Tab` to width 4 by overriding `[0x09, 0x09]`,
or reclassifying `[` as zero-width for a layout system). Tabs
are already in `[0, 0x1F]` which doesn't go through the fast
path, but the principle matters.

## PR description outline

Title: `Add Options.Overrides for declarative per-range width
overrides`

Body sections (adapted from issue-draft):

1. Problem — three motivating cases (Nerd Font SPUA, half-width
   Kana per lipgloss #666, powerline). One paragraph.
2. What this PR does — `Overrides` shape, consultation order,
   ASCII fast-path note.
3. Tests — list of cases covered, fuzz target.
4. Docs — README section, GoDoc on every exported symbol.
5. Performance — nil-overrides path is unchanged; benchmark
   evidence (`go test -bench=. -benchmem` before/after on the
   existing benchmarks shows the fast paths still zero-alloc).
6. Use case — poplar / lipgloss #666 / Ghostty discussions.
7. Open to redirection — API shape and naming. Happy to revise.

## Pass-end on poplar

Specs (`2026-05-08-spua-width-upstream-investigation.md` and
`2026-05-08-displaywidth-issue-draft.md`) and this plan move to
their archives. STATUS records the PR URL and queues Pass 9q as
the new current pass. No ADR (the pre-1.0 stack is unchanged
until upstream lands).
