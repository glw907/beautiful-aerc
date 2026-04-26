---
title: Error banner + spinner consolidation (Pass 2.5b-6)
status: accepted
date: 2026-04-25
---

# Error banner + spinner consolidation

Pass 2.5b-6 captures backend errors that prior passes dropped silently
and standardizes the spinner placeholder so future consumers (Pass 3
folder-load, Pass 9 send) inherit a single style. It is the
last-prototype consolidation pass before live-backend wiring (Pass 3).

## Goals

- Surface every `mail.Backend` Cmd error to the user via a persistent
  one-line banner above the status bar.
- Promote the existing `backendErrMsg` private type to an exported
  `ErrorMsg` carrying the failing operation name, so any future Cmd
  (compose send, IMAP worker, research worker) emits the same shape.
- Hoist banner ownership to `App` so the surface is global chrome,
  not an `AccountTab` concern. Mirrors the help-popover pattern from
  Pass 2.5b-5 (ADR-0071).
- Centralize the spinner style behind a single `styles.NewSpinner()`
  helper. Future spinner consumers call this helper rather than
  constructing `bubbles/spinner.Model` locally.

## Non-goals

- Multi-error queue, animation, or auto-dismiss timer (Pass 6 toast +
  undo bar will replace this surface with a queue).
- A dismiss key. The banner is last-write-wins; no key clears it.
- Spinner consumers beyond the viewer body-load placeholder.
- A new spinner wrapper type. The shared style is a constructor, not
  an abstraction layer.
- Restructuring the existing per-Cmd error path. `cmds.go` already
  wraps backend errors; this pass only renames + exports + populates
  an `op` field.

## Decisions settled in brainstorming

1. **Banner anchoring + dismissal.** Above-footer, persistent,
   replaces on next error (last-write-wins). No dismiss key in v1.
   Rationale: closest precursor to the Pass 6 toast/undo bar (same
   surface, evolves into it); preserves the modifier-free + no
   `:`-mode invariants by adding zero new bindings.
2. **Banner styling.** Tinted text only — `⚠ <message>` in
   `ColorError` foreground, no background fill, single line,
   truncated with `…` to fit terminal width. Filled bar option was
   rejected as visually competing with the status bar; multi-line
   wrap was rejected to keep layout stable.
3. **Spinner reuse.** Shared style helper, not a wrapper model.
   `styles.NewSpinner() spinner.Model` returns a configured
   `spinner.Dot` with foreground = `FgDim` (so the spinner does not
   compete with the cursor accent). Callers continue to embed
   `spinner.Model` directly.
4. **Error-stream wiring.** App owns `lastErr`. `backendErrMsg` is
   renamed and exported as `ErrorMsg` with an added `op string` field
   for human-readable context. App.Update intercepts `ErrorMsg`,
   stores it, and continues delegating so child models still
   progress on other state. `AccountTab`'s prior empty handler is
   deleted.

## Architecture

### Error type

```go
// ErrorMsg carries a failure from any tea.Cmd. The banner surface
// reads the most recent ErrorMsg and renders "⚠ <op>: <err>".
type ErrorMsg struct {
    Op  string // human-readable operation, e.g. "mark read"
    Err error
}
```

Lives in `internal/ui/cmds.go` (replacing the unexported
`backendErrMsg`). Existing call sites populate `Op`:

| Cmd                       | Op             |
|---------------------------|----------------|
| `loadFoldersCmd`          | `list folders` |
| `openFolderCmd`           | `open folder`  |
| `fetchBodyCmd`            | `fetch body`   |
| `markReadCmd`             | `mark read`    |
| (Pass 2.5b-4 xdg-open)    | `open link`    |

### App ownership

`App` gains `lastErr ErrorMsg` (zero value = no banner). `App.Update`
intercepts `ErrorMsg` before delegation:

```go
case ErrorMsg:
    a.lastErr = msg
    // fall through: still delegate so child models advance
```

`App.View` composes (top to bottom):

1. If `helpOpen` → `help.View(...)` (unchanged short-circuit).
2. Otherwise: `accountTab.View(width, height - bannerH - statusH)`.
3. Banner row (one line, only rendered if `lastErr.Err != nil`).
4. Status bar.

`bannerH` is `1` when an error is present, `0` otherwise. Account
region shrinks; no overlay.

`AccountTab.Update` no longer handles `ErrorMsg` (the `backendErrMsg`
case is removed). The forward-to-viewer fallthrough at the end of
`AccountTab.Update` is unaffected.

### Banner rendering

New file: `internal/ui/error_banner.go`. Pure function:

```go
// renderErrorBanner formats an ErrorMsg for the single banner row.
// Returns "" if msg.Err is nil. Output is exactly one cell tall and
// at most width cells wide, truncated with "…" if the formatted
// "⚠ <op>: <err>" string overflows.
func renderErrorBanner(msg ErrorMsg, width int, styles *Styles) string
```

Style: `Styles.ErrorBanner = lipgloss.NewStyle().Foreground(t.ColorError)`.
The leading `⚠ ` glyph is part of the rendered string, not a separate
style. Truncation uses `lipgloss.Width` and runewise trimming so
multi-byte glyphs do not split.

### Spinner style helper

New helper in `internal/ui/styles.go`:

```go
// NewSpinner returns a configured bubbles/spinner.Model with the
// poplar default style (Dot variant, FgDim foreground). Callers
// embed the returned model directly.
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
    sp := spinner.New()
    sp.Spinner = spinner.Dot
    sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
    return sp
}
```

Viewer's `New(...)` constructor switches from local construction to
`styles.NewSpinner(theme)`.

## Data flow

```
mail.Backend method fails
        │
        ▼
cmds.go: tea.Cmd returns ErrorMsg{Op: "...", Err: err}
        │
        ▼
App.Update sees ErrorMsg → a.lastErr = msg → delegates further
        │
        ▼
App.View → renderErrorBanner(a.lastErr, width, styles)
        │
        ▼
JoinVertical(account, banner, statusBar)
```

A subsequent successful Cmd does **not** clear the banner. Only the
next `ErrorMsg` replaces it. (Pass 6 will introduce explicit clearing
when the toast bar gains a queue + dismiss key.)

## Files touched

| File                                  | Change                              |
|---------------------------------------|-------------------------------------|
| `internal/ui/cmds.go`                 | rename type, add `Op`, populate     |
| `internal/ui/account_tab.go`          | delete `backendErrMsg` case         |
| `internal/ui/app.go`                  | add `lastErr`, intercept, render    |
| `internal/ui/styles.go`               | `NewSpinner` helper, `ErrorBanner`  |
| `internal/ui/viewer.go`               | use `NewSpinner`                    |
| `internal/ui/error_banner.go` (new)   | banner formatter                    |
| `internal/ui/error_banner_test.go`    | render / truncate / nil tests       |
| `internal/ui/app_test.go`             | banner integration                  |
| `docs/poplar/styling.md`              | banner surface in palette map       |

## Tests

### `error_banner_test.go`
- nil `Err` returns empty string regardless of width.
- non-nil `Err` formats as `⚠ <op>: <err message>`.
- truncates with `…` when formatted string exceeds width.
- ColorError foreground appears in the rendered output (assert ANSI
  sequence presence, mirroring existing style assertions).
- multi-byte glyphs in `Err.Error()` truncate at rune boundaries.

### `app_test.go`
- `App.Update(ErrorMsg{...})` stores the message in `lastErr` and
  the next `View()` contains `⚠ ` + the op string.
- Second `ErrorMsg` replaces the first (last-write-wins).
- View height accounting: account region renders one row shorter
  when the banner is present (assert via line count).
- Banner suppressed while help popover is open (popover takes the
  full screen — `helpOpen` short-circuits before banner composition).

### Existing tests
- `account_tab_test.go` and any other test referencing
  `backendErrMsg` are updated for the renamed exported type.

## Risks / open questions

- **Severity coloring.** Future warnings (non-fatal) may want a
  different color than fatal errors. Out of scope for this pass; the
  `ErrorMsg` struct can grow a `Level` field later without breaking
  callers (zero value stays "error").
- **Truncation aesthetics.** A long error from JMAP may truncate
  away the actionable suffix. Acceptable for v1; Pass 6 toast can
  show full text on hover/expand.
- **Resize churn.** A persistent banner means a transient JMAP error
  during a reconnect lingers visually. Accepted: the alternative is
  auto-dismiss, which we rejected as flicker-prone. Pass 6 will add
  explicit clearing.

## Pass-end checklist

Standard `poplar-pass` ritual: simplify, ADRs (banner ownership,
ErrorMsg type, spinner helper), invariants update, archive plan +
spec, `make check`, commit, push, install.
