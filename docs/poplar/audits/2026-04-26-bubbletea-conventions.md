# Bubbletea Conventions Audit — 2026-04-26

## Summary

11 findings: 3 high, 5 medium, 3 low.

---

## Findings

### A1 — Nerd Font SPUA-A icons measured as 1 cell; rows overflow terminal width (BACKLOG #16)

**Severity:** high
**File:** `internal/ui/sidebar.go:199–221`, `internal/ui/msglist.go:16–22`, `internal/ui/sidebar_search.go:151–172`
**Rule:** §8 Anti-patterns — "Rune-counting Nerd Font icons. SPUA-A glyphs (U+F0000+) often render double-width but `runewidth` reports 1."
**Evidence:**
```go
// sidebar.go:209–221 — icon is a SPUA-A glyph, lipgloss.Width underreports it
leftContent := indicator + bgStyle.Render(" ") + icon + bgStyle.Render("  ") + name
leftWidth := lipgloss.Width(leftContent)          // icon counted as 1 cell
gap := max(1, s.width-leftWidth-countWidth-rightMargin)  // gap is 1 too large
row := leftContent +
    bgStyle.Render(strings.Repeat(" ", gap)) +
    countStr +
    bgStyle.Render(strings.Repeat(" ", rightMargin))
return fillRowToWidth(row, s.width, bgStyle)  // rw == s.width, no correction
```
```go
// msglist.go:16–22 — flag column hardcoded as 1 cell; icons render as 2
// Flag cell is 1 cell wide because lipgloss.Width reports Nerd Font glyphs as 1 cell
mlFixedWidth = 1 + 2 + 1 + 2 + mlSenderWidth + 2 + 2 + mlDateWidth + 1
```
All nine folder icons (`󰇰 󰏫 󰑚 󰀼 󰍷 󰩺 󰂚 󰑴 󰡡`), three message-list flag icons (`󰇮 󰑚 󰈻`), and the search-shelf icon (`󰍉`) are SPUA-A (U+F0000–U+FFFFD). Each renders as 2 terminal cells but `lipgloss.Width` and `runewidth.StringWidth` both report 1.
**Why it matters:** Every sidebar row is 1 display cell wider than its allocated column. `JoinHorizontal` trusts the block's reported width (it calls the same underlying `StringWidth`), so the sidebar block is padded to the wrong baseline. The resulting row overflows the terminal width by 1 cell per icon. The terminal soft-wraps, displacing subsequent sidebar content into the message-list column. Confirmed as a visible rendering defect in Pass 3 (BACKLOG #16 documents the specific symptom: body text bleeding into the sidebar column on the Google Ads message).
**Suggested fix:** Introduce an explicit icon-cell table in `internal/ui/` mapping each SPUA-A codepoint to its actual display width (2 in every tested terminal). Replace calls to `lipgloss.Width` / `runewidth.StringWidth` that measure strings containing these icons with a helper that sums display widths using the table. Update `mlFixedWidth` and the sidebar row-layout arithmetic accordingly. Consider introducing a `displayCells(s string) int` utility that wraps `runewidth.StringWidth` and overrides for the known SPUA-A set.

---

### A2 — `spinner.TickMsg` consumed by `AccountTab`; viewer loading spinner is frozen

**Severity:** high
**File:** `internal/ui/account_tab.go:160–167`
**Rule:** §5 Async I/O — "Mutation discipline: Never in a `tea.Cmd` closure." (Corollary: messages with matching types must be routed to their intended recipients.)
**Evidence:**
```go
case spinner.TickMsg:
    if m.loading {
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    // Drop ticks while not loading; the next openFolderCmd will re-Tick.
    return m, nil  // ← early return; viewer spinner tick is discarded
```
The "forward any other Msg to viewer" block at line 173 is unreachable for `spinner.TickMsg` because the `case` handler always returns. When the user opens a message (`loading == false`), `AccountTab` drops every `spinner.TickMsg` — including the ticks that the viewer's loading-phase spinner emitted via `SpinnerTick()`.
**Why it matters:** The viewer's "Loading message…" spinner never advances past its initial frame. The placeholder is frozen for the entire body-fetch duration, which is visually broken and makes the client look hung during slow fetches.
**Suggested fix:** Replace the blanket early-return with a check on which spinner the tick belongs to: if `m.spinner.ID() == msg.ID`, handle it; otherwise fall through to the viewer forwarding block. Alternatively, invert the guard so the viewer receives `TickMsg` first when it is open (viewer loading takes priority since folder loading can't happen simultaneously with viewer loading in the current UX).

---

### A3 — All actionable key dispatch uses `msg.String()` switches; `key.Binding` fields are unused for dispatch

**Severity:** high
**File:** `internal/ui/app.go:117–151`, `internal/ui/account_tab.go:212–258`, `internal/ui/viewer.go:159–165`
**Rule:** §4 Key bindings — "`key.Matches(msg, m.KeyMap.Up)` is the canonical dispatch. String switches are listed as an anti-pattern (ref-apps §8 avoid #4)."
**Evidence:**
```go
// app.go:117–151 — GlobalKeys struct exists but is never consulted
case tea.KeyMsg:
    if m.helpOpen {
        switch msg.String() {
        case "?", "esc":
            m.helpOpen = false
        }
        return m, nil
    }
    switch msg.String() {
    case "q":
        ...
    case "ctrl+c":
        return m, tea.Quit
    case "?":
        ...

// account_tab.go:212–258 — entire key dispatch is a string switch
switch msg.String() {
case "/":
    ...
case "J":
    ...
case "enter":
    return m.openSelectedMessage()
```
```go
// keys.go — declarations exist but dispatch never calls key.Matches
type GlobalKeys struct {
    Help key.Binding
    Quit key.Binding
}
```
**Why it matters:** Bindings are invisible to `bubbles/help` (though the popover is a custom modal per ADR-0071, not `bubbles/help`, so the visibility gap is less severe here). More critically, `key.Matches` respects the `Enabled()` flag, enabling declarative per-state activation/deactivation. String switches cannot be disabled without adding manual state checks inline, making conditional binding logic scattered and hard to audit. This pattern also blocks any future rebinding or keybinding-config feature.
**Suggested fix:** Declare a `KeyMap` struct for `AccountTab` (and extend `GlobalKeys` for `App`) using `key.Binding` values. Replace every `switch msg.String()` dispatch in `app.go`, `account_tab.go`, and `viewer.go` with `key.Matches(msg, m.keys.X)` chains. This is medium-effort but pays off immediately in correctness (disabled-binding safety) and future-proofs the code for the help vocabulary integration.

---

### A4 — `fillRowToWidth` only pads, never truncates; over-wide rows are returned unchanged

**Severity:** medium
**File:** `internal/ui/styles.go:107–112`
**Rule:** §2 Sizing contract — every component must honor its width contract. §8 Anti-patterns: the component, not the parent, is responsible for enforcing line width.
**Evidence:**
```go
func fillRowToWidth(row string, width int, bgStyle lipgloss.Style) string {
    if rw := lipgloss.Width(row); rw < width {
        return row + bgStyle.Render(strings.Repeat(" ", width-rw))
    }
    return row  // ← no truncation when rw >= width
}
```
`fillRowToWidth` is the width-enforcement function shared by sidebar, message list, and search shelf row renderers. If a row's measured width already equals or exceeds `width`, it is returned as-is even if it is actually wider than the target (as happens whenever an icon is measured incorrectly per A1). There is no `ansi.Truncate` fallback.
**Why it matters:** Any row that overflows its column due to measurement error, unexpected content, or a future styling change will pass through silently, violating the contract that `JoinHorizontal` depends on. This converts what should be a bounded layout defect into an unbounded one — the caller has no safety net.
**Suggested fix:** Add an `else if rw > width` branch that calls `ansi.Truncate(row, width, "")` to clip over-wide rows. This does not replace fixing A1; it provides a defensive catch-all for measurement-error cases.

---

### A5 — `AccountTab` does not forward `WindowSizeMsg` to children as a message

**Severity:** medium
**File:** `internal/ui/account_tab.go:95–105`
**Rule:** §2 Sizing contract — "Parent also forwards `WindowSizeMsg` into each child's `Update`. Bubbles components (viewport, textarea, list) rely on the message to reinitialise scroll state." (ref-apps §8 avoid #6)
**Evidence:**
```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    sw := min(sidebarWidth, m.width/2)
    folderHeight := max(1, m.height-sidebarHeaderRows-searchShelfRows)
    m.sidebar.SetSize(sw, folderHeight)
    m.sidebarSearch.SetSize(sw)          // ← SetSize only, no Update(msg)
    mw := max(1, m.width-sw-1)
    m.msglist.SetSize(mw, m.height)      // ← SetSize only, no Update(msg)
    m.viewer = m.viewer.SetSize(mw, m.height)  // ← SetSize only
    return m, nil
```
The `textinput` embedded in `SidebarSearch` has `Width = 0` (never set) and never receives a `WindowSizeMsg`. The viewer's `bubbles/viewport` is reconstructed via `layout()` during `SetSize`, sidestepping the forwarding requirement for that specific component — but the pattern still deviates from the documented norm.
**Why it matters:** The `textinput.Width = 0` means the prompt input renders at natural (unbounded) width. For typical short queries the sidebar shelf clips visually via `fillRowToWidth`, but for queries longer than the available sidebar column width (~25 chars) the input content overflows before `fillRowToWidth` applies its padding. The mismatch also makes the code fragile if a future component (e.g. a `bubbles/list` for folder picking) is added and relies on `WindowSizeMsg`.
**Suggested fix:** In `SetSize`, also call `m.sidebarSearch.input.Width = width - iconAndPrefixOverhead` to limit the textinput's display width. Forward `msg` into `m.sidebarSearch.Update(msg)` so the textinput's internal state is in sync. Forwarding `msg` through `m.viewer.Update(msg)` covers the viewport case and brings the pattern in line with the ref-app norm.

---

### A6 — `SidebarSearch.textinput.Width` is never set; long queries overflow the sidebar column

**Severity:** medium
**File:** `internal/ui/sidebar_search.go:30–41`, `internal/ui/sidebar_search.go:47–50`
**Rule:** §2 Sizing contract — components own their size contract; their `View()` must not produce lines wider than the assigned width.
**Evidence:**
```go
func NewSidebarSearch(styles Styles, width int) SidebarSearch {
    ti := textinput.New()
    ti.Prompt = "/"
    ti.CharLimit = 0      // unlimited input
    // Width is never set — textinput.Width defaults to 0
    return SidebarSearch{input: ti, ...}
}

func (s *SidebarSearch) SetSize(width int) {
    s.width = width
    // input.Width is not updated here
}
```
With `ti.Width = 0`, `textinput.View()` renders the full query string without any horizontal scrolling. The query is composed into the prompt row at `renderPromptRow`; `fillRowToWidth` then uses `lipgloss.Width` to right-pad. For queries longer than `~s.width - 4` cells, the rendered line exceeds `s.width` before padding is applied, and `fillRowToWidth` returns it uncorrected (see A4).
**Why it matters:** A sufficiently long search query causes the sidebar prompt row to overflow its column. `JoinHorizontal` will observe a wider sidebar block and produce a layout that displaces the divider and message-list column, breaking the horizontal layout at runtime in response to user input.
**Suggested fix:** In `SetSize`, assign `s.input.Width = max(1, width-promptOverhead)` where `promptOverhead` accounts for the indent spaces and icon prefix. This makes `textinput` internally clip/scroll the query at the column boundary so `textinput.View()` always produces a string within the available width.

---

### A7 — Help popover has no width floor; overflows at narrow terminal widths

**Severity:** medium
**File:** `internal/ui/help_popover.go:157–189`
**Rule:** §2 Sizing contract — `View()` must honor assigned width in all branches.
**Evidence:**
```go
func (h HelpPopover) View(width, height int) string {
    // ... builds box from static content ...
    return lipgloss.Place(
        width, height,
        lipgloss.Center, lipgloss.Center,
        popover,       // ← Place is a no-op if popover > width
    )
}
```
`lipgloss.Place` is documented as a no-op when content is larger than the target dimension (norms §4: "Both are no-ops if the content is already larger than the box in that dimension"). The account-context help popover renders three columns of groups plus a Go-To grid; the rendered box is approximately 60–65 cells wide. Below that terminal width, `Place` returns the unclipped popover, which overflows the terminal and may cause content wrapping in both the horizontal and vertical dimensions.
**Why it matters:** At ≤60 terminal columns (narrow tmux splits, embedded terminal panes), the help overlay renders incorrectly and may obscure the underlying layout in an unrecoverable way until `?` or `Esc` is pressed.
**Suggested fix:** Before calling `lipgloss.Place`, measure `lipgloss.Width(popover)`. If it exceeds the available `width`, either render a condensed single-column version of the binding tables, or apply `ansi.Truncate` per line to clip the popover at `width`. Since poplar targets 80+ columns as a practical minimum, a simple guard that renders a "terminal too narrow" message below a threshold (e.g. 60 cols) is acceptable.

---

### A8 — Viewer loading placeholder (`lipgloss.Place`) is not guarded by `clipPane`

**Severity:** medium
**File:** `internal/ui/viewer.go:209–215`
**Rule:** §2 Sizing contract — `clipPane` is the canonical enforcer; all `View()` branches must be clipped.
**Evidence:**
```go
if v.phase == viewerLoading {
    text := v.spinner.View() + " Loading message…"
    return lipgloss.Place(     // ← Place, not clipPane
        v.width, v.height,
        lipgloss.Center, lipgloss.Center,
        v.styles.Dim.Render(text),
    )
}
// Only the ready branch uses clipPane:
out := lipgloss.JoinVertical(lipgloss.Left, v.headerStr, v.viewport.View())
return clipPane(out, v.width, v.height)
```
`lipgloss.Place` centers content that is smaller than the target and is a no-op when content is larger. For the loading placeholder, the spinner text is short, so `Place` produces correctly-sized output in practice. But the two `View` branches use different size-contract enforcement strategies. If the spinner text were ever changed to include long content (e.g. a filename during compose), the loading branch would silently overflow.
**Why it matters:** Inconsistent enforcement means a future edit to the loading-phase content could introduce an overflow that is easy to miss in review. The `ready` branch's use of `clipPane` is the pattern the conventions doc explicitly endorses.
**Suggested fix:** Wrap the `lipgloss.Place` output in `clipPane` before returning it, making both branches symmetric. Alternatively, inline the `clipPane` logic (pad + truncate) on the `Place` output.

---

### A9 — Zero-latency intra-model `tea.Cmd` wrappers for parent signaling

**Severity:** low
**File:** `internal/ui/cmds.go:321–329`, `internal/ui/viewer.go:163,187,192`, `internal/ui/account_tab.go:296,327`
**Rule:** §5 Async I/O — "tea.Cmd is not for intra-model messaging. That can almost always be done in the update function." (`tea.go:62–64`)
**Evidence:**
```go
// cmds.go:325–329
func viewerOpenedCmd() tea.Cmd { return func() tea.Msg { return ViewerOpenedMsg{} } }
func viewerClosedCmd() tea.Cmd { return func() tea.Msg { return ViewerClosedMsg{} } }
func viewerScrollCmd(pct int) tea.Cmd {
    return func() tea.Msg { return ViewerScrollMsg{Pct: pct} }
}

// Similarly: folderChangedCmd emits FolderChangedMsg as a zero-latency Cmd
```
`ViewerOpenedMsg`, `ViewerClosedMsg`, `ViewerScrollMsg`, and `FolderChangedMsg` are all emitted from `AccountTab` (child) to be caught by `App` (parent). They carry no I/O and exist only to signal parent state changes. The bubbletea comment explicitly flags this as an anti-pattern: "there's almost never a reason to use a command to send a message to another part of your program. That can almost always be done in the update function."
**Why it matters:** These Cmds run asynchronously. There is a one-frame delay between the child state change and the parent receiving the signal (footer context, status bar mode). This is currently invisible because the user can't act between frames, but it adds an extra round-trip through the event loop and makes causal ordering harder to reason about (e.g. `ViewerScrollMsg` fires *after* the viewer has already advanced scroll state, so the status bar always lags by one frame).
**Suggested fix:** `App.Update` should check `m.viewerOpen` vs the child's `IsOpen()` state after delegating, and directly update chrome fields (`m.footer`, `m.statusBar`) based on the child's new state — no Cmd necessary. `FolderChangedMsg` is the hardest case because `App.Update` currently can't inspect `AccountTab`'s selected folder; one option is to expose a `SelectedFolderInfo()` method on `AccountTab` and call it directly in `App.Update` after delegation.

---

### A10 — `App.View` performs parent-side per-line padding of child content

**Severity:** low
**File:** `internal/ui/app.go:170–176`
**Rule:** §8 Anti-patterns — "Defensive parent-side clipping. A parent calling `MaxWidth` papers over a child contract violation. Fix the child instead." (The inverse — parent-side *padding* — is a weaker form of the same violation.)
**Evidence:**
```go
rawContent := m.acct.View()
rightBorder := m.styles.FrameBorder.Render("│")
contentLines := strings.Split(rawContent, "\n")
for i, line := range contentLines {
    pad := max(0, m.width-1-lipgloss.Width(line))
    contentLines[i] = line + strings.Repeat(" ", pad) + rightBorder
}
```
`App.View` iterates every line of `m.acct.View()` to measure it and add padding before the right border. This means `App` is performing parent-side layout adjustment on child output. It also means if any line is shorter than `m.width-1` (which can happen for the loading placeholder when content is narrower), `App` pads it. Lines *longer* than `m.width-1` silently receive the border beyond the terminal edge (no truncation).
**Why it matters:** The right border attachment could be expressed more cleanly if `AccountTab.View()` strictly honored its width contract — `App.View` could then append the border string without measuring each line. The current approach works but obscures the contract: `App` is doing post-processing on `AccountTab`'s output rather than trusting it.
**Suggested fix:** Ensure `AccountTab.View()` produces lines of exactly `m.width-1` cells (currently `m.acct` receives `contentMsg.Width = m.width-1`). Then `App.View` can append the border character without per-line measurement. As a transitional measure, add an `ansi.Truncate` guard alongside the existing padding so over-wide lines are also handled.

---

### A11 — `GlobalKeys` key.Binding fields declared but not consulted in `App.Update`

**Severity:** low
**File:** `internal/ui/keys.go`, `internal/ui/app.go:115–151`
**Rule:** §4 Key bindings — "Poplar should keep using `key.Binding` for declaration even when help text is just `"k"` rather than `"↑/k"`."
**Evidence:**
```go
// keys.go — binding declarations
type GlobalKeys struct {
    Help key.Binding
    Quit key.Binding
}

// app.go:115–151 — no call to key.Matches anywhere
case tea.KeyMsg:
    if m.helpOpen {
        switch msg.String() {
        case "?", "esc":  // ← string literal, not m.keys.Help
```
`GlobalKeys` is instantiated in `NewApp` and stored on the model, but `App.Update` never calls `key.Matches(msg, m.keys.Help)` or `key.Matches(msg, m.keys.Quit)`. The bindings are purely decorative.
**Why it matters:** Dead declarations add confusion — readers expect `m.keys.Help` to be used somewhere. The `keys.go` pattern is sound and should be propagated forward to actual dispatch; leaving it unused is a maintenance trap (future editors might remove it or add new bindings that also never get wired).
**Suggested fix:** Replace the `switch msg.String()` in `App.Update`'s key handler with `key.Matches` calls using `m.keys.Help` and `m.keys.Quit`. This closes the gap identified in A3 at the root level and validates the declaration approach before rolling it out to `AccountTab`.

---

## Triage

| ID | Decision | Rationale |
|---|---|---|
| A1 | `[fix-now]` | Original motivator; user-visible layout defect. |
| A2 | `[fix-now]` | User-visible bug; small fix. |
| A3 | `[fix-now-partial] + [backlog]` | App.Update slice fixed in pass; AccountTab + Viewer migration backlogged as a dedicated structural-cleanup pass to keep this pass scoped. |
| A4 | `[fix-now]` | One-line defensive guard; complements A1. |
| A5 | `[fix-now]` | Conventions deviation; small fix. |
| A6 | `[fix-now]` | Couples with A5; actual overflow at long queries. |
| A7 | `[fix-now]` | User-visible at narrow widths. |
| A8 | `[fix-now]` | Tiny consistency fix. |
| A9 | `[backlog]` | Architecture-level refactor; current behaviour is correct, only the framing is off-norm. |
| A10 | `[backlog]` | Depends on the full A3 migration to land first. |
| A11 | `[fix-now]` | Subsumed by the A3 App.Update slice — same edits close it. |

## Dimensions with no findings

- **Component shape (§1):** All `Update` methods return concrete types. `Init` is correctly implemented for `App` and `AccountTab` (`return nil` equivalent via early return of typed Cmd). No `progress`-style `tea.Model` return type. No `NewModel` deprecated constructors.
- **State ownership (§1):** No package-level mutable state. No mutations in `View()`. No goroutines outside `tea.Cmd`. All `Cmd` closures capture scalar values (not model pointers).
- **Async I/O patterns (§5):** All blocking I/O lives inside `tea.Cmd`. `tea.Batch(cmds...)` pattern used correctly throughout. `pumpUpdatesCmd` uses the canonical channel + blocking-Cmd pattern. No `tea.Sequentially` (deprecated).
- **Program setup (§6):** `cmd/poplar/root.go:85` uses `tea.WithAltScreen()` as a constructor option — correct. No `EnterAltScreen()` in any `Init()`. No deprecated `WithANSICompressor`.
- **Text rendering in `content/` package (§3):** `content.render.go` uses `ansi.Hardwrap(ansi.Wordwrap(...))` correctly per the conventions.
- **`clipPane` implementation (§2):** The `clipPane` helper in `viewer.go:228–249` correctly implements the Width+Height pad / MaxWidth+MaxHeight truncate idiom using `ansi.Truncate` and `strings.Repeat`. Its implementation is sound.
- **`lipgloss.Width` usage for ANSI-safe measurement:** Outside of the SPUA-A icon issue (A1), all width math in the UI layer uses `lipgloss.Width` or `runewidth.StringWidth` rather than `len(s)`.
