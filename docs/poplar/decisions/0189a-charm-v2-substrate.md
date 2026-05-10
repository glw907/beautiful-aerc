---
title: charm.land/v2 substrate (mechanical migration + AdaptiveColor removal)
status: accepted
date: 2026-05-10
---

## Context

Pass 14 needs `charm.land/huh/v2` for its first-run wizard. huh/v2
links only against the `charm.land/{bubbletea,lipgloss,bubbles}/v2`
stack, and a single tea.Program cannot mix v1 and v2 inside one
process — the `tea.Msg` and `tea.Cmd` types are incompatible
across the major-version boundary. Migration is mandatory before
Pass 14.

The original Pass 13.2 plan bundled the mechanical migration
(imports, mechanical drift, AdaptiveColor removal) with three
architectural reframes that v2's `tea.View` model unlocks
(declarative chrome, hoisted cursor, paste handling) into one
13-task pass. Mid-execution two signals fired the split-inline
rule from CLAUDE.md "Migrations and breaking changes":

- **Scope discovery.** Task 1's codemod missed v2's KeyPressMsg
  field-layout drift. v1's `tea.KeyMsg` had `Type tea.KeyType`,
  `Runes []rune`, `Alt bool`, `Paste bool`. v2's `tea.KeyPressMsg`
  embeds `tea.Key` with `Code rune`, `Text string`, `Mod KeyMod`,
  `ShiftedCode/BaseCode rune`, `IsRepeat bool`. 200+ call sites
  needed hand migration that the import-rewrite codemod couldn't
  reach. Task 1.5 was added to absorb it.
- **Cross-task scaffolding.** Tasks 3+4+5 (cursored subpackages
  return tea.View, App pulls cursor up, declarative chrome) had
  to be bundled into a single subagent dispatch because
  splitting them would have left intermediate transitional
  state — cursored child returning tea.View that the App
  doesn't yet read; per-input `cursor.Model` removed before the
  App-level ticker is wired. Per the no-scars rule that landed
  in CLAUDE.md mid-pass, that's not allowed.

Both signals are exactly what the split-inline rule exists to
detect. Splitting now means Pass 13.2a (this ADR) lands the
substrate cleanly with `make check` green, and Pass 13.2b lands
the architectural reframes with a green starting point and full
attention.

## Decision

**Land charm.land/v2 as a substrate-only pass.** Tree compiles on
v2; every test passes; every theme renders. The transitional
`App.View()` returns `tea.NewView(s)` wrapping the existing v1-
shaped composition string — no declarative chrome fields, no
hoisted cursor, no `tea.PasteMsg` arms. This is the **published
end state** of 13.2a, not scaffolding.

The reframes (chrome, cursor, paste) defer to Pass 13.2b. The
seam between 13.2a and 13.2b is the transitional `App.View()`:
13.2a publishes it; 13.2b's first commit replaces it with the
v2-native declarative composition.

### What landed in 13.2a

**Substrate imports** — every `.go` file under `cmd/` and
`internal/`:

- `github.com/charmbracelet/bubbletea` → `charm.land/bubbletea/v2`
- `github.com/charmbracelet/lipgloss` → `charm.land/lipgloss/v2`
- `github.com/charmbracelet/bubbles` → `charm.land/bubbles/v2`

`charmbracelet/x/ansi` stays at v1 — only the three top-level
libs moved domains. `internal/ansix/` continues to wrap it; the
"is `lipgloss.Width` enough now" audit defers to Pass 13.3 or 15.

**Mechanical drift**:

- `tea.KeyMsg` → `tea.KeyPressMsg`
- `tea.Sequentially` → `tea.Sequence`
- `spinner.NewModel` → `spinner.New`
- `viewport.New(w, h)` → `viewport.New(viewport.WithWidth(w),
  viewport.WithHeight(h))`
- bubbles components: exported `Width`/`Height` fields → `SetWidth`
  / `SetHeight` methods
- `textarea.SetCursor(col)` → `SetCursorColumn(col)` (rename)
- `textinput.Styles` field accesses → nested `Styles.Focused` /
  `Styles.Blurred`
- `viewport.Model.YOffset` field → `YOffset()` method

**KeyPressMsg field-layout drift** (the surprise) — 200+ call
sites:

| v1 | v2 |
|---|---|
| `k.Type == tea.KeyEnter` | `k.Code == tea.KeyEnter` |
| `k.Type == tea.KeyRunes` | `len(k.Text) > 0` |
| `k.Runes[0]`, `string(k.Runes)` | `k.Code`, `k.Text` |
| `k.Alt` | `k.Mod & tea.ModAlt != 0` |
| `k.Paste` | gone — paste is `tea.PasteMsg` (13.2b) |
| `tea.KeyCtrlO` etc. | `k.Mod&tea.ModCtrl != 0 && k.Code == 'o'` |
| `tea.KeyShiftTab` | `k.Mod&tea.ModShift != 0 && k.Code == tea.KeyTab` |
| Test literal `KeyPressMsg{Type: tea.KeyRunes, Runes: ...}` | `KeyPressMsg{Code: ..., Text: ...}` |

The v1 `Paste` flag has no v2 equivalent on KeyPressMsg —
bracketed paste arrives as a separate `tea.PasteMsg`.
`internal/catkin/`'s former `handlePaste` (v1 `Paste`-flag check,
URL-paste wrapping) was deleted outright in Task 1.5; 13.2b
writes a v2-native `tea.PasteMsg` handler from scratch and brings
the URL-paste wrapping back.

**`lipgloss.AdaptiveColor` removal**. v2 dropped `AdaptiveColor`
from the public API; a `compat.AdaptiveColor` shim exists. Poplar
declines the shim. `internal/theme/palette.go` field types
changed from v1's `lipgloss.Color` (a string-typed alias) to
stdlib `color.Color`. v2's `lipgloss.Color(s)` is a function
returning `color.Color`, not a struct — value-comparison sites
were rewritten.

**Type-3 substrate regressions discovered while making
`make check` green**:

- `key.NewBinding(key.WithKeys(" "))` no longer matches the
  Space key in v2. v2's `KeyPressMsg{Code: ' '}.String()` returns
  `"space"`, not `" "`. Three production sites fixed:
  `compose.AttachPicker.Toggle`, `reader.Model.PageDown`,
  `account.keys.ToggleFold`.
- `truncate(s, n)` in compose's local helper was rune-by-rune
  with `lipgloss.Width(string(r))`, which counts ANSI bracket /
  digit / `m` runes as width 1 (incorrect). v2 textinput emits
  ANSI codes in positions where v1 did not, surfacing the latent
  bug. Replaced with `ansix.Truncate`.
- Several test goldens regenerated for v2's TrueColor default
  (substrate shift, not visual regression).

### What does NOT land in 13.2a

These are 13.2b's scope — explicitly named so the substrate
boundary is clear:

- **Declarative chrome.** `cmd/poplar/root.go` still passes
  `tea.WithAltScreen()` to `tea.NewProgram`. `App.View()` does
  not yet set `view.AltScreen` / `view.MouseMode` /
  `view.ReportFocus` / `view.WindowTitle`.
- **Hoisted cursor.** Per-input `cursor.Model` instances still
  tick. `VirtualCursor` defaults stand. `App.View()` does not
  read focused-child cursors. `tea.View.Cursor` stays nil.
- **Cursored subpackages return `tea.View`.** `compose.Model`,
  `contacts.Form`, `messagelist`'s search-mode input still
  return `View() string`.
- **Compose `tea.PasteMsg` arms.** Address-list atomic emission
  via `content.ParseAddressList`, body-arm Catkin Undo bundling,
  Catkin URL-paste wrapping — all deferred. Compose currently
  treats paste as a stream of keystrokes (v1 behavior).

### Named further deferrals

- `internal/ansix/` audit (does v2 `lipgloss.Width` cover SPUA?)
  → Pass 13.3 or Pass 15. Needs SPUA-fixture measurement.
- Per-subpackage `Styles` restructuring (mirror v2 textinput's
  nested `Focused`/`Blurred` shape) → Pass 15.
- Color profile + isDark threading via `term.Resolve` (extend
  startup capability resolution to also return
  `(colorprofile.Profile, isDark bool)`) → Pass 14.1 or 15.
  Additive; works fine via lipgloss defaults until then.

## Consequences

The transitional `App.View()` is unusual but principled. ADR-0105
(pre-beta posture) explicitly endorses breaking changes with no
compat shims, so Pass 13.2b's first commit reshapes `App.View()`
without any legacy adapter. The substrate-vs-architecture seam is
the cleanest split available — splitting earlier would have meant
mixing v1 and v2 in one tea.Program (forbidden by upstream),
splitting later would have meant landing 200+ KeyPressMsg field
sites in the same commit as architectural reframes (the original
budget overrun).

`docs/poplar/bubbletea-conventions.md` was authored aspirationally
in Pass 13.1.5, describing the v2-native end state. After 13.2a,
the "Declarative chrome (v2)" and "Cursor hoist (v2)" sections
describe 13.2b's target, not current reality — a note at the top
of the doc names this. ADR-0189b refreshes the doc against the
post-13.2b reality.

The CLAUDE.md "Migrations and breaking changes" bullet was
strengthened mid-pass with the "no scars" rule (no v1 idioms in
v2 syntax, no stubs awaiting a sibling task, no cross-task TODO
markers, no commented-out legacy preserved as "reference"). That
rule is now binding for every future major migration.

## Pass shape

13.2a was 9 tasks (8 implementation + 1 doc). 13.2b is also 9
tasks. Both fit CLAUDE.md's 8–12 budget. The split-inline call
mid-execution illustrates the rule's value: catching the overrun
before the ADR was written rather than after.

## Alternatives considered

- **Keep the original 13-task pass.** Rejected mid-execution
  once the KeyPressMsg surprise plus the 3+4+5 bundling forced
  scope to 14+ tasks across multiple architectural concerns.
- **Per-subpackage adapter shims** to flip v1→v2 incrementally.
  Forbidden by ADR-0105 (no compat shims) and impossible in
  practice — `tea.Msg`/`tea.Cmd` types thread through every
  Update method, so no subpackage compiles in isolation.
- **`compat.AdaptiveColor` shim**. Rejected. Each poplar theme
  resolves to one mode at compile time; the AdaptiveColor
  abstraction was dead weight even in v1.

## References

- `docs/superpowers/specs/2026-05-10-pass-13-2a-charm-v2-substrate-design.md`
- `docs/superpowers/plans/2026-05-10-pass-13-2a-charm-v2-substrate.md`
- ADR-0105 (pre-beta posture)
- ADR-0189b (charm.land/v2 reframes — pending)
