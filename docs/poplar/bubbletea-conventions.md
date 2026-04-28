# Bubbletea Conventions

The structural contract between bubbletea components in poplar.
Every UI/UX change is planned and reviewed against these
conventions. Deviations are allowed but must be named and
justified — the default is to do it the way the Charm libraries
and reference apps do it.

This doc is **research-grounded**: every normative claim cites
either the source-survey research at
[`research/2026-04-26-bubbletea-norms.md`](research/2026-04-26-bubbletea-norms.md)
or the reference-app survey at
[`research/2026-04-26-reference-apps.md`](research/2026-04-26-reference-apps.md).
Those docs cite primary sources (file:line in the upstream
modules, or pinned-tag GitHub permalinks). When this doc and the
sources disagree, the sources win — file an ADR documenting the
deviation, or fix this doc.

## Purpose and scope

This document governs the **structural contract** between
bubbletea components: how they're shaped, sized, wired, and
updated. It does not govern the **visual language** (color,
icons, box-drawing vocabulary, density, aesthetic direction) —
that belongs to the `bubbletea-design` skill.

## 1. Component shape

### Update returns concrete type

Components return their concrete `Model` type from `Update`, not
`tea.Model`. This is the dominant bubbles pattern: viewport,
spinner, help, stopwatch, table, textinput, textarea all use
`(Model, tea.Cmd)` — see norms §1. The lone outlier is `progress`
(returns `tea.Model`), and that's a vestigial API the package
itself flags. **Use the concrete-type return.**

### View signature

`func (m Model) View() string`. Components that depend on
external state (like `bubbles/help`, which takes a `KeyMap`
arg — `help/help.go:104`) deviate explicitly via the signature.
For poplar's purposes, no `View` mutates state — see §6.

### Init for composability

Components that need no async startup return `Init() tea.Cmd
{ return nil }` — the bubbles pattern (`viewport/viewport.go:78`).
This is required to satisfy `tea.Model`.

### State ownership

All mutable state lives in the `Model` struct (norms §1). No
package-level mutables. Components that emit ticks (spinner,
stopwatch) carry an atomic ID and a per-instance tag, and reject
messages whose ID/tag don't match — `spinner/spinner.go:138-148`.
Poplar follows the same discipline: stale-message guards on every
component that fires its own `tea.Cmd`s.

## 2. Sizing contract

### Components own their size

A bubbletea component has three jobs:

1. Accept a size from its parent (struct fields, `SetSize(w, h)`,
   or via `tea.WindowSizeMsg` — see below).
2. Mutate state in `Update` only — never in `View`.
3. Render in `View` at **exactly** the assigned width and height.
   No more, no less.

Job 3 is the layout contract. Parents call
`lipgloss.JoinHorizontal` or `JoinVertical` and trust their
children. When a child returns lines wider than its assigned
width, the parent's joined output exceeds the terminal width, the
terminal soft-wraps, and content displaces adjacent panes —
**always the child's bug, never the parent's.**

### How parents resize children

The reference-app pattern (ref-apps §4):

1. Parent receives `tea.WindowSizeMsg` at the root.
2. Parent stores `width`/`height` on the model (or shared
   context).
3. Parent computes chrome margins (tab row, status bar, padding)
   and calls `child.SetSize(width-wm, height-hm)` on every child.
4. Parent **also forwards** `WindowSizeMsg` into each child's
   `Update`. Bubbles components (viewport, textarea, list) rely
   on the message to reinitialise scroll state — `SetSize` alone
   is insufficient. The wishlist app skips the forward and that's
   listed as an anti-pattern (ref-apps §8 avoid #6); soft-serve
   does both correctly (ref-apps §4).

For poplar's tree, the root model's `WindowSizeMsg` handler is
the canonical place for both the size store and the child fan-out.

### The viewport clipPane idiom

The canonical clipping pattern is in `viewport.View()` — norms §2:

```go
contents := lipgloss.NewStyle().
    Width(contentWidth).      // pad to width
    Height(contentHeight).    // pad to height
    MaxHeight(contentHeight). // truncate height if taller
    MaxWidth(contentWidth).   // truncate width if wider
    Render(content)
return m.Style.
    UnsetWidth().UnsetHeight(). // outer style does not re-size
    Render(contents)
```

Width+Height pad; MaxWidth+MaxHeight truncate; unsetting the outer
style's size after applying it inside avoids double-counting.
Poplar's `internal/ui` should keep its `clipPane` helper
implementing this exact pattern.

### Style.Width vs Style.MaxWidth

`Style.Width(n)` is a minimum and a wrap target (norms §4) — sets
the block width and wraps text at it. `Style.MaxWidth(n)` is a
ceiling — truncates each rendered line to ≤ n cells without
wrapping. Use Width to size a block you own; use MaxWidth as a
safety truncation on content whose final width is uncertain.

### JoinHorizontal padding

`lipgloss.JoinHorizontal` right-pads every line in every block to
that block's widest line — `lipgloss/join.go:88`, norms §4.
Callers must **not** add their own padding on top: a block
containing `"abc\nde\nf"` outputs lines of width 3, 3, 3, not
3, 2, 1. If you observe what looks like missing padding from a
child, the child's `View` is producing inconsistent line widths —
fix the child.

## 3. Text rendering

### The wordwrap + hardwrap pair

A renderer that takes a `width int` parameter must produce output
where no line exceeds that width. **Wordwrap alone is
insufficient** because a single long token (URL, code identifier,
pasted blob) won't be broken — `ansi.Wordwrap` only breaks at
word boundaries (norms §5). The standard pattern is
`Hardwrap(Wordwrap(s, w, ""), w, false)`: wordwrap for natural
breaks, then hardwrap to catch overflow.

`ansi.Truncate(s, w, tail)` returns single-line content cut to ≤
w cells, appending tail (typically `…`) — norms §5.

`ansi.Strip(s)` removes ANSI escapes; `ansi.StringWidth(s)` and
`lipgloss.Width(s)` return cell width. **Never `len(s)` for
layout math** — counts bytes, not display cells.

### Glamour caveat

Glamour has no runtime width-change API (norms §5). When the
terminal resizes, a new `TermRenderer` must be constructed.
Poplar does not currently use glamour (themes are compiled
lipgloss values per ADR-0046), so this is informational.

## 4. Key bindings and help

### key.Binding declarations

Bindings are declared as `key.Binding` values in a `KeyMap`
struct, not as raw strings — norms §3:

```go
type KeyMap struct {
    Up   key.Binding
    Down key.Binding
}

var DefaultKeyMap = KeyMap{
    Up:   key.NewBinding(key.WithKeys("k", "up"),
                         key.WithHelp("k", "up")),
    Down: key.NewBinding(key.WithKeys("j", "down"),
                         key.WithHelp("j", "down")),
}
```

`key.Matches(msg, m.KeyMap.Up)` is the canonical dispatch (norms
§3). All production reference apps use it (ref-apps §3); only
glow falls back to `msg.String()` switches and that's listed as
an anti-pattern (ref-apps §8 avoid #4). Disabled bindings never
match, which makes per-state activation declarative.

### Poplar's keybinding deviations

Poplar binds modifier-free, single-key, no-command-mode keys
(ADR-0015, 0024, 0051, 0068, 0076). The community norm is `key.Binding`
declarations regardless of binding shape — the modifier-free
choice is upstream; the *declaration form* is shared. Poplar
should keep using `key.Binding` for declaration even when help
text is just `"k"` rather than `"↑/k"`.

### Help integration

`bubbles/help` is the universal help renderer (ref-apps §3) —
every surveyed app except gum uses it. Components implement
`ShortHelp() []key.Binding` and `FullHelp() [][]key.Binding`
(`help/help.go:18-27`). The poplar HelpPopover deviates:
ADR-0071 chose a custom modal popover with future-binding hints
rather than `bubbles/help`'s footer/expanded shape. **The
deviation is documented; new help surfaces should not extend it
without an ADR.**

`help.SetWidth(w)` must be called from the `WindowSizeMsg`
handler so it can truncate gracefully (ref-apps §3); the poplar
HelpPopover's centering is `lipgloss.Place` and does not need
this.

## 5. Async I/O and update flow

### tea.Cmd is the only async boundary

All blocking I/O lives inside `tea.Cmd` (a `func() tea.Msg`).
**No goroutines outside Cmds.** Reference apps unanimous on this
(ref-apps §5).

### Channel + blocking-Cmd for server push

For real-time push (server events, IMAP IDLE, JMAP /events): one
goroutine writes events to a channel; one Cmd blocks on receive
and returns one msg; on receipt, `Update` re-fires the blocking
Cmd to wait for the next event. The bubbletea `realtime` example
is canonical (ref-apps §5).

### Cmd is not for intra-model messaging

`tea.go:62-64`: "there's almost never a reason to use a command
to send a message to another part of your program. That can
almost always be done in the update function." Routing messages
between sub-models via `tea.Cmd` adds asynchronous indirection
where direct delegation would do.

### tea.Batch on a slice

Build `[]tea.Cmd` then pass as variadic: `tea.Batch(cmds...)`.
Every reference app does it this way (ref-apps §7 emulate #4).
Nil cmds in the slice are silently dropped (norms §6).

### Mutation discipline

State mutates only in `Update`. Never in `View`. Never inside a
`tea.Cmd` closure (Cmds run async — closing over the model risks
data races and stale state). When a Cmd needs values from the
model, capture them by value at construction time.

### Error messages

Poplar's canonical error message is
`ErrorMsg{Op string; Err error}` (per ADR-0073). Every poplar
`tea.Cmd` that can fail returns an `ErrorMsg`. App-level handling
stores last-error and renders the banner; components do not own
error UI.

## 6. Program setup

### tea.NewProgram options

Poplar uses (cmd/poplar/root.go):

- `WithAltScreen()` — fullscreen altbuffer at startup. **Never**
  use `tea.EnterAltScreen()` in `Init()`: norms §7 — "Because
  commands run asynchronously, this command should not be used in
  your model's Init function." The `WithAltScreen()` constructor
  option is the right place.
- `WithMouseCellMotion()` if mouse interaction is wanted (poplar
  is keyboard-only in v1).

`WithFPS` defaults to 60 (max 120) — norms §6. Poplar leaves the
default.

### Spinner pattern

Spinners need their owner to:
1. Construct via `spinner.New(spinner.WithSpinner(spinner.Dot))`.
2. Return `m.spinner.Tick` from `Init` or from the Update branch
   that starts the spinner (not `EnterAltScreen`-equivalent
   restrictions; spinner ticks are allowed in Init).
3. Forward `spinner.TickMsg` into `m.spinner.Update` in the
   parent's Update; the spinner's ID/tag mechanism (norms §1)
   ensures only its own ticks are processed.

Poplar's shared `NewSpinner(t *theme.CompiledTheme)` helper in
`internal/ui/styles.go` is the canonical constructor — see ADR-0074.

## 7. Layout patterns from reference apps

### Component interface with SetSize

Both soft-serve and gh-dash define a `Component` interface that
embeds `tea.Model` and adds `SetSize(w, h int)` (ref-apps §1):

```go
type Component interface {
    tea.Model
    help.KeyMap
    SetSize(w, h int)
}
```

Lets the root resize all children uniformly without
type-switching. Poplar should adopt this shape when the model
tree gets deep enough that the root model's `SetSize`
implementation needs to type-switch.

### Margin subtraction before SetSize

Multi-pane apps compute chrome height (tab row + statusbar +
padding) and pass `height - hm` to children — soft-serve, glow
(ref-apps §2). The child's `View()` then fills exactly the space
it was given. Poplar's account-tab layout follows this.

### Root short-circuits on focused input

When a text input (search shelf, compose body) is focused, root
`Update` checks first and routes the message there, returning
immediately (ref-apps §7 emulate #3). Poplar's account view does
this for the search shelf already (per ADR for search).

### Theme as semantic tokens

Production apps separate palette from semantics (ref-apps §6).
The `Theme` / `Styles` struct fields are role names
(`PrimaryBorder`, `SelectedBackground`), populated from a small
palette of named colors. Hex literals only appear in the palette
definition. Poplar's `internal/theme/` follows this; raw hex
appears only in `themes.go`.

## 8. Anti-patterns

Each item below is a known failure mode. The cited source
contains the receipt.

- **Defensive parent-side clipping.** A parent calling
  `lipgloss.NewStyle().MaxWidth(w).Render(child.View())` papers
  over a child contract violation. Fix the child instead. (Source:
  JoinHorizontal trust contract, norms §4.)
- **Wordwrap without hardwrap.** Long URLs and unbreakable tokens
  overflow. Always pair them. (norms §5.)
- **`len(s)` for layout math.** Counts bytes, not cells. Use
  `lipgloss.Width(s)`. (norms §5.)
- **Rune-counting Nerd Font icons.** SPUA-A glyphs (U+F0000+)
  often render double-width but `runewidth` reports 1. Use
  `displayCells(s, spuaCellWidth)` with the startup-resolved width;
  maintain an explicit icon table in `internal/ui/icons.go`. (ADR-0084.)
- **`lipgloss.JoinHorizontal`/`JoinVertical` on SPUA-A rows.**
  These pad using `lipgloss.Width`, which undercounts fancy-mode
  icons. Use row-by-row `strings.Join` with pre-padded children.
  This restriction applies when `spuaCellWidth != 1` (i.e., fancy
  mode on Mono Nerd Font terminals). In simple mode and on
  narrow-Nerd-Font terminals the ban is technically inert, but the
  existing manual row-by-row join code is kept under both regimes
  — see ADR-0084.
- **Mutating model state in `View()` or in a `tea.Cmd` closure.**
  Update is the only mutation point. (norms §1.)
- **Children calling parent methods or holding callbacks.**
  Children signal up via `tea.Msg` types. (Elm architecture; ADR-0023.)
- **`tea.ExecProcess` for in-app surfaces.** Compose, picker,
  inline editors render in-pane. ExecProcess is for genuine
  shell-out (e.g., embedded `nvim`). (ADR-0033.)
- **`EnterAltScreen()` in `Init()`.** Use `tea.WithAltScreen()`
  on `NewProgram` instead. Same warning applies to
  `EnableMouseCellMotion`/`EnableMouseAllMotion`. (norms §7.)
- **`tea.Cmd` for intra-model messaging.** Use direct `Update`
  delegation. (`tea.go:62-64`.)
- **Re-firing `Init()` from inside `Update()`.** Soft-serve does
  this on `RepoMsg` and produces duplicate spinner ticks (ref-apps
  §8 avoid #5). Use a dedicated reset method that returns only
  the Cmds the new state needs.
- **Not forwarding `WindowSizeMsg` to children.** `SetSize` alone
  is insufficient — bubbles components rely on the msg to reset
  internal state. Always do both. (ref-apps §8 avoid #6.)
- **Package-level `.Render` closures** (`var foo = NewStyle()....Render`).
  Bakes styles at init time; can't respond to runtime theme
  changes. Use a `Styles` struct constructed when theme is known.
  (glow's `ui/styles.go:36-64`; ref-apps §8 avoid #1.)
- **String switch on `msg.String()` for actionable keys.** Hides
  bindings from `bubbles/help` and prevents rebinding. Use
  `key.Matches`. (ref-apps §8 avoid #4.)
- **Using deprecated APIs.** `viewport.HighPerformanceRendering`,
  `tea.Sequentially`, `spinner.Tick()` (package-level no-arg form),
  `*Model.NewModel` constructors. (norms §7.)

## 9. Planning checklist

Before writing any UI code, the plan or spec answers:

- [ ] Which bubbles component is the closest analogue? (viewport,
      list, table, textinput, textarea, spinner, help.) Cite the
      file in the bubbles module.
- [ ] What is each component's owned state? What does it receive
      from its parent (struct fields, `SetSize`, or
      `WindowSizeMsg`)?
- [ ] How does each component's `View()` enforce its width/height
      contract (the `clipPane` pattern, lipgloss
      Width+Height+MaxWidth+MaxHeight, or equivalent)?
- [ ] Which messages flow up from children, and which are handled
      locally? Is the `tea.Msg` type explicit (not a callback)?
- [ ] How is `tea.WindowSizeMsg` propagated — both `SetSize` and
      forwarding the msg into children?
- [ ] If async I/O: which Cmd wraps it? Does it return `ErrorMsg`
      on failure?
- [ ] If keys: declared as `key.Binding`, dispatched with
      `key.Matches`?

Deviations from a bubbles analogue are explicit. "We need a
custom list because X" is fine; "we just wrote a custom thing"
is not.

## 10. Review checklist

Run this against every UI diff at pass-end. Each item is
verifiable from the diff or from a tmux capture.

- [ ] Every changed/new component's `View()` returns no lines
      wider than its assigned width and no more rows than its
      assigned height. **Verified with a live tmux capture at
      120×40** (and at the minimum viable width if the pass
      touches layout).
- [ ] No state mutation in `View()` or in any `tea.Cmd` closure.
- [ ] All blocking I/O lives inside `tea.Cmd`.
- [ ] Width math uses `lipgloss.Width` / `ansi.StringWidth`,
      never `len()`.
- [ ] Renderers that take a `width` parameter honor it via
      wordwrap + hardwrap (or equivalent) — wordwrap-only is
      insufficient.
- [ ] No defensive parent-side clipping — a `MaxWidth` on
      `child.View()` output is a sign the child isn't honoring
      its contract; fix the child instead.
- [ ] Children signal parents via `tea.Msg` types, not callbacks
      or parent pointers.
- [ ] `WindowSizeMsg` is forwarded into children after the parent
      stores dims and calls `SetSize`.
- [ ] Keys declared as `key.Binding`; dispatched with
      `key.Matches`. New keys, if any, included in the help
      vocabulary per ADR-0072.
- [ ] No deprecated API usage (`HighPerformanceRendering`,
      `tea.Sequentially`, package-level `spinner.Tick`,
      `*Model.NewModel`).

Any deviation introduced this pass is named in an ADR with
explicit rationale. Silent deviation is not acceptable.

## See also

- [`research/2026-04-26-bubbletea-norms.md`](research/2026-04-26-bubbletea-norms.md)
  — source-survey of bubbles, glamour, lipgloss, bubbletea,
  x/ansi. Authority of last resort: if this doc and the source
  disagree, the research doc wins.
- [`research/2026-04-26-reference-apps.md`](research/2026-04-26-reference-apps.md)
  — production-app survey across glow, gum, soft-serve, wishlist,
  gh-dash, official examples.
- `internal/ui/viewer.go` — `clipPane` is the canonical poplar
  size-contract enforcer.
- `elm-conventions` skill — the broader Elm architecture rules
  this doc lives inside.
- `bubbletea-design` skill — the visual language (color, icons,
  box-drawing, density) that complements this structural contract.
- `.claude/docs/tmux-testing.md` — the live render workflow.
- `.claude/hooks/bubbletea-conventions-lint.sh` — the
  PostToolUse linter that flags structural violations as
  warnings (added Pass 4).
