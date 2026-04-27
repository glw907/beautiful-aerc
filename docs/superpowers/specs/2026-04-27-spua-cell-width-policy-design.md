# SPUA-A Cell-Width Policy: Design

Date: 2026-04-27
Status: design (pre-implementation)
Tracks: BACKLOG #20, supersedes ADR-0079, narrows ADR-0083
Re-audits: BACKLOG #16

## Problem

Pass 4.1 finding F2 declared the message-list right-border jitter
"fixed" by the +1-cell SPUA-A correction in `displayCells`. The fix
was never visually confirmed by the user; the alignment defect has
in fact persisted on the workstation throughout. ADR-0079's premise
— that "every modern terminal renders SPUA-A as 2 cells" — is false
in the workstation's actual configuration (kitty + `JetBrainsMonoNL
Nerd Font` via `symbol_map`), where SPUA-A glyphs render at 1 cell.
The +1 has been an unforced overcount since it was introduced.

The cell width of an SPUA-A glyph (U+F0000–U+FFFFD) is determined
by the runtime triple of terminal × font × symbol-map config. No
static policy spans the matrix:

- kitty + `JetBrainsMonoNL` + `symbol_map` → 1 cell
- kitty + `Hack Nerd Font` (Mono) → 2 cells
- alacritty/foot/gnome-terminal + system mono (no Nerd Font) → tofu
  fallback at 1 cell, no glyphs visible

Empirical research (see brainstorm transcript) shows ~10–15% of
fresh-install users on Linux/macOS get Nerd Fonts working zero-
config; the rest see tofu. ADR-0079 / ADR-0083's discipline solved
the wrong problem with the wrong assumption.

## Goal

Pick a policy that is robust across the kitty/alacritty/wezterm/
foot/gnome-terminal × Mono Nerd Font / symbol-mapped variable-width
/ no-Nerd-Font matrix, with zero required user setup on the default
path, and a one-line config override when autodetection is wrong.

## Decision

Three-mode iconography with autodetection by default.

### Config

A new field in `[ui]` (`internal/config/UIConfig`):

```toml
[ui]
icons = "auto"   # "auto" (default) | "simple" | "fancy"
```

`auto` is the shipped default. `simple` and `fancy` are explicit
overrides for users where autodetection is wrong.

### Mode resolution

At `cmd/poplar/main.go` startup, before `tea.NewProgram(...)`:

```
case cfg.UI.Icons:
  "simple": effectiveMode = simple
  "fancy":  effectiveMode = fancy
  "auto":   effectiveMode = fancy if term.HasNerdFont() else simple

if effectiveMode == fancy:
    w := term.MeasureSPUACells()
    if w == 1 || w == 2: ui.SetSPUACellWidth(w)
    else:                ui.SetSPUACellWidth(2)   // probe failed; assume Mono
else:
    ui.SetSPUACellWidth(1)

iconSet := ui.SimpleIcons
if effectiveMode == fancy:
    iconSet = ui.FancyIcons
```

The `App` constructor takes the resolved `IconSet` and threads it
into `Sidebar`, `MessageList`, `Viewer`, and chrome.

### Two icon tables

`internal/ui/icons.go` defines:

```go
type IconSet struct {
    Inbox, Drafts, Sent, Archive, Spam, Trash string
    Flag, Attachment, Thread                   string
    StatusConnected, StatusReconnecting, StatusOffline string
    Search, Warning                            string
    // additional surfaces as the codebase grows
}

var SimpleIcons = IconSet{
    Inbox:   "▣",   // U+25A3
    Drafts:  "✎",   // U+270E
    Sent:    "✉",   // U+2709
    Archive: "▢",   // U+25A2
    Spam:    "⚠",   // U+26A0 (verify Narrow class on target terminals)
    Trash:   "✗",   // U+2717
    Flag:    "⚑",   // U+2691
    // ...
}

var FancyIcons = IconSet{
    Inbox:   "",
    Drafts:  "",
    Sent:    "",
    // ... existing SPUA-A inventory
}
```

Every rune in `SimpleIcons` must be East Asian Width "Na" or "N"
— strictly Narrow under Unicode UAX #11, so `lipgloss.Width` is
correct. The unit test `TestSimpleIcons_AllNarrow` asserts every
rune returns `lipgloss.Width == 1`; it fails the build if an
Ambiguous-class codepoint sneaks in. Concrete per-surface choices
(e.g. whether to use ⚠ U+26A0 vs `!` for warning) are an
implementation detail; the structural decision locked here is "the
table contains only Narrow-class runes."

`FancyIcons` is the existing SPUA-A inventory unchanged.

### Width math

`internal/ui/iconwidth.go` keeps `displayCells` and `displayTruncate`
but they become parameterized on a package-level `spuaCellWidth int`
set once at startup via `ui.SetSPUACellWidth(int)`:

```go
var spuaCellWidth = 1   // default before SetSPUACellWidth is called

func displayCells(s string) int {
    return lipgloss.Width(s) + (spuaCellWidth - 1) * spuaCount(s)
}
```

In simple mode, `spuaCellWidth = 1`; the helper degenerates to
`lipgloss.Width` (still correct, no overhead). In fancy mode, the
helper applies the measured per-glyph correction.

`displayTruncate` keeps its current loop shape; the loop terminates
in `(spuaCellWidth - 1) * spuaCount(s)` iterations (0 in simple
mode, ≤ K in fancy mode where K is the number of SPUA-A glyphs in
the string).

### Composition

ADR-0083's ban on `lipgloss.JoinHorizontal` / `lipgloss.JoinVertical`
for SPUA-A-bearing rows is **narrowed**, not lifted. The
restriction now applies only when `spuaCellWidth != 1`. In simple
mode and in fancy mode on narrow-Nerd-Font terminals, `Join*` is
safe.

The existing manual row-by-row join code in `AccountTab.View` and
`App.renderFrame` is **kept unchanged** in this pass. It is correct
under both width regimes, and reverting it to `Join*` is a future
cleanup (out of scope here).

## Components

### `internal/term/` — new package

Two responsibilities, each isolated:

#### `term.HasNerdFont() bool`

Cached after first call. Implementation:

- Use `github.com/adrg/sysfont` (MIT, pure-Go) to enumerate installed
  fonts. `sysfont.NewFinder(nil).List()` returns `[]*sysfont.Font`
  with `Family` field.
- Return true if any `Family` contains `"Nerd Font"` or matches the
  ` NF` suffix pattern.
- `sysfont` covers `~/.local/share/fonts` (the actual Ubuntu/Mint
  user-install path), `~/.fonts`, `/usr/share/fonts`,
  `/usr/local/share/fonts` on Linux; `~/Library/Fonts`,
  `/Library/Fonts`, `/System/Library/Fonts` on macOS.
- Returns false on enumeration failure or unsupported OS.

`sysfont` is the only new third-party dependency. ~1 MB compiled.

#### `term.MeasureSPUACells() int`

Returns the rendered cell width (1 or 2) of an SPUA-A glyph in the
current terminal+font, or 0 on any failure. The probe pattern
follows `hymkor/go-cursorposition.AmbiguousWidth` (MIT — vendored
with attribution as `internal/term/probe.go`):

1. `term.IsTerminal(os.Stdin.Fd())` — if false, return 0.
2. Open `/dev/tty` directly. Using `/dev/tty` rather than stdin
   means the probe works correctly even if stdin is redirected,
   and isolates the probe's raw-mode lifecycle from whatever
   bubbletea later does with stdin.
3. `term.MakeRaw` on the tty fd; defer `term.Restore`.
4. Write `\x1b[6n` and read CPR via `unix.Select` with 200ms timeout.
   Parse `\x1b[<row>;<col>R`. Record `colBefore`.
5. Write the test SPUA-A glyph (``).
6. Write `\x1b[6n` again; read+parse. Record `colAfter`.
7. Return `colAfter - colBefore`.

Use `github.com/charmbracelet/x/ansi` (already a transitive
dependency via bubbletea) for CPR response parsing. Vendor the
`/dev/tty`-open and `MakeRaw` orchestration; do not depend on
`go-cursorposition` directly.

The probe is pre-tea, synchronous, with a hard 200ms timeout. On
timeout, missing TTY, or unparseable response: return 0.

### `internal/ui/icons.go` — new file

`IconSet` struct, `SimpleIcons`, `FancyIcons` package vars. Static.
No constructor — these are compiled values in the spirit of
`internal/theme/`.

### `internal/ui/iconwidth.go` — refactor

- Replace `const spuaAStart/spuaAEnd` correction logic with
  package-level `spuaCellWidth int` (default 1).
- Add `func SetSPUACellWidth(w int)` — called once at startup.
  Asserts `w == 1 || w == 2`; panics on other values (programmer
  error). Idempotent.
- `displayCells(s) = lipgloss.Width(s) + (spuaCellWidth - 1) * spuaCount(s)`.
- Existing fast-path ASCII byte scan is preserved (unchanged).

### `cmd/poplar/main.go` — startup wiring

Insert mode-resolution block after config load, before
`tea.NewProgram`. ~15 LOC. Threads resolved `IconSet` into
`ui.NewApp(...)`.

### `cmd/poplar/diagnose.go` — new cobra subcommand

`poplar diagnose` runs the full detection pipeline, prints the
result to stdout in a stable format, exits without launching tea.
Output:

```
Terminal:
  TERM           = xterm-kitty
  COLORTERM      = truecolor
  TTY            = /dev/pts/3
  is_terminal    = true

Fonts:
  detected       = JetBrainsMonoNL Nerd Font, ...
  has_nerd_font  = true
  source         = sysfont

Probe:
  glyph          = U+F422
  cpr_before     = col 1
  cpr_after      = col 2
  cell_width     = 1
  duration       = 1.4ms

Resolved:
  config.icons   = auto
  effective_mode = fancy
  spua_cell_w    = 1
  icon_set       = FancyIcons
```

Used by:
- The pass-end manual verification matrix.
- Future user troubleshooting.
- Regression check whenever a new terminal+font combo is reported.

`diagnose` is the empirical receipt that prevents the
"declared-fixed-without-verification" failure mode that produced
this bug in the first place.

### `internal/ui/app.go` and children — IconSet plumbing

`App` accepts an `IconSet` in its constructor and stores it. Sidebar,
MessageList, Viewer, and chrome reference `m.icons.<surface>` rather
than literal codepoints. Existing callsites that hardcode glyphs
are migrated as part of this pass.

## Failure modes

| Situation                                    | Resolved mode | spuaCellWidth | User experience          |
|---|---|---|---|
| `auto` + no NF installed                     | simple        | 1             | Unicode icons, aligned   |
| `auto` + NF installed, terminal uses it (Mono)| fancy        | 2 (probed)    | NF icons, aligned        |
| `auto` + NF installed, terminal uses it (narrow) | fancy     | 1 (probed)    | NF icons, aligned        |
| `auto` + NF installed, terminal *not* using it | fancy       | 1 (probed)    | tofu boxes; user sets `icons = "simple"` |
| `auto` + non-TTY (CI, pipe)                  | simple        | 1             | (no UI rendered)         |
| `simple` forced                              | simple        | 1             | Unicode icons            |
| `fancy` forced + probe succeeds              | fancy         | 1 or 2        | NF icons, aligned        |
| `fancy` forced + probe fails                 | fancy         | 2 (default)   | NF icons; alignment if Mono, off if narrow |

The honest limitation, called out in the new ADR: font *presence*
is a proxy for "will glyphs render," not a guarantee. The `simple`
override exists precisely for the "NF installed but terminal
configured to use a different font" edge case. We cannot
disambiguate tofu-rendered-at-1-cell from narrow-NF-rendered-at-1-
cell via DSR alone — the cursor moves the same amount in both.

## Testing strategy

Five layers, all required before claiming the work done.

### 1. Unit tests

- `internal/term/cpr_parse_test.go`: table-driven CPR byte
  sequences → expected (row, col) or parse error. Includes
  malformed responses, partial reads, embedded BEL, etc.
- `internal/term/font_detect_test.go`: mock the sysfont enumerator
  behind a small interface; table-driven family-list scenarios →
  expected `HasNerdFont` bool. Includes empty list, list with `NF`
  suffix, list without any NF, mixed case.
- `internal/term/icon_resolve_test.go`: `(configMode, hasNF,
  probeResult) → (effectiveMode, spuaCellWidth)` truth table.
  All 12 combinations enumerated explicitly.
- `internal/ui/icons_test.go`:
  - `TestSimpleIcons_AllNarrow`: every rune in `SimpleIcons`
    returns `lipgloss.Width == 1`. Fails the build if an Ambiguous-
    class codepoint sneaks in.
  - `TestFancyIcons_AllSPUA`: every rune in `FancyIcons` is in
    `[U+F0000, U+FFFFD]`.

### 2. Probe round-trip with `creack/pty`

`internal/term/probe_pty_test.go`. Spawns a pty pair. A goroutine
on the slave side reads master output; when it sees `\x1b[6n`,
writes back a configured `\x1b[<row>;<col>R` reply.

Cases:
- Slave replies col=2 after glyph (width 1) → probe returns 1.
- Slave replies col=3 after glyph (width 2) → probe returns 2.
- Slave never replies → 200ms timeout fires, probe returns 0.
- Slave replies with malformed bytes → parse fails, probe returns 0.

### 3. Width regression tests (extend existing)

Pass 4.1's `TestMessageList_RowWidthEqualAcrossReadStates` and
`TestApp_RightBorderAlignment` are parameterized across:
- `(spuaCellWidth=1, SimpleIcons)`
- `(spuaCellWidth=1, FancyIcons)` — the workstation case
- `(spuaCellWidth=2, FancyIcons)` — the Mono Nerd Font case

Every fixture row asserts `displayCells(row) == m.width` exactly.

### 4. Manual visual verification matrix

Document at `docs/poplar/testing/icon-modes.md`:

| # | Terminal       | Font                        | `icons` cfg | Expected mode | Expected `spua_cell_w` | Visual check                  |
|---|---|---|---|---|---|---|
| 1 | kitty          | JetBrainsMonoNL + symbol_map | auto       | fancy         | 1                       | borders aligned, fancy icons   |
| 2 | kitty          | Hack Nerd Font (Mono)       | auto        | fancy         | 2                       | borders aligned, fancy icons   |
| 3 | kitty          | DejaVu Mono (no NF)         | auto        | simple        | 1                       | borders aligned, Unicode icons |
| 4 | kitty          | JetBrainsMonoNL — `simple`  | simple      | simple        | 1                       | borders aligned, Unicode icons |
| 5 | kitty          | DejaVu Mono — `fancy`       | fancy       | fancy         | (probe-driven)         | borders aligned, tofu (expected) |
| 6 | gnome-terminal | system default              | auto        | simple        | 1                       | borders aligned                |
| 7 | alacritty      | system default              | auto        | (depends)     | 1                       | borders aligned                |
| 8 | tmux ⇒ kitty (#1) | —                        | auto        | fancy         | 1                       | borders aligned in pane        |

For each row:
1. Run `poplar diagnose`, paste output into the matrix doc.
2. Launch poplar, capture a tmux snapshot per
   `.claude/docs/tmux-testing.md`.
3. Verify visual alignment against the snapshot.

The matrix gates the pass: no row may be left unchecked before
shipping.

### 5. Re-audit BACKLOG #16

The original "sidebar misalignment" report and ADR-0079's premise
get a retroactive note in the new ADR's context section: on this
workstation, SPUA-A has always been 1 cell, the +1 was always
wrong, and the previously-claimed fix was never visually
confirmed. BACKLOG #16's `[x]` line gets a brief amendment pointing
to the new ADR for the corrected explanation.

This is institutional record-keeping, not code work — but it's the
case study that justifies the "measure, don't assume" rule and
should be the first paragraph of the new ADR.

## ADR impact

- **Supersedes ADR-0079** (`displayCells` "+1 always" correction).
  The premise is wrong. The new ADR cites this design and the
  brainstorm transcript.
- **Narrows ADR-0083** (`lipgloss.Join*` ban). The ban applies only
  when `spuaCellWidth != 1`. The existing manual-join code is kept
  in this pass.
- **New ADR**: `0084-icon-mode-policy-with-runtime-probe.md` —
  documents the three-mode model, the autodetection logic, the
  fallback semantics, and the testing requirement.

## Pass scope

One focused pass. Estimated commit sequence:

1. Add `internal/term/font.go` (sysfont wrapper) + tests.
2. Add `internal/term/probe.go` (vendored CPR pattern) + pty tests.
3. Add `internal/ui/icons.go` with `SimpleIcons` + `FancyIcons`
   + `IconSet`; add unit tests.
4. Refactor `iconwidth.go`: package-level `spuaCellWidth` +
   `SetSPUACellWidth`.
5. Thread `IconSet` through `App` → `Sidebar` / `MessageList` /
   `Viewer` / chrome. Migrate hardcoded glyph callsites.
6. Wire startup mode resolution in `cmd/poplar/main.go`.
7. Add `cmd/poplar/diagnose.go` cobra subcommand.
8. Extend regression tests (parameterize across modes).
9. Write ADR-0084; update ADR-0079 (status: superseded by 0084)
   and ADR-0083 (status: narrowed by 0084).
10. Update `invariants.md`, `styling.md`, `bubbletea-conventions.md`
    for the narrowed `Join*` discipline.
11. Update `BACKLOG.md`: close #20, retroactive note on #16.
12. Manual matrix verification; commit
    `docs/poplar/testing/icon-modes.md` with diagnose output for
    each row.

## Out of scope

- **Reverting manual row-joins to `lipgloss.Join*` in simple mode.**
  Future cleanup pass once we have empirical confidence that the
  narrowed restriction holds.
- **Per-icon size tuning** (e.g. picking the perfect Unicode
  alternative for each surface). The `SimpleIcons` table can be
  refined in follow-up; the structural decision (use a Narrow-class
  set) is what this pass locks in.
- **Linux-specific TTY quirks** (BSD/illumos/etc.). Linux + macOS
  only, matching the rest of poplar's target.
- **Windows.** Out of poplar's target entirely.

## Dependencies

New: `github.com/adrg/sysfont` (MIT). Pure-Go.

Vendored (with attribution): ~50 LOC from
`github.com/hymkor/go-cursorposition` (MIT) into
`internal/term/probe.go`.

Already present (transitive): `golang.org/x/term`,
`github.com/charmbracelet/x/ansi`, `github.com/creack/pty`
(test-only).
