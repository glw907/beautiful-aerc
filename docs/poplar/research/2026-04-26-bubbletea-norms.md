# Bubbletea Library Norms — Source Survey 2026-04-26

> **Note (2026-05-09):** Researched against bubbletea v1.3.10,
> lipgloss v1.1.x, bubbles v1.0.0. Pass 13.2 migrates poplar to
> `charm.land/{bubbletea,lipgloss,bubbles}/v2`; some upstream APIs
> cited below have been renamed, restructured, or removed in v2.
> The patterns this research grounds (size contract, JoinHorizontal
> trust, key.Binding declarations, Cmd-isolated I/O, etc.) all
> hold under v2 — only specific API names changed.
> `docs/poplar/bubbletea-conventions.md` supersedes for any
> conflict; load this doc only when the conventions doc is silent
> and primary sources are needed.

Survey of the four Charm libraries used in this project:
- `bubbles@v1.0.0`
- `glamour@v1.0.0`
- `lipgloss@v1.1.1-0.20250404203927-76690c660834`
- `bubbletea@v1.3.10`
- `x/ansi@v0.11.6`

Every normative claim below cites a specific file and line range.

---

## 1. Component Shape

### Update return type: concrete model, not tea.Model

Every bubbles component returns its own concrete type from `Update`, not `tea.Model`. This is the canonical pattern across the entire library:

- `viewport`: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)` — `viewport/viewport.go:402`
- `spinner`: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)` — `spinner/spinner.go:136`
- `help`: `func (m Model) Update(_ tea.Msg) (Model, tea.Cmd)` — `help/help.go:99`
- `stopwatch`: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)` — `stopwatch/stopwatch.go:111`
- `table`: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)` — `table/table.go:204`
- `textinput`: returns `(Model, tea.Cmd)` (pointer receiver `Focus`/`Blur`, value receiver `Update`)
- `textarea`: returns `(Model, tea.Cmd)` (same pattern)

Exception: `progress` uses `func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)` — `progress/progress.go:210`. This is the only outlier and is likely a vestige of an earlier API design.

### View signature

All components implement `func (m Model) View() string` except `help`, which takes an explicit argument: `func (m Model) View(k KeyMap) string` — `help/help.go:104`. The `KeyMap` argument is the `help.KeyMap` interface (not the component's own `KeyMap` struct).

### Init for composability

Components that do not need async startup implement `Init() tea.Cmd { return nil }` to satisfy `tea.Model` for embedding purposes. Examples: `viewport/viewport.go:78`, `progress/progress.go:201`. This is documentation-explicit: "exists to satisfy the tea.Model interface for composability purposes."

### State ownership patterns

- All mutable state lives in the `Model` struct. No globals, no package-level state beyond ID counters.
- Atomic ID counters (`var lastID int64`) are used by `spinner`, `progress`, `stopwatch`, `filepicker` to route tick messages. Each component instance gets a unique ID via `nextID()` and rejects ticks carrying a different ID. Example: `spinner/spinner.go:138-145`.
- Components that receive ticks also carry a `tag int` that increments on every tick. Stale ticks from a previous generation are dropped by comparing `msg.tag != m.tag`. This prevents runaway double-ticking. Example: `spinner/spinner.go:143-148`, `stopwatch/stopwatch.go:128-133`.

---

## 2. Sizing

### Constructor takes width and height

`viewport.New(width, height int)` — `viewport/viewport.go:15` stores them directly on the struct as public fields `Width` and `Height`. There is no `SetSize` method; callers reassign the fields or reconstruct.

`table.New(opts ...Option)` uses functional options: `WithHeight(h int)` and `WithWidth(w int)` — `table/table.go:169,176`. `WithHeight` subtracts the header row height from the viewport height: `m.viewport.Height = h - lipgloss.Height(m.headersView())` — `table/table.go:171`.

`list.New(items, delegate, width, height int)` — `list/list.go:207` takes width and height positionally; stores as private fields `m.width`, `m.height`.

`textinput` stores width as a public field `Width int` — `textinput/textinput.go:120`. The comment is explicit: "Width is the maximum number of characters that can be displayed at once. It essentially treats the text field like a horizontally scrolling viewport."

`textarea` has `defaultWidth = 40` and `defaultMaxWidth = 500` — `textarea/textarea.go:28-31`.

`filepicker` has `AutoHeight bool` field (default true) — `filepicker/filepicker.go:39`. When true, the component auto-sizes its visible rows.

### tea.WindowSizeMsg handling

`WindowSizeMsg` is defined in `bubbletea` as: `type WindowSizeMsg struct { Width int; Height int }` — `screen.go:7-10`.

The comment on `WindowSize()` command states: "Keep in mind that WindowSizeMsgs will automatically be delivered to Update when the Program starts and when the window dimensions change so in many cases you will not need to explicitly invoke this command." — `commands.go:216-217`.

No bubbles component handles `tea.WindowSizeMsg` internally. Every component expects the parent to resize it explicitly (by updating the struct fields or calling `SetContent` again). The parent is responsible for propagating resize events.

### View clipping in viewport

The canonical clipping idiom used by `viewport.View()` — `viewport/viewport.go:518-525`:

```go
contents := lipgloss.NewStyle().
    Width(contentWidth).      // pad to width
    Height(contentHeight).    // pad to height
    MaxHeight(contentHeight). // truncate height if taller
    MaxWidth(contentWidth).   // truncate width if wider
    Render(strings.Join(m.visibleLines(), "\n"))
return m.Style.
    UnsetWidth().UnsetHeight(). // Style size already applied in contents
    Render(contents)
```

This is the definitive example of guarding output to exact dimensions. `Width`+`Height` pad; `MaxWidth`+`MaxHeight` truncate. Unsetting the outer style's size after applying it to inner content avoids double-counting frame size.

The inner content dimensions account for frame size: `contentWidth := w - m.Style.GetHorizontalFrameSize()` — `viewport/viewport.go:516`.

---

## 3. Key Bindings

### Declaring bindings with key.Binding

The canonical declaration form from the package doc comment — `key/key.go:5-19`:

```go
type KeyMap struct {
    Up   key.Binding
    Down key.Binding
}

var DefaultKeyMap = KeyMap{
    Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "move up")),
    Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "move down")),
}
```

`key.WithKeys` sets the match strings; `key.WithHelp` sets the display pair for `bubbles/help`. Both are `BindingOpt` functions applied via `NewBinding` — `key/key.go:54-60`.

### Dispatching with key.Matches

The canonical dispatch form:

```go
case key.Matches(msg, m.KeyMap.Up):
```

`key.Matches` is generic: `func Matches[Key fmt.Stringer](k Key, b ...Binding) bool` — `key/key.go:130`. It returns true only when the binding is enabled (`b.Enabled()`) — `key/key.go:132-138`. Disabled bindings never match.

All bubbles components use `key.Matches` in their `Update`; none do raw string comparison on `msg.String()`. Example: `viewport/viewport.go:420-461`, `table/table.go:211-228`.

### Enabling/disabling bindings

`b.SetEnabled(v bool)` or `b.Unbind()` — `key/key.go:110-121`. `Unbind` is stronger than `SetEnabled(false)`: it also clears help text, so the binding disappears from help views without leaving an empty row. `Enabled()` returns false when keys slice is nil OR when `disabled` is true — `key/key.go:107-108`.

### Binding focus guards in Update

`table.Update` short-circuits to `return m, nil` when `!m.focus` — `table/table.go:205-207`. This is the canonical pattern for components that can be unfocused: check focus first and return early with no mutation.

### help.KeyMap interface

For a component's `KeyMap` to integrate with `bubbles/help`, it must implement:

```go
type KeyMap interface {
    ShortHelp() []key.Binding
    FullHelp() [][]key.Binding
}
```
— `help/help.go:18-27`

`table.KeyMap` implements this interface — `table/table.go:55-65`. `ShortHelp` returns a flat slice; `FullHelp` returns a slice of slices where each inner slice becomes a column.

`help.ShortHelpView` truncates items that would exceed `m.Width` by appending an ellipsis — `help/help.go:141-147`. `FullHelpView` uses `JoinHorizontal(Top, ...)` per column, then joins all columns horizontally — `help/help.go:198,218`.

Disabled bindings (`!kb.Enabled()`) are silently skipped in both short and full view — `help/help.go:124`, `help/help.go:189`.

---

## 4. Layout Primitives

### JoinHorizontal

`lipgloss.JoinHorizontal(pos Position, strs ...string) string` — `join.go:28`. Behavior with mismatched-height inputs:

- Finds the tallest block's line count as `maxHeight` — `join.go:44-53`.
- Pads all shorter blocks with empty string lines according to `pos`: `Top` appends empty lines at the bottom; `Bottom` prepends; any other value splits the gap by the fractional position — `join.go:55-78`.
- Then pads each line to its block's max width with spaces (right-pads every line to the widest line in that block) — `join.go:88`.

Critical: **JoinHorizontal right-pads every line in every block to that block's widest line**. A block containing `"abc\nde\nf"` outputs lines of width 3, 3, 3 — not 3, 2, 1. The caller must not add extra padding on top of this or content will be double-padded.

### JoinVertical

`lipgloss.JoinVertical(pos Position, strs ...string) string` — `join.go:116`. Aligns each line of every block to `maxWidth` via `pos`: left-pads for `Right`, right-pads for `Left`, splits for center — `join.go:141-162`.

### Place

`lipgloss.Place(width, height int, hPos, vPos Position, str string, opts ...WhitespaceOption) string` — `position.go:36`. It composes `PlaceHorizontal` then `PlaceVertical` — `position.go:43`. Both are no-ops if the content is already larger than the box in that dimension — `position.go:56`, `position.go:112`. Used for centering overlays (e.g. modal dialogs) within a known terminal size.

### Style.Width vs Style.MaxWidth

- `Style.Width(i int)`: "sets the width of the block before applying margins. The width, if set, also determines where text will wrap." — `set.go:226-228`. Wrapping is done with `cellbuf.Wrap(str, wrapAt, "")` — `style.go:368`. The result is then right-padded by `alignTextHorizontal` so all lines reach `width`. Width is a minimum and a wrap target.
- `Style.MaxWidth(n int)`: "useful in enforcing a certain width at render time, particularly with arbitrary strings and styles." — `set.go:613-614`. Applied after all padding, borders, and margins by truncating each line: `ansi.Truncate(lines[i], maxWidth, "")` — `style.go:454`. MaxWidth is a ceiling: it never wraps, only cuts.

Summary: use `Width` to size and wrap a block you own; use `MaxWidth` as a safety truncation on content whose final width is uncertain.

### Style.Inline

`Style.Inline(true)` strips all newlines (`\n` → `""` — `style.go:362`), skips padding, borders, and margins — `style.go:361-447`. Used for rendering key/desc pairs in `help.ShortHelpView` — `help/help.go:121,136-137`.

---

## 5. Text Rendering

### ansi.Wordwrap (soft wrap at word boundaries)

`ansi.Wordwrap(s string, limit int, breakpoints string) string` — `x/ansi/wrap.go:128`. Does not break mid-word unless a word exceeds the limit by itself. The `breakpoints` argument is a string of characters (each 1-cell wide) that are treated as break opportunities in addition to whitespace; hyphen is always a break point — `x/ansi/wrap.go:121-126`. Non-breaking space (U+00A0, `0xA0`) is treated as a word character and is never a break point — `x/ansi/wrap.go:195`.

### ansi.Hardwrap (hard wrap at exact column)

`ansi.Hardwrap(s string, limit int, preserveSpace bool) string` — `x/ansi/wrap.go:21`. Breaks at `limit` cells regardless of word boundaries. When `preserveSpace` is false (the common default), leading spaces on newly-created lines are skipped — `x/ansi/wrap.go:65-70`. Use when content must never overflow a fixed column.

### ansi.Truncate

`ansi.Truncate(s string, length int, tail string) string` — `x/ansi/truncate.go:53`. Truncates to `length` cells, appending `tail` (e.g. `"…"`) in place of the removed content. `tail` width is subtracted from the available `length` — `x/ansi/truncate.go:71-73`. Returns `s` unchanged when `StringWidth(s) <= length` — `x/ansi/truncate.go:67-69`. ANSI escape codes are preserved and never cut mid-sequence.

### ansi.Cut

`ansi.Cut(s string, left, right int) string` — `x/ansi/truncate.go:15`. Extracts the cell range `[left, right)` without adding any prefix or tail. Used by `viewport.visibleLines()` for horizontal scrolling — `viewport/viewport.go:159`. Internally composed of `TruncateLeft` + `Truncate`.

### ansi.Strip

`ansi.Strip(s string) string` — `x/ansi/width.go:10`. Removes all ANSI escape codes; returns printable characters only.

### ansi.StringWidth

`ansi.StringWidth(s string) int` — `x/ansi/width.go:65`. Cell width of a string: ignores ANSI escapes, accounts for wide characters (East Asian, emoji). Uses grapheme cluster segmentation. The `Wc` variants (`StringWidthWc`, `HardwrapWc`, etc.) use the simpler `WcWidth` method instead of grapheme clustering — `x/ansi/width.go:74`.

### Glamour width control

`glamour.WithWordWrap(wordWrap int) TermRendererOption` — `glamour/glamour.go:191`. Sets `ansiOptions.WordWrap`, which is passed as `WordWrap` to the underlying ansi renderer. The default is `defaultWidth = 80` — `glamour/glamour.go:27`. When the caller wants exact terminal-width rendering, it passes `WithWordWrap(terminalWidth)` at construction time. There is no runtime width change; a new `TermRenderer` must be created when the terminal resizes. Glamour style config is JSON-based (`WithStylePath`, `WithStyles`, `WithStylesFromJSONBytes`) — `glamour/glamour.go:147,164,172`.

---

## 6. Program-Level Patterns

### tea.Model interface

Three methods, defined in `tea.go:44-56`:

```go
type Model interface {
    Init() Cmd
    Update(Msg) (Model, Cmd)
    View() string
}
```

`Cmd` is `func() Msg` — `tea.go:65`. A nil `Cmd` is a no-op.

### NewProgram and common options

`tea.NewProgram(model, opts...)` — options are applied via `ProgramOption` functions. Most commonly used:

- `WithAltScreen()` — start in fullscreen alternate buffer — `options.go:109-113`. Note: `EnterAltScreen()` as a command should *not* be used in `Init()` "Because commands run asynchronously" — `screen.go:29-31`. Use the constructor option instead.
- `WithMouseCellMotion()` — click/release/drag/wheel events — `options.go:137-141`. Better supported than AllMotion.
- `WithMouseAllMotion()` — all motion including hover — `options.go:162-166`. "Many modern terminals support this, but not all."
- `WithReportFocus()` — delivers `FocusMsg`/`BlurMsg` on terminal focus change — `options.go:248-252`. Requires tmux to be configured to report focus events.
- `WithInput(nil)` — disable stdin input — `options.go:38`.
- `WithFPS(fps int)` — cap renderer FPS (default 60, max 120) — `options.go:235-239`.

### tea.Batch and tea.Sequence

`tea.Batch(cmds ...Cmd) Cmd` — runs commands concurrently, no ordering guarantee — `commands.go:15`. Nil commands in the list are silently dropped by `compactCmds` — `commands.go:36-53`. When only one non-nil cmd is present, `compactCmds` returns that cmd directly (no wrapper) — `commands.go:47`.

`tea.Sequence(cmds ...Cmd) Cmd` — runs commands one at a time, in order — `commands.go:25`. Contrast with `Batch`.

### tea.Tick and tea.Every

`tea.Tick(d time.Duration, fn func(time.Time) Msg) Cmd` — starts timer from now, fires once — `commands.go:154`. Must be returned again from `Update` on each tick message to produce a loop. Example in the doc:

```go
case TickMsg:
    return m, doTick() // return the next tick to loop
```
— `commands.go:150`.

`tea.Every(duration, fn)` ticks in sync with the system clock (wall-clock aligned) — `commands.go:102`. Use `Tick` for component-owned timers; `Every` for wall-clock alignment (e.g., "update every full second").

Comment in source: "there's almost never a reason to use a command to send a message to another part of your program. That can almost always be done in the update function." — `tea.go:62-64`.

### WindowSizeMsg delivery

Delivered automatically on program start and on every terminal resize — `commands.go:216-217`. Also available as explicit command `tea.WindowSize()` for querying current size — `commands.go:218`.

### Spinner Init/Tick pattern

Spinner ticking requires the parent to call `cmd = m.spinner.Tick` as the initial command (not via `Init`), then pass `TickMsg` through to `m.spinner.Update` in the parent's `Update`. The model-level `tick` method uses `tea.Tick` with the spinner's FPS — `spinner/spinner.go:189-195`. The deprecated package-level `Tick()` function (no args) produces a `TickMsg` with no ID, which would be picked up by all spinners — `spinner/spinner.go:203`.

### Stopwatch Init

`stopwatch.Model.Init()` returns `m.Start()`, making `Init` non-nil — `stopwatch/stopwatch.go:72-74`. `Start()` uses `tea.Sequence` to first send a `StartStopMsg` and then schedule the first tick — `stopwatch/stopwatch.go:78-81`.

---

## 7. Anti-Patterns Called Out by the Libraries

### HighPerformanceRendering is deprecated

`viewport.Model.HighPerformanceRendering` bool field: "Deprecated: high performance rendering is now deprecated in Bubble Tea." — `viewport/viewport.go:62-63`. The comment in `View()` still conditionally emits newlines for the deprecated path but normal rendering is the only supported path. The related `Sync`, `ViewDown`, `ViewUp` package-level functions are also deprecated — `viewport/viewport.go:166,374,391`.

### EnterAltScreen in Init is wrong

`screen.go:29-31`: "Because commands run asynchronously, this command should not be used in your model's Init function. To initialize your program with the altscreen enabled use the WithAltScreen ProgramOption instead." Same warning applies to `EnableMouseCellMotion` — `screen.go:59-62` — and `EnableMouseAllMotion` — `screen.go:77-80`.

### Commands are not for intra-model messaging

`tea.go:62-64`: "there's almost never a reason to use a command to send a message to another part of your program. That can almost always be done in the update function." Using `tea.Cmd` to route messages between sub-models creates unnecessary async indirection.

### WithANSICompressor deprecated

`options.go:188-190`: "Deprecated: this incurs a noticeable performance hit. A future release will optimize ANSI automatically without the performance penalty." Do not use.

### Sequentially is deprecated

`commands.go:176-179`: `Sequentially` is deprecated; use `Sequence` instead.

### package-level spinner.Tick() is deprecated

`spinner/spinner.go:203`: `func Tick() tea.Msg` (package-level, no args) is deprecated. Use `model.Tick` (method form) instead, which embeds the spinner's ID and tag for proper routing — `spinner/spinner.go:175-187`.

### NewModel constructors are deprecated

Across all packages, `NewModel` is deprecated in favour of `New`:
- `spinner.NewModel` → `spinner.New` — `spinner/spinner.go:126`
- `help.NewModel` → `help.New` — `help/help.go:96`
- `list.NewModel` → `list.New` — `list/list.go:257`
- `textinput.NewModel` → `textinput.New` — `textinput/textinput.go:182`

### Misuse of Style.Width on outer wrapper after inner size is already applied

`viewport.View()` — `viewport/viewport.go:525` — calls `UnsetWidth().UnsetHeight()` on the outer style before rendering content that was already padded/truncated to the correct dimensions. If the outer style retained its `Width`/`Height`, the renderer would wrap and pad again, adding incorrect extra whitespace.

### list.SetFilterText calls a command synchronously

`list/list.go:288-290`: `cmd := filterItems(*m); msg := cmd()` — it synchronously executes the async filter command and casts the result directly. This is an intentional pattern for programmatic filter initialization but is explicitly non-Elm: the comment is absent, but the anti-pattern is present for a reason (avoid an async round-trip for initial state). Downstream code should not copy this pattern for normal state transitions.

---

## Summary of Key Interface Facts

| Component | Constructor | Update returns | Focus API | Size API |
|---|---|---|---|---|
| viewport | `New(w, h)` | `(Model, Cmd)` | none | public fields `Width`/`Height` |
| list | `New(items, delegate, w, h)` | `(Model, Cmd)` | none | private; `SetSize` not present, reconstruct |
| table | `New(opts...)` | `(Model, Cmd)` | `Focus()`/`Blur()` methods | `WithWidth`/`WithHeight` options; `SetStyles` triggers UpdateViewport |
| textinput | `New()` | `(Model, Cmd)` | `Focus() Cmd`/`Blur()` | public `Width int` field |
| textarea | `New()` | `(Model, Cmd)` | `Focus() Cmd`/`Blur()` | `SetWidth()`/`SetHeight()` methods |
| spinner | `New(opts...)` | `(Model, Cmd)` | none | no size |
| help | `New()` | `(Model, Cmd)` | none | public `Width int` field |
| progress | `New(opts...)` | **(tea.Model, Cmd)** | none | public `Width int` field |
| stopwatch | `New()` | `(Model, Cmd)` | none | no size |
| timer | `New(timeout, interval)` | `(Model, Cmd)` | none | no size |
| filepicker | `New()` | `(Model, Cmd)` | none | public `Height int`; `AutoHeight bool` |
| paginator | `New()` | `(Model, Cmd)` | none | no size |
