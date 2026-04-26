# Pass 2.5b-5 — Help popover design

**Date:** 2026-04-25
**Pass:** 2.5b-5 (per-screen prototype)
**Status:** Spec — pending implementation plan

## Goal

Ship the help popover — a modal overlay triggered by `?` from the
account view and the viewer, advertising the keybindings users need
to remember. First modal in the codebase.

## Open question (resolved)

**Future-binding policy.** Three options were on the table (audit
2026-04-25, plan-shape findings §"Pass 2.5b-5"):

- A. Show only currently-wired keys.
- B. Show all eventual keys (silent dead keys).
- C. Show all keys with a "future" marker.

**Decision: C, with C1 styling.** The popover advertises the full
planned vocabulary. Wired rows use the standard bright-bold key +
dim description. Unwired rows render the entire row in `FgDim`,
no glyph, no legend. Group headings stay bright regardless. The
dim-key→bright-key delta is the sole signal — it matches the
existing footer precedent (`keybindings.md` §"Future hints shown")
that already advertises future bindings as aspirational vocabulary.

The `wired bool` flag lives per-row in the binding tables. Pass-by-
pass, the wiring pass flips a flag from `false` to `true` and the
dim treatment lifts automatically.

## Architecture

**Owner: `App`.** The `?` key already routes to `App.Update`
(app.go:102, currently a stub). `App` adds two fields:

- `helpOpen bool`
- `help HelpPopover`

`App` already tracks `viewerOpen bool`. That flag is the context
selector — when `?` is pressed, the popover is constructed for
`HelpViewer` if the viewer is open, else `HelpAccount`.

**New file: `internal/ui/help_popover.go`.** Exports `HelpPopover`,
a tea.Model fragment with `View(width, height int) string` and no
`Init`/`Update` of its own. `App` owns key routing.

```go
type HelpContext int

const (
    HelpAccount HelpContext = iota
    HelpViewer
)

type HelpPopover struct {
    styles  Styles
    context HelpContext
}

type bindingRow struct {
    key   string
    desc  string
    wired bool
}

type bindingGroup struct {
    title string
    rows  []bindingRow
}
```

**Binding tables are static package-level data**, not derived from
a registry. Each row carries `key`, `desc`, `wired bool`. Two
top-level layouts — `accountGroups []bindingGroup` and
`viewerGroups []bindingGroup` — defined as `var` blocks at the
top of `help_popover.go`.

No new dependency on `keys.go` or `footer.go`. The popover
describes what we *advertise*; those describe what we *bind*.
They diverge during the future-binding period and converge by
v1.

## Binding tables

### Account context (7 groups, 4 layout sections)

| Group | Rows |
|---|---|
| Navigate | `j/k up/down` ✓ · `g/G top/bot` ✓ |
| Triage | `d delete` ✗ · `a archive` ✗ · `s star` ✗ · `. read/unrd` ✗ |
| Reply | `r reply` ✗ · `R all` ✗ · `f forward` ✗ · `c compose` ✗ |
| Search | `/ search` ✓ · `n next` ✗ · `N prev` ✗ |
| Select | `v select` ✗ · `␣ toggle` ✗ |
| Threads | `␣ fold` ✓ · `F fold all` ✓ |
| Go To | `I inbox` ✓ · `D drafts` ✓ · `S sent` ✓ · `A archive` ✓ · `X spam` ✓ · `T trash` ✓ |

Bottom hint line: `Enter open` ✓ · `? close` ✓.

Layout section order (top to bottom in the JoinVertical stack):

1. Navigate · Triage · Reply (single layout row, three side-by-side
   columns).
2. Search · Select · Threads (single layout row, three columns).
3. Go To — rendered as an internal 3×2 grid of folder bindings
   (`I/D/S` on the first sub-row, `A/X/T` on the second).
4. Bottom hint line.

### Viewer context (3 groups, 1 layout row + hint)

| Group | Rows |
|---|---|
| Navigate | `j/k scroll` ✓ · `g/G top/bot` ✓ · `␣/b page d/u` ✓ · `1-9 open link` ✓ |
| Triage | `d delete` ✗ · `a archive` ✗ · `s star` ✗ |
| Reply | `r reply` ✗ · `R all` ✗ · `f forward` ✗ · `c compose` ✗ |

Bottom hint line: `Tab link picker` ✗ · `q close` ✓ · `? close` ✓.

### Departures from wireframe

- **`Space` collision (Threads/Select).** The wireframe shows `␣`
  in both Threads (`fold`) and Select (`toggle`). Both are
  accurate — `Space` is dual-meaning per ADR-0052 (fold outside
  visual mode, toggle inside). The popover lists each row in its
  group with its own description.
- **Viewer triage column.** Wireframe omits `.  read` from the
  viewer context. The popover follows the wireframe — viewer
  triage stays at `d/a/s` for this pass. The key is reserved by
  the global keymap and will be added in Pass 6 if the design
  warrants it.

## Render strategy

**Popover box.** Rounded border in `BgBorder`, padding (1, 2),
title (`Message List` or `Message Viewer`) in `AccentPrimary`
bold, embedded in the top border. Title splicing mirrors the
existing `top_line.go` approach.

**Group rendering.** Each `bindingGroup` is a fixed-width column.
Header is `FgBright` bold. Rows are two-column flex:

- **Key column** (right-aligned within the column): `FgBright`
  bold for wired rows; `FgDim` (no bold) for unwired rows.
- **Description column** (left-aligned): `FgDim` for both wired
  and unwired (the bold drop on the key column is the C1 signal).

Net visual: wired rows have a bright bold key with a dim
description; unwired rows are uniformly dim with no bold.

**Overall layout.** Groups are arranged with `lipgloss.JoinHorizontal`
per layout row, then `lipgloss.JoinVertical` stacks the rows,
wrapped by the bordered box.

**Centering.** `lipgloss.Place(width, height, lipgloss.Center,
lipgloss.Center, popoverBox)` produces the centered output at the
App's full dimensions. Whitespace fills with spaces (the default).

**No background dimming in v1.** The wireframe annotation calls
for dimmed content behind the popover. Lipgloss has no native
opacity, and ANSI-level color stripping of the underlying view
is fragile. The popover's bordered box, accent-colored title,
and centered placement give enough visual distinction. Logged as
a BACKLOG item to revisit if user testing flags it.

**Sizing.** The popover box has a fixed natural width derived from
content (~62 cols for account, ~58 for viewer). For terminals
smaller than the popover's natural size, `lipgloss.Place` clips
gracefully. Responsive popover layout is out of scope for v1.

## Key routing

**App.Update gains an early modal branch.** When `helpOpen` is
true, every `tea.KeyMsg` is intercepted before any existing
routing:

```go
case tea.KeyMsg:
    if m.helpOpen {
        switch msg.String() {
        case "?", "esc":
            m.helpOpen = false
        }
        return m, nil // swallow everything else
    }
    // existing q / ctrl+c / ? routing continues
```

The existing `?` stub at app.go:102 becomes:

```go
case "?":
    m.helpOpen = true
    ctx := HelpAccount
    if m.viewerOpen {
        ctx = HelpViewer
    }
    m.help = NewHelpPopover(m.styles, ctx)
    return m, nil
```

**Why steal everything.** Wireframe annotation: "All keypresses
route to popover when open. Only `?` and `Escape` are handled;
everything else is ignored." Stricter than the search shelf's
modal stealing (search accepts printable runes); help has no
input surface, so the simplest correct behavior is to swallow
all non-dismiss keys.

**Ordering with `q`.** `q` outside the popover quits the app (or
closes the viewer, or clears search). Inside the popover, `q` is
swallowed. Deliberate divergence from the search-shelf rule
(`q` clears search) — help is a view, not a state to escape.

**Footer behavior while open.** Footer underneath the popover
does not change. The popover's own bottom hint line shows
`? close` clearly. Conditional footer branching isn't worth the
churn.

**Window resize while open.** `tea.WindowSizeMsg` flows through
the existing handler. The popover re-renders centered on the new
dimensions.

## Testing

### Unit tests in `help_popover_test.go`

- `TestHelpPopoverAccountContent` — render the account popover at
  fixed dimensions, assert the rendered string contains every
  expected key/desc pair (`d delete`, `r reply`, `I inbox`, etc.)
  and the title `Message List`.
- `TestHelpPopoverViewerContent` — same for viewer context;
  assert title `Message Viewer` and absence of account-only
  groups (Search, Select, Threads, Go To).
- `TestHelpPopoverWiredStyling` — table-driven over a few
  representative rows. Assert wired keys render with the
  bright-bold style applied to the key column; unwired keys
  render dim with no bold. Checks raw ANSI sequences,
  consistent with `styles_test.go`.
- `TestHelpPopoverGroupHeaders` — assert group headings render
  bright regardless of how many of their rows are unwired
  (e.g., the Reply group is fully unwired today).

### Integration tests in `app_test.go`

- `TestAppHelpOpenClose` — start app, send `?`, assert
  `helpOpen == true` and rendered View contains the popover
  title. Send `?`, assert `helpOpen == false`.
- `TestAppHelpDismissEsc` — same, dismiss with `Esc`.
- `TestAppHelpStealsKeys` — open the popover; send `j`, `J`,
  `d`, `r`, `/`. Assert the underlying `acct.msglist.Cursor()`
  and `acct.sidebar.Cursor()` are unchanged, and
  `acct.sidebarSearch.State() == SearchIdle`. Confirms full
  key stealing.
- `TestAppHelpContextSwitch` — open viewer (`Enter` on a
  message), send `?`, assert title is `Message Viewer`. Close
  help, close viewer (`q`), reopen help, assert title is now
  `Message List`.
- `TestAppHelpQuitSwallowed` — open the popover, send `q`,
  assert app is still alive (no `tea.Quit` cmd returned) and
  `helpOpen == true`.

### Live verification

Per `.claude/docs/tmux-testing.md`: capture the popover open
over both account and viewer, confirm the dim treatment reads
at a glance.

### Out of scope for tests this pass

- Background-dim rendering (deferred to a future polish pass).
- Responsive popover layout for narrow terminals.

## Pass-end consequences

**New ADRs (anticipated):**

- Help popover modal infrastructure (App ownership, key stealing,
  no background dim in v1).
- Future-binding policy (C1 — full vocabulary, dim row for unwired,
  no glyph).
- Possibly an ADR superseding ADR-0022 per the audit's
  recommendation; that is independent of this pass and may land
  separately as a "consolidation" commit.

**Invariants update:** add facts for help popover ownership, key
routing, future-binding policy. Remove or rewrite any
keybinding-related fact that this pass refines.

**STATUS update:** mark Pass 2.5b-5 done; replace next starter
prompt with Pass 2.5b-6 (error banner + spinner consolidation,
per audit recommendation).

**BACKLOG additions:**

- Background dim for popover overlay (revisit if testing flags
  the no-dim approach).
- Responsive popover layout for narrow terminals.
