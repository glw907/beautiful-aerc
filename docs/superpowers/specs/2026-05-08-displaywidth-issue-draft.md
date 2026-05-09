# Draft text for `clipperhouse/displaywidth` contribution

Companion to `2026-05-08-spua-width-upstream-investigation.md`.
The body below is the kernel that feeds either an issue or a PR
description. The decision (per pass-end discussion 2026-05-08) is
to go **PR-first**, matching the repo's observed pattern.

Strategy notes (not for posting):

- File on `clipperhouse/displaywidth` first, not `charmbracelet/x/ansi`
  or `lipgloss`. displaywidth is the lowest layer; x/ansi just
  embeds an unexported `*displaywidth.Options`. Until displaywidth
  ships the knob, downstream changes are blocked. Filing higher in
  the stack first repeats the pattern from lipgloss #563 / #666:
  width-related issues at the user-facing layer get stalled.
- Frame the proposal as a general runtime-override hook, not as
  "fix Nerd Fonts." The same hook resolves lipgloss #666 (half-
  width katakana rendering as 2 cells in Kitty/Ghostty) and the
  Ghostty `narrow_symbols` request thread. Broader framing →
  more compelling motivating section → higher land probability.
- Range-table API shape (`Options.Overrides []OverrideRange`),
  not a callback. Mirrors the established `EastAsianWidth` and
  `ControlSequences` precedents — both are declarative `Options`
  fields, both ship as boolean knobs that change classification
  for whole input categories. A `func(string) (int, bool)`
  callback adds an indirect call to the hot path and has no
  precedent in the codebase. The maintainer's recent merges
  show what shape lands quickly.
- **PR-first, not issue-first.** Pattern research found 3/3 recent
  feature PRs came in cold (no associated issue). The repo has no
  issue-first norm. Going straight to PR matches observed culture;
  issue-first would signal mismatched expectations.
- **Match the workflow precisely.** Solo maintainer who uses
  Copilot/AI-assisted review (per `AGENTS.md`). Copilot's review
  pattern focuses on test gaps and GoDoc completeness — both must
  be tight before submission. API shape is never redirected during
  review (0/3 cases observed), so the shape must be right on
  arrival. Reading prior PRs (#20, #21, #22) for tone and
  structural cues before drafting is mandatory.
- The body text below works for both an issue and a PR
  description — the PR will adapt it (drop "Scope questions for
  you," tighten the proposal section into a "What this PR does"
  framing, add a "Tests" and "Docs" section).

---

## Title

`Options.Overrides: declarative per-range width overrides for runtime terminal/font configuration`

## Body

Hi — thanks for `displaywidth`. Filing as an issue first to check
scope before drafting a PR.

### Problem

Terminal cell width is a function of `(codepoint, font, terminal
configuration)`. Unicode classification (the data this library is
built on) gets the codepoint half right; for the other two factors,
the answer is runtime-dependent. A few cases where the Unicode-
correct width and the rendered cell count diverge in a stable,
configurable way:

1. **Nerd Font glyphs** in the Supplementary Private Use Area
   (`U+E000-U+F8FF`, `U+F0000-U+FFFFD`). PUA codepoints are not
   classified Wide by Unicode. But terminals configured with a
   Nerd Font `symbol_map` — Kitty's `symbol_map`, Ghostty's
   bundled Nerd Font fallback, wezterm equivalents — render these
   glyphs at 2 cells. Surfaces in any bubbletea / Charm-stack TUI
   whose render path uses Nerd Font icons in alignment-sensitive
   columns.

2. **Half-width Kana** (`ﾟ`, `ﾞ` and similar). lipgloss issue
   [#666](https://github.com/charmbracelet/lipgloss/issues/666)
   reports these rendering at 2 cells in Kitty and Ghostty while
   `lipgloss.Width` (and therefore displaywidth) reports 1.

3. **Powerline glyphs** at `U+E0A0-U+E0AF`. Same shape: terminal
   renders 2, library reports 1. Surfaces in any status-line app.

The common shape: the *terminal-and-font configuration* is the
authority on width for these ranges, and that authority is stable
per session — the user's `~/.config/kitty/kitty.conf` doesn't
change between calls to `Width`. What's missing is a way for a
caller that knows its runtime configuration to declare those
overrides to the library.

### Reproduction

```go
package main

import (
	"fmt"

	"github.com/clipperhouse/displaywidth"
)

func main() {
	cases := []struct {
		name string
		s    string
	}{
		{"SPUA-A U+F01C (nf-fa-inbox)", string(rune(0xF01C))},
		{"Powerline U+E0A0", string(rune(0xE0A0))},
		{"Half-width katakana U+FF9F (ﾟ)", "ﾟ"},
		{"Wide CJK 你 (control)", "你"},
	}
	for _, c := range cases {
		fmt.Printf("%-32s width=%d\n", c.name, displaywidth.String(c.s))
	}
}
```

Output:

```
SPUA-A U+F01C (nf-fa-inbox)      width=1
Powerline U+E0A0                 width=1
Half-width katakana U+FF9F (ﾟ)    width=1
Wide CJK 你 (control)              width=2
```

In a Kitty session with the standard Nerd Font `symbol_map`
configuration (or Ghostty 1.2+ with bundled NF fallback), the
first three render at 2 cells. The CJK case is correct in both
the library and the terminal.

### Proposal

Add a declarative `Overrides` field to `Options`, mirroring the
existing pattern of declarative knobs on that struct:

```go
// Add to Options:
//   Overrides []OverrideRange // applied per grapheme; nil = current behavior

// OverrideRange forces a width for graphemes whose first rune
// falls in [Lo, Hi] (inclusive). Overrides are consulted before
// the trie lookup; the first matching range wins.
type OverrideRange struct {
    Lo, Hi rune
    Width  int
}
```

Caller usage:

```go
opts := displaywidth.Options{
    Overrides: []displaywidth.OverrideRange{
        {Lo: 0xE000,  Hi: 0xF8FF,  Width: 2}, // SPUA-A
        {Lo: 0xF0000, Hi: 0xFFFFD, Width: 2}, // SPUA-B
    },
}
opts.String("…") // returns 2 in this configuration
```

Why range-table (not a callback):

- Matches the existing `Options` idiom — current fields are all
  declarative knobs, no callbacks.
- Bounded performance overhead: small `len(opts.Overrides)` linear
  scan on the slow path; ASCII fast-path (`printableASCIILength`)
  unchanged. Nil slice = zero overhead, current behavior bit-for-bit.

### Scope questions for you

1. Does this fit the library's mission, or do you read it as
   "runtime terminal config isn't this library's job"? Either
   answer is fine — I'd rather know before drafting the PR.
2. If yes, should `Overrides` be on `Options` (per-call) or also
   on a package-level `DefaultOverrides` for callers who don't
   thread `Options` through their stack? (`go-runewidth` has
   `DefaultCondition` for the same reason.)
3. Range-table vs alternatives: `map[rune]int`, callback,
   `Method` enum extension. Range-table felt closest to your
   existing patterns — happy to be redirected.
4. Test coverage: I'd plan to add table-driven tests for SPUA
   ranges, half-width Kana, the CJK control case, and an empty/
   nil-overrides regression. Anything else you'd want covered?

### Use case / scale

I'm asking from the poplar bubbletea TUI mail client, but the
pattern is not poplar-specific:

- The lipgloss issue tracker has an open report
  ([#666](https://github.com/charmbracelet/lipgloss/issues/666))
  describing the same root cause on half-width Kana — different
  codepoint class, same library-vs-terminal divergence.
- Ghostty's discussion [#8822](https://github.com/ghostty-org/ghostty/discussions/8822)
  (Nerd Font glyph width in 1.2.0, dozens of affected users) and
  [#5588](https://github.com/ghostty-org/ghostty/discussions/5588)
  (`narrow_symbols` request) show the symptom is well-known on
  the terminal side.
- lazygit [#1933](https://github.com/jesseduffield/lazygit/issues/1933)
  is a long-running Nerd Font thread where users repeatedly
  surface render quirks without a single canonical fix.

The downstream consequence I keep running into: components like
`bubbles/help` and `bubble-table` call `lipgloss.JoinHorizontal`
in their internal render path, which undercounts every SPUA-bearing
row by 1 cell. With no width-override hook at the foundation
layer, the only way to compose those bubbles into an icon-bearing
TUI without misalignment is to hand-roll a parallel width-math
layer (which is what poplar has today, and what every similar
project I've looked at also has). A library-level hook lets that
layer collapse and lets us compose upstream components instead of
forking them.

I'm happy to send a PR if this is in scope, and happy to close
the issue if it isn't. And either way, thanks for considering it.
