# Bubbletea Reference App Survey

**Date:** 2026-04-26
**Surveyor:** Claude Sonnet 4.6

## Apps and versions

| App | Tag / Commit | Role |
|---|---|---|
| charmbracelet/bubbletea examples | commit `640d879` (HEAD on main, ~v2.1.0) | canonical examples |
| charmbracelet/glow | `v1.5.1` / commit `ad21129` | multi-pane markdown reader |
| charmbracelet/gum | `v0.14.3` / commit `5d96f84` | per-command program primitives |
| charmbracelet/soft-serve | `v0.7.5` / commit `876db8d` | server TUI, multi-tab, multi-view |
| charmbracelet/wishlist | `v0.2.0` / commit `05611d3` | SSH list + form |
| dlvhdr/gh-dash | commit `9ac6fc9` (HEAD on main, ~v4.x) | community pick; see §6 |

All repos cloned to `/tmp/refapps/` for inspection. Permalinks below use
`github.com/<repo>/blob/<tag-or-commit>/<path>#L<N>` form.

---

## 1. App shape

### Single model vs nested model tree

**Flat state machine (glow, wishlist):** A single root model owns sub-models as
plain struct fields. The root Update dispatches on `state` or `page` iota to
decide which child to call.

- `glow/ui/ui.go:167–179` — `model` embeds `stash stashModel` and `pager
  pagerModel` as value fields; root dispatches on `state` enum
  (`stateShowStash` / `stateShowDocument`).
  https://github.com/charmbracelet/glow/blob/v1.5.1/ui/ui.go#L167

- `wishlist/wishlist.go:59–63` — single `listModel` struct; no sub-models;
  quit is `tea.Quit` from `Update` when enter is pressed.
  https://github.com/charmbracelet/wishlist/blob/v0.2.0/wishlist.go#L59

**Interface-based tree (soft-serve, gh-dash):** A `Component` interface
(embeds `tea.Model`, adds `SetSize`) allows typed collections of pages/panes.
The root owns `[]Component` and calls their `Update` inside a loop.

- `soft-serve/pkg/ui/common/component.go:9–13` — `Component` interface:
  `tea.Model + help.KeyMap + SetSize(width, height int)`.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/common/component.go#L9

- `soft-serve/pkg/ssh/ui.go:36–47` — `UI` owns `pages []common.Component`
  (index 0 = selection, 1 = repo); active page tracked by `activePage page`
  iota.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L36

- `gh-dash/internal/tui/ui.go:48–66` — root `Model` holds typed sidebar,
  prView, issueSidebar, etc. as value fields alongside `prs []section.Section`.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/ui.go#L48

**Shared context struct (soft-serve, gh-dash):** Both pass a mutable pointer
to a shared-context struct rather than plumbing individual fields through every
constructor.

- `soft-serve/pkg/ui/common/common.go:31–41` — `Common` struct carries
  `Width`, `Height`, `Styles`, `KeyMap`, `Logger`, `Renderer`; embedded in
  every component.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/common/common.go#L31

- `gh-dash/internal/tui/context/context.go:31–51` — `ProgramContext` carries
  screen dims, config, theme, error, `StartTask func`.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/context/context.go#L31

### Quit / sigint

All apps bind `q` and `ctrl+c` to `tea.Quit`. None handle `os.Signal`
directly; the runtime sends `tea.QuitMsg` on SIGINT.

- `bubbletea examples/help/main.go:62–65` — `key.Binding` named `Quit` with
  keys `"q", "esc", "ctrl+c"`.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/help/main.go#L62

- `glow/ui/ui.go:301–303` — `ctrl+c` in the global switch always quits,
  regardless of state.
  https://github.com/charmbracelet/glow/blob/v1.5.1/ui/ui.go#L301

- `bubbletea examples/prevent-quit/main.go:30–41` — `tea.WithFilter` intercepts
  `tea.QuitMsg` before it reaches the model, used to block quit when dirty state.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/prevent-quit/main.go#L30

---

## 2. Layout composition

### Multi-pane with lipgloss.JoinHorizontal

Every app with side-by-side panes uses `lipgloss.JoinHorizontal` as the
primary layout primitive. Clipping is pushed into each child; the parent joins.

- `bubbletea examples/split-editors/main.go:196–207` —
  `lipgloss.JoinHorizontal(lipgloss.Top, m.inputViews()...)` joins N editors;
  each editor is pre-sized by `SetWidth` in `sizeInputs()`.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/split-editors/main.go#L196

- `bubbletea examples/composable-views/main.go:128–131` —
  `lipgloss.JoinHorizontal(lipgloss.Top, focusedModelStyle.Render(...), modelStyle.Render(...))`;
  style encodes the border.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/composable-views/main.go#L128

### Margin accounting: subtract before delegating

Children receive sizes already reduced by chrome (tabs, status bar, padding).
The parent computes the margin, subtracts it, then calls `child.SetSize`.

- `soft-serve/pkg/ssh/ui.go:67–132` — `getMargins()` returns `(wm, hm int)`;
  `SetSize` calls `child.SetSize(width-wm, height-hm)` for all pages.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L67

- `soft-serve/pkg/ui/pages/repo/repo.go:87–105` — `Repo.SetSize` subtracts
  `hm` derived from tab height + statusbar height before passing to each pane.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/pages/repo/repo.go#L87

- `glow/ui/pager.go:167–180` — `setSize(w, h)` sets viewport dimensions to
  `h - statusBarHeight`, and further subtracts help height when shown.
  https://github.com/charmbracelet/glow/blob/v1.5.1/ui/pager.go#L167

### Tab row width measured from rendered output

Tabs are rendered first; the window content width is derived from the tab
row's rendered width, not from the terminal width directly.

- `bubbletea examples/tabs/main.go:114–117` —
  `row := lipgloss.JoinHorizontal(...)`, then
  `s.window.Width(lipgloss.Width(row)).Render(...)`.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/tabs/main.go#L114

### Modal overlays: View short-circuits, not stacked

No app stacks renderers or uses `tea.ExecProcess`. Modals replace `View()`
output at the root.

- `gum/choose/command.go:21–116` — `huh.NewForm(...).Run()` is a synchronous
  call wrapping its own program; gum treats each command as an independent
  `tea.NewProgram`.
  https://github.com/charmbracelet/gum/blob/v0.14.3/choose/command.go#L21

---

## 3. Help and key bindings

### bubbles/help + key.Binding is universal

Every surveyed app (except gum which relies on `huh`) uses `bubbles/help`
and `key.Binding`. No app hand-rolls a help footer from scratch.

- `bubbletea examples/help/main.go:16–65` — canonical `keyMap` struct
  implementing `ShortHelp()` and `FullHelp()`; `help.Model.View(m.keys)` in
  `View()`.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/help/main.go#L16

- `soft-serve/pkg/ssh/ui.go:86–119` — `UI` implements `ShortHelp()` and
  `FullHelp()` by delegating to the active page, then appending global
  bindings (`Quit`, `Help`).
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L86

- `soft-serve/pkg/ui/keymap/keymap.go:1–26` — `KeyMap` struct with all bindings
  as `key.Binding` fields; `DefaultKeyMap()` constructor.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/keymap/keymap.go#L1

### key.Matches over string switch

Production apps use `key.Matches(msg, binding)` for all actionable keys, not
`msg.String() == "j"` switches. String switches only appear for literal
character dispatch in text input contexts.

- `soft-serve/pkg/ssh/ui.go:185–199` — all key handling via `key.Matches`.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L185

- `gh-dash/internal/tui/ui.go:225–285` — entire key section is `key.Matches`
  calls.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/ui.go#L225

### help.SetWidth on WindowSizeMsg

`help.Model` must receive the terminal width to gracefully truncate. This
call is always paired with `WindowSizeMsg`.

- `bubbletea examples/help/main.go:89–91` — `m.help.SetWidth(msg.Width)` in
  the `tea.WindowSizeMsg` branch.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/help/main.go#L89

---

## 4. Window resize

### Pattern: store dims on root, delegate via SetSize

All multi-component apps store `width`/`height` on the root or shared context,
then fan out to children via `SetSize` or direct field assignment.

- `glow/ui/ui.go:307–311` — `WindowSizeMsg` sets `m.common.width/height`, then
  immediately calls `m.stash.setSize` and `m.pager.setSize`.
  https://github.com/charmbracelet/glow/blob/v1.5.1/ui/ui.go#L307

- `soft-serve/pkg/ssh/ui.go:122–132` — `UI.SetSize` calls `common.SetSize`,
  then `header.SetSize(w-wm, h-hm)` and all pages.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L122

- `soft-serve/pkg/ssh/ui.go:173–181` — `WindowSizeMsg` calls `ui.SetSize(w, h)`
  then loops over all pages calling `p.Update(msg)`.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L173

- `gh-dash/internal/tui/ui.go:825–826` — `WindowSizeMsg` dispatches to
  `m.onWindowSizeChanged(msg)`, which stores dims on the shared context and
  calls `syncMainContentDimensions()` and `syncSidebar()`.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/ui.go#L825

- `wishlist/wishlist.go:81–83` — `WindowSizeMsg` subtracts margin inline:
  `m.list.SetSize(msg.Width-left-right, msg.Height-top-bottom)`.
  https://github.com/charmbracelet/wishlist/blob/v0.2.0/wishlist.go#L81

### WindowSizeMsg also forwarded to children

In all multi-component apps, after root stores the dims, `WindowSizeMsg` is
forwarded into children so each child can also reflow internal state (viewport,
textarea, etc.).

- `soft-serve/pkg/ssh/ui.go:173–181` — loops `p.Update(msg)` for all pages
  on `WindowSizeMsg`.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ssh/ui.go#L173

- `bubbletea examples/split-editors/main.go:158–166` — `WindowSizeMsg` stores
  dims then calls `sizeInputs()`, which calls `SetWidth/SetHeight` on each
  textarea; also falls through to the `for i := range m.inputs` update loop
  that passes the msg to each textarea.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/split-editors/main.go#L158

---

## 5. Async I/O

### tea.Cmd is the only async boundary

All async work is a `tea.Cmd` (a `func() tea.Msg`). No goroutines are spawned
outside of Cmds. No `go func()` callbacks.

- `bubbletea examples/http/main.go:71–81` — `checkServer` is a plain function
  matching `func() tea.Msg`; the HTTP client runs inside it.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/http/main.go#L71

- `gum/spin/spin.go:62–95` — `commandStart` spawns `exec.Command` and blocks
  until done; returns `finishCommandMsg`.
  https://github.com/charmbracelet/gum/blob/v0.14.3/spin/spin.go#L62

### Channel-based polling for real-time push

For server-push or streaming data, the pattern is: one Cmd runs indefinitely
writing to a channel; a second Cmd blocks on `<-ch` and returns the next msg;
on receipt, `Update` re-fires the blocking Cmd.

- `bubbletea examples/realtime/main.go:24–37` — `listenForActivity` runs
  forever writing to a channel; `waitForActivity` blocks on the channel and
  returns `responseMsg`; `Update` fires `waitForActivity` again on each receipt.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/realtime/main.go#L24

- `gum/spin/spin.go:97–103` — `Init` batches spinner tick + `commandStart` +
  timeout together.
  https://github.com/charmbracelet/gum/blob/v0.14.3/spin/spin.go#L97

### Spinner ticker coupled to ID

In nested-model apps with multiple spinners, each spinner's `TickMsg` carries
an ID; the parent checks the ID before delegating.

- `soft-serve/pkg/ui/pages/repo/repo.go:231–249` — `spinner.TickMsg` handler
  checks `r.spinner.ID() == msg.ID`; if not, iterates panes looking for the
  matching `SpinnerID()`.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/pages/repo/repo.go#L231

---

## 6. Theming and styling

### Named color variables + AdaptiveColor (light/dark pairs)

Every production app separates color palette from style application.
Colors are named package-level `var` blocks; styles use those named colors.

- `glow/ui/styles.go:7–31` — 20+ named color vars (`normal`, `indigo`,
  `fuchsia`, etc.) all as `AdaptiveColor{Light: ..., Dark: ...}`.
  https://github.com/charmbracelet/glow/blob/v1.5.1/ui/styles.go#L7

- `bubbletea examples/tabs/main.go:12–43` — `newStyles(bgIsDark bool)` returns
  a `*styles` struct; `lipgloss.LightDark(bgIsDark)` picks the variant;
  all styles constructed once on startup.
  https://github.com/charmbracelet/bubbletea/blob/640d879/examples/tabs/main.go#L12

### Styles struct passed through shared context

Large apps do not scatter `lipgloss.NewStyle()` calls. They construct a
`Styles` struct once and thread it through a shared context or embed it.

- `soft-serve/pkg/ui/styles/styles.go:11–110` — `Styles` struct with named
  sub-structs (`Repo`, `LogItem`, `Ref`, etc.) each carrying normal and active
  variants.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/styles/styles.go#L11

- `soft-serve/pkg/ui/common/common.go:31–41` — `Common.Styles *styles.Styles`
  is the single styles reference; every component accesses styles via
  `c.common.Styles.*`.
  https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/common/common.go#L31

### Theme struct for semantic tokens (gh-dash)

gh-dash separates palette from semantics: `Theme` struct fields are semantic
role names (`PrimaryBorder`, `SelectedBackground`, `SuccessText`, etc.) with
`AdaptiveColor` values. Components use semantic names, not hex literals.

- `gh-dash/internal/tui/theme/theme.go:12–37` — `Theme` struct with role-named
  `compat.AdaptiveColor` fields; `DefaultTheme` var with ANSI color values.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/theme/theme.go#L12

### Render functions on package-level vars (glow anti-pattern note)

Glow calls `.Render` immediately on construction to produce `func(string) string`
closures stored in package-level vars. This pre-bakes styles and cannot respond
to window-size or theme changes at runtime.

- `glow/ui/styles.go:36–64` — `normalFg = NewStyle().Foreground(normal).Render`
  — illustrates the pattern.
  https://github.com/charmbracelet/glow/blob/v1.5.1/ui/styles.go#L36

---

## Community pick: dlvhdr/gh-dash

**Selected because:** gh-dash is the most-starred non-Charm bubbletea application
(6 000+ GitHub stars as of 2026), appears in every "Awesome Bubbletea" list,
and has received explicit Charm community recognition in blog posts and tweets.
It is also architecturally significant: it ported from bubbletea v1 to v2 and
uses the full production stack (`key.Binding`, `bubblezone`, shared context,
semantic theme, channel-based tasks).

Key patterns that distinguish it from the Charm-owned apps:

- **`ProgramContext` pointer threading:** All children receive a `*ProgramContext`
  and call `UpdateProgramContext(ctx)` every frame via `syncProgramContext()`.
  This avoids plumbing dozens of individual fields through constructors.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/ui.go#L1033

- **`StartTask func(Task) tea.Cmd` on context:** Async tasks are launched
  through a function field on the shared context, not through Cmd plumbing.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/context/context.go#L48

- **bubblezone for mouse hit-testing:** `zone.Manager` on the shared context;
  zone IDs registered in `View()`, hit-tested in `Update()`.
  https://github.com/dlvhdr/gh-dash/blob/9ac6fc9/internal/tui/ui.go#L183

---

## 7. Patterns to emulate

1. **`Component` interface with `SetSize(w, h int)`** — used by both
   soft-serve and gh-dash. Lets the root resize all children uniformly without
   type-switching. (soft-serve `component.go:9`, gh-dash uses the same contract
   informally.)

2. **Margin subtraction before `SetSize`** — all multi-pane apps compute chrome
   height (tabs + statusbar + padding) and pass `height-hm` to children. The
   child's `View()` fills exactly the space it was given.
   (soft-serve `ui.go:122`, `repo.go:97`; glow `pager.go:167`.)

3. **Root Update short-circuits on focused input** — when a text input (search,
   filter, note) is focused, all apps check first and route the message there,
   returning immediately. No key falls through to navigation.
   (glow `ui.go:274–279`; gh-dash `ui.go:184–188`.)

4. **`tea.Batch(cmds...)` from a `[]tea.Cmd` slice** — every app builds a
   slice then passes it as variadic args. No nesting of `tea.Batch` calls.
   (soft-serve `ui.go:145`; gh-dash `ui.go:168`; glow `ui.go:232`.)

5. **Semantic `AdaptiveColor` in a `Theme` / `Styles` struct** — glow, soft-serve,
   and gh-dash all separate palette from semantics and structure styles into
   named sub-structs. No caller sees a raw hex literal.

6. **Channel + blocking-Cmd for server push** — the `listenForActivity` /
   `waitForActivity` split (bubbletea `realtime` example) is the canonical
   real-time pattern. One goroutine produces; the Cmd consumes one event at a
   time and re-schedules itself.

7. **`help.ShortHelpView([]key.Binding{...})` for inline footer** — when
   `bubbles/help` is used as a footer line rather than a toggleable panel,
   apps call `ShortHelpView` directly with the relevant slice.
   (bubbletea `split-editors/main.go:197`; `prevent-quit/main.go:143`.)

---

## 8. Patterns to avoid

1. **Package-level `.Render` closures** — glow's `normalFg = NewStyle()....Render`
   idiom bakes styles at init time. These cannot adapt to runtime theme or
   size changes. Prefer a `Styles` struct constructed (or refreshed) when the
   theme or size is known.
   (glow `ui/styles.go:36–64`.)

2. **Large monolithic `Update` functions** — glow's root `Update` at ~200 lines
   mixes state-machine dispatch with child delegation inline. gh-dash's
   `ui.go:166–870` (700 lines) is even larger. Both are hard to extend.
   The alternative demonstrated by soft-serve: short root Update with
   single-responsibility helpers (`setStatusBarInfo`, `updateModels`, etc.).

3. **`commonModel` as a value embedded in every child** — glow embeds
   `common *commonModel` as a pointer; this works but requires manual nil
   checks. Soft-serve's `Common` embed and gh-dash's `*ProgramContext`
   argument are cleaner: they make the shared context explicit and
   constructors receive it as a parameter.

4. **String switch for key handling in production apps** — glow's stash
   and pager use `msg.String()` switches throughout. This means key
   rebinding is impossible and the bindings are invisible to `bubbles/help`.
   Both soft-serve and gh-dash use `key.Matches` exclusively for actionable
   keys.
   (glow `ui/stash.go` vs soft-serve `ui/keymap/keymap.go`.)

5. **Calling `Init()` again inside `Update()`** — soft-serve calls
   `r.Init()` inside `Update` on `RepoMsg`
   (`repo.go:156`). This re-fires all child `Init` Cmds and can produce
   duplicate spinner ticks. Prefer a dedicated reset method that returns
   only the Cmds needed for the new state.
   https://github.com/charmbracelet/soft-serve/blob/v0.7.5/pkg/ui/pages/repo/repo.go#L156

6. **Not forwarding `WindowSizeMsg` to children** — wishlist does not forward
   `WindowSizeMsg` to `m.list.Update`; it only sets size via `SetSize`.
   Bubbles components (viewport, textarea, list) rely on receiving the msg to
   reinitialise scroll state. Always both call `SetSize` and forward the msg.
   (wishlist `wishlist.go:81–88`; contrast soft-serve `ui.go:173–181`.)
