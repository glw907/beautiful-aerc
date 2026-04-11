# Poplar Chrome Shell Design (Pass 2.5b-1)

Design spec for the chrome shell prototype — the persistent frame
that every poplar screen renders inside.

## Goal

Themed shell renders, Tab cycles focus, footer shows hints. This
is the scaffold that all subsequent passes (sidebar, message list,
viewer, overlays) drop into.

## Deliverables

1. Root bubbletea program (`cmd/poplar/root.go` rewritten)
2. Root model (`internal/ui/app.go`)
3. Tab bar with 3-row bubble rendering
4. Status bar (normal mode only — toast/undo/error in 2.5b-6)
5. Command footer via `bubbles/help`
6. Focus cycling between sidebar and message list placeholders
7. Theme-to-lipgloss bridge (`internal/ui/styles.go`)
8. Mock backend (`internal/mail/mock.go`)

## Dependencies to Add

```
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
```

`lipgloss` is already in `go.mod`.

## Root Model (`internal/ui/app.go`)

The root model owns:

- `tabs []Tab` — starts with one `AccountTab`
- `activeTab int` — index into tabs
- `styles Styles` — derived from compiled theme
- `statusBar StatusBar` — normal status for now
- `width, height int` — terminal dimensions from `tea.WindowSizeMsg`

### Init

Returns no commands. Mock data, no I/O at startup.

### Update

1. Handle `tea.WindowSizeMsg` — store dimensions, propagate to
   active tab
2. Handle global keys:
   - `1-9` — switch tab by position
   - `gt` — next tab (cycles, not shown in footer)
   - `gT` — previous tab (cycles, not shown in footer)
   - `?` — toggle help popover (2.5b-5, stubbed as no-op)
   - `:` — command mode (2.5b-7, stubbed as no-op)
3. Delegate remaining keys to `tabs[activeTab].Update(msg)`
4. Collect all child cmds into `tea.Batch`

### View

```
lipgloss.JoinVertical(lipgloss.Left,
    renderTabBar(),
    tabs[activeTab].View(),
    statusBar.View(),
    footer.View(),
)
```

Content area height = `height - 3 (tab bar) - 1 (status) - 1 (footer)`.

## Tab Interface

```go
type Tab interface {
    tea.Model
    Title() string
    Icon() string
    Closeable() bool
}
```

### AccountTab

- `Title()` = current folder name (e.g., "Inbox")
- `Icon()` = folder Nerd Font icon (e.g., "󰇰")
- `Closeable()` = false
- Owns `focusedPanel` enum (`SidebarPanel` / `MsgListPanel`)
- `Tab` key toggles `focusedPanel`
- Renders two-panel split: sidebar (30 cols) + message list (remaining)
- Both panels are placeholders in 2.5b-1 (centered `fg_dim` text)
- Focused panel gets subtle `bg_selection` background

### ViewerTab (not implemented in 2.5b-1)

Will be added in Pass 2.5b-4.

## Tab Bar Rendering

3-row custom lipgloss layout. The active tab is a rounded bubble
that opens into the content area below.

### Row structure

```
Row 1:  ╭───────────╮
Row 2:  │ 󰇰  Inbox  │  Re: Project update for Q2 launch
Row 3: ─╯           ╰──────────────────────────────────────────╮
```

**Row 1:** `╭` + `─` fill to active tab width + `╮`. Padded
left to the active tab's horizontal position. Remaining width
is spaces.

**Row 2:** `│` + active tab icon + title + `│`. Inactive tab
titles follow as plain `fg_dim` text separated by ` · `. Fill
to right edge.

**Row 3:** `─╯` + spaces for active tab inner width + `╰` +
`─` fill to right edge + `╮` (frame corner).

### Active tab positioning

The active tab's horizontal offset is computed from the widths
of all preceding tabs. When tab 1 is active, the bubble starts
at the left edge. When tab 2 is active, it shifts right past
tab 1's rendered width.

### Styling

- Active tab text: `accent_secondary` on `bg_base`
- Active tab borders: `bg_border`
- Inactive tab text: `fg_dim`
- Inactive tab separator: `·` in `fg_dim`
- Connecting line and frame corner: `bg_border`

### Overflow

If tabs exceed terminal width, rightmost inactive tabs are
truncated with `…`. Active tab is always fully visible.

## Status Bar

One row between content and footer. For 2.5b-1, renders only
normal status:

```
│ 󰇰  Inbox · 10 messages · 2 unread                  ● connected │
```

**Left side:** Folder icon + name, message count, unread count.
`fg_bright` on `bg_border`.

**Right side:** Connection indicator. `●` in `color_success`
+ "connected" text.

Full transient state (toast, undo, error) comes in 2.5b-6.

## Command Footer

Uses `bubbles/help` with a `KeyMap` struct. One row at the
bottom of the frame.

### Styling

- Key character: `fg_bright` bold
- Separator `:` and hint text: `fg_dim`

### Context variants

The footer `KeyMap` swaps based on active tab type and focused
panel:

**Message list (default):**
`d:del  a:archive  s:star  r:reply  R:all  f:fwd  c:compose  /:search  ?:help  ::cmd`

**Sidebar:**
`Enter:open  c:compose  ::cmd`

**Viewer:** (2.5b-4)
`d:del  a:archive  s:star  r:reply  R:all  f:fwd  Tab:links  q:close`

For 2.5b-1, only message list and sidebar contexts are wired.

## Focus Cycling

`Tab` key on the `AccountTab` toggles between `SidebarPanel`
and `MsgListPanel`.

### Visual indicators (2.5b-1 placeholder)

- Focused panel: `bg_selection` background on placeholder text
- Unfocused panel: no highlight

### Visual indicators (when real panels land)

- Sidebar focused: `┃` thick left border in `accent_primary`
  on selected row
- Sidebar unfocused: `bg_selection` background only, no `┃`
- Message list focused: `▐` right-half block in `accent_primary`
  at left edge of cursor row
- Message list unfocused: `bg_selection` background only, no `▐`

### Key routing

Only the focused panel receives key events. The `AccountTab`
checks its own `focusedPanel` before delegating j/k and other
panel-specific keys to the appropriate child.

## Styles Struct (`internal/ui/styles.go`)

Derived from `*theme.CompiledTheme`. Created once at startup.

```go
type Styles struct {
    // Tab bar
    TabActiveBorder  lipgloss.Style
    TabActiveText    lipgloss.Style
    TabInactiveText  lipgloss.Style
    TabConnectLine   lipgloss.Style

    // Content frame
    FrameBorder      lipgloss.Style
    PanelDivider     lipgloss.Style

    // Status bar
    StatusBar        lipgloss.Style
    StatusConnected  lipgloss.Style
    StatusReconnect  lipgloss.Style
    StatusOffline    lipgloss.Style

    // Footer
    FooterKey        lipgloss.Style
    FooterHint       lipgloss.Style

    // Selection (used by focus cycling)
    Selection        lipgloss.Style

    // Placeholder text
    Dim              lipgloss.Style
}
```

Additional styles will be added as sidebar (2.5b-2) and message
list (2.5b-3) land. The struct grows incrementally.

### Color mappings

| Style | Theme slot |
|-------|-----------|
| TabActiveText | `accent_secondary` on `bg_base` |
| TabActiveBorder | `bg_border` |
| TabInactiveText | `fg_dim` |
| TabConnectLine | `bg_border` |
| FrameBorder | `bg_border` |
| PanelDivider | `bg_border` |
| StatusBar | `fg_bright` on `bg_border` |
| StatusConnected | `color_success` |
| StatusReconnect | `color_warning` |
| StatusOffline | `fg_dim` |
| FooterKey | `fg_bright`, bold |
| FooterHint | `fg_dim` |
| Selection | `bg_selection` background |
| Dim | `fg_dim` |

## Mock Backend (`internal/mail/mock.go`)

Implements `mail.Backend` with hardcoded data. Returns
immediately (no blocking, no goroutines). Stays permanently
for development, testing, and demos.

### Mock data

- **Folders:** Inbox (3 unread), Drafts, Sent, Archive,
  Spam (12 unread), Trash, Notifications, Remind,
  Lists/golang, Lists/rust
- **Messages:** ~10 messages in Inbox matching the wireframe
  sample data (Alice Johnson, Bob Smith, Carol White, etc.)
- **Connection:** Always returns connected

The mock backend is not wired to the UI in 2.5b-1 — the
`AccountTab` uses it to populate the status bar counts. Real
backend wiring is Pass 3.

## File Layout

```
cmd/poplar/root.go           — tea.NewProgram, startup
internal/ui/app.go           — root model (App)
internal/ui/tab.go           — Tab interface
internal/ui/account_tab.go   — AccountTab model
internal/ui/tab_bar.go       — renderTabBar()
internal/ui/status_bar.go    — StatusBar model
internal/ui/footer.go        — footer KeyMap + rendering
internal/ui/styles.go        — Styles struct + NewStyles()
internal/ui/keys.go          — key bindings (bubbles/key)
internal/mail/mock.go        — mock Backend implementation
```

## Gate Condition

Pass 2.5b-1 is done when:

1. `poplar` binary launches a fullscreen bubbletea program
2. Tab bar renders with the 3-row bubble style (single tab)
3. Content area shows two-panel placeholder with `│` divider
4. Status bar shows mock folder stats + connection indicator
5. Command footer shows context-appropriate keybindings
6. `Tab` key cycles focus between panels (visible highlight)
7. `q` or `ctrl+c` exits cleanly
8. All colors come from the compiled Nord theme via `Styles`
9. `make check` passes

## Elm Architecture Compliance

All code in `internal/ui/` must follow:

- All state in `tea.Model` structs, no package-level mutable vars
- State changes only in `Update`
- All I/O in `tea.Cmd`
- Children signal parents via `Msg` types
- Shared state hoisted to root, passed down read-only
- Use `bubbles` components before building custom
- No raw ANSI — lipgloss only
- Child `Update` returns typed model, not `tea.Model`
- Every child `tea.Cmd` goes into `tea.Batch`
