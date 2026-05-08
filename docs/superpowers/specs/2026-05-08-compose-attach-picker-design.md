# Compose-side attach picker — Pass 9p

**Date:** 2026-05-08
**Issue:** #24 (compose-side attachments, the third leg of the
attachments triplet from passes 8.6 and 8.7).
**Status:** approved.

## Goal

Let the user add and remove attachments while composing a message,
end-to-end through the existing outbound stack. The backend
`AssembleMIME` path already supports `multipart/mixed` from
`Draft.Attachments []string`; what is missing is the UI surface that
populates and edits that slice.

## Scope

- New `compose.AttachPicker` sub-model in
  `internal/ui/compose/attachpicker.go`: a multi-select TUI file
  browser overlay built on `uicore.ModalShell`.
- A focusable "Attach:" row in compose's header strip showing
  current attachments with cursor-driven removal.
- A `^O attach` footer hint in compose, ranked between `^T tidy` and
  `^X send`.
- ADR-0179 covering both the picker decision and the elevation of
  "every `ModalShell` overlay carries a footer hint row" to a
  binding fact in `ui-invariants.md`.

Out of scope: xdg-portal / GUI dialog, drag-and-drop, clipboard
path paste, attachment thumbnails, filename autocomplete in a
non-overlay path entry, cross-session persistence of last-browsed
dir, attachment renaming or Content-Disposition overrides.

## Prior art

- `bubbles/filepicker` (Charm, v1.0.0, 522 lines): vim `h/l` back
  and descend, `g/G` first/last, `j/k`+arrows nav, pgup/pgdown,
  hidden-file toggle. Async `readDir` returning `readDirMsg{id,
  entries}` with an integer id per instance to discard stale
  results across rapid descend/ascend. Stack-based nav state —
  push `(cursor, min, max)` on descend, pop on ascend so the
  cursor lands back on the directory you came out of. Single
  select only; no per-row marker rendering hook for multi-select.
- `aerc` ships `:attach <path>` with readline tab completion. No
  overlay; different idiom from poplar.
- Nothing in the bubbletea ecosystem ships a multi-select TUI
  file picker.

Decision: reimplement inside `internal/ui/compose/` (do not embed
`filepicker.Model`) so we control per-row selection-marker
rendering and the multi-select layer. Borrow vocabulary: vim
keys, async readDir + id guard, stack nav.

## Footer-hint contract

Audit found every existing `ModalShell` overlay already carries a
footer hint row: `reader.AttachPicker`, `reader.LinkPicker`,
`movepicker`, `confirm_modal`, `conflict_overlay`,
`outbox_overlay`, `contacts.Popover`, `contacts.Form`. Pass 9p
elevates the pattern from observed convention to binding fact.

`ui-invariants.md` gets a new sentence in the modal cascade
section:

> Every `ModalShell` overlay carries at least one `footerRows`
> line enumerating its active key bindings, padded to `contentW`.
> Yes/no confirms are exempt — the action set (y/n/Esc) is
> implicit — but in practice they include the hint anyway. This
> is the discoverability contract: a user landing in any complex
> modal can read the keys without leaving it.

## Data shapes

```go
// internal/ui/compose/attachpicker.go

type AttachPicker struct {
    shell      uicore.ModalShell
    id         int                   // guards stale readDirMsg
    dir        string                // absolute current directory
    entries    []entry
    cursor     int
    offset     int
    selected   map[string]bool       // absolute paths
    showHidden bool
    stack      []viewState           // ascend restoration
    err        string                // sticky on readDir failure
    styles     Styles
    icons      uicore.IconSet
    keys       attachPickerKeys
}

type entry     struct { name, path string; isDir bool; size int64 }
type viewState struct { cursor, offset int }

type attachPickerKeys struct {
    Up, Down               key.Binding
    PgUp, PgDown           key.Binding
    GoTop, GoBottom        key.Binding
    Open                   key.Binding   // l, right, enter
    Back                   key.Binding   // h, left, backspace
    Toggle                 key.Binding   // space
    Accept                 key.Binding   // a
    ToggleHidden           key.Binding   // .
    Close                  key.Binding   // esc
}
```

Compose model gains:

```go
attach        AttachPicker
attachCursor  int                       // index into Draft.Attachments
attachLastDir string                    // session-scoped browse memory
```

The `focus` enum gains `focusAttach`, slotted between
`focusSubject` and `focusBody`. Tab cycle skips it when
`len(Draft.Attachments) == 0`.

## Picker behavior

| Key                            | Action                                              |
|--------------------------------|-----------------------------------------------------|
| `j` / `k`, `↓` / `↑`           | Cursor down / up                                    |
| `pgdn` / `pgup`, `J` / `K`     | Page down / up                                      |
| `g` / `G`                      | First / last entry                                  |
| `l` / `→` / `Enter` (on dir)   | Descend (push view state, `readDirCmd`)             |
| `l` / `→` (on file)            | Toggle selection on this file                       |
| `Enter` (on file, 0 selected)  | Single-attach shortcut: emit `AttachAccepted{[p]}`  |
| `h` / `←` / `Backspace`        | Ascend (pop view state, `readDirCmd`)               |
| `Space`                        | Toggle selection on focused file                    |
| `a`                            | Accept: emit `AttachAccepted{paths}` (if ≥1)        |
| `.`                            | Toggle hidden-file visibility                       |
| `Esc`                          | Emit `AttachCancelled{}`, close                     |

Listing sort: directories first (case-insensitive), then files
(case-insensitive). Hidden files (leading `.`) excluded unless
`showHidden`.

`readDir` is a `tea.Cmd` returning `readDirMsg{id, entries, err}`.
The picker discards messages whose id does not match its current
id, set in `Open` and bumped on every descend/ascend. This avoids
displaying stale entries when the user descends quickly.

Stack: `pushView` on descend captures `(cursor, offset)`; `popView`
on ascend restores them so the cursor lands on the directory just
exited. Ascending past the start dir keeps going (does not trap).

`Open(startDir) (AttachPicker, tea.Cmd)`:
- bumps `id`,
- clears `selected` and `stack`,
- sets `dir = startDir`,
- returns `readDirCmd`.

## Rendering

Picker row format: `{icon} {name}{padding}{size}`. Icons drawn
via `IconSet`: directory icon for dirs, attachment icon for files.
A multi-select marker prefix renders to the left of the icon: `✓ `
when `selected[entry.path]`, two spaces otherwise (so cursor row
and selection state are independent visual channels).

Width math: `uicore.DisplayCells` for any row containing icons;
truncation via `uicore.DisplayTruncate`.

`View` builds:
- title: `"Attach files"` (constant — current dir lives in footer
  row 2 to keep the title bar still).
- bodyRows: rendered entries clipped to picker viewport.
- footerRows: two rows.
  - Row 1, hint line, two variants:
    - 0 selected: `j/k nav · l/Enter open · Space select · a accept · . hidden · Esc cancel`
    - ≥1 selected: `j/k nav · l/Enter open · Space toggle · a accept (N) · Esc cancel`
    - On `err != ""`: replace row 1 with the error message until
      the next nav input; row 2 still shows the path.
  - Row 2, current path. If wider than `contentW`, truncate from
    the left with leading `…/` so the tail stays readable.
- box: `shell.Box("Attach files", bodyRows, footerRows, contentW)`.

## Compose model integration

- `Ctrl+O` (in `bind.go`) from any focus opens the picker:
  `m.attach, cmd = m.attach.Open(dirOrPWD)` where `dirOrPWD` is
  `attachLastDir` if set else `os.Getwd()` (falling back to `$HOME`
  on error).
- `AttachAcceptedMsg{Paths}` in `Update`:
  - dedupe against `Draft.Attachments` (skip duplicates silently),
  - append remaining paths,
  - `attachLastDir = filepath.Dir(paths[0])`,
  - `localDirty = true`,
  - kick the existing autosave tick.
- `AttachCancelledMsg{}`: just close the picker.
- `focusAttach` keys: `←/→` move `attachCursor` within the
  attachment list; `d`, `Backspace`, `Delete` remove
  `Draft.Attachments[attachCursor]`. If the list empties, focus
  snaps back to `focusSubject`. `Ctrl+O` from `focusAttach` opens
  the picker just like any other focus.

## Attach row rendering (in compose `View`)

Visible only when `len(Draft.Attachments) > 0`, between Subject
and Body rows.

Layout: label `"Attach: "` (label-aligned with To/Cc/Bcc/Subject
to keep the colon column straight), followed by chips:

```
Attach:  📎 foo.pdf (124 KB)  📎 bar.png (18 KB)  📎 baz.txt
```

Chip = `📎 {basename} ({humanize.Bytes(size)})`. Size is read
once at chip render via `os.Stat`; on stat failure the size is
omitted (`📎 foo.pdf`) but the chip still renders.

Cursor highlight on focusAttach: focused chip uses the inverse
selection style; others are dim.

Truncation: when the joined chip row exceeds `width - labelWidth`,
render as many leading chips as fit and replace the overflow with
a final `📎 +N` chip showing how many are hidden.

## Footer hint addition

In `internal/ui/footer_hints.go` (or wherever the compose hint
list lives — verify in plan), add `^O attach` between `^T tidy`
and `^X send`. Drop rank: same as `^T tidy` — it is a
precondition action.

## Error handling

- `os.ReadDir` failure surfaces as `readDirMsg{err}` → picker sets
  `err` and renders the message in footer row 1 until the next
  nav input. Body keeps showing the prior listing if any.
- Empty directory: body shows a single dim `(empty)` line.
- Symlinks: follow on descend (matches `bubbles/filepicker`).
- Stat failure on a chip render: omit size; do not crash.
- Path no longer exists at send time: existing `AssembleMIME`
  returns a wrapped error → existing outbox conflict path. No new
  pre-send validation (the file may legitimately appear later).
- No client-side size cap. Server rejection routes through the
  existing outbox conflict UI.

## Testing

`internal/ui/compose/attachpicker_test.go`, table-driven, no
assertion libs:

- readDir async correctness: stale-id messages are dropped.
- nav: j/k bounds; g/G; pgup/pgdn against a tree with > 1 page.
- descend onto a child dir pushes view state; ascend restores
  cursor to the source directory entry.
- ascend past start dir keeps going (no trap).
- hidden toggle includes/excludes dotfiles.
- Space toggles selection; multiple selections accumulate.
- `a` with ≥1 selected emits `AttachAcceptedMsg` with all paths,
  in stable order (selection order preferred; alphabetical
  acceptable if simpler).
- Enter on a file with empty selection emits
  `AttachAcceptedMsg{[file]}` (single-attach shortcut).
- `a` with 0 selected: no-op (no msg).
- Esc emits `AttachCancelledMsg{}`.
- Footer hint variants: count appears when ≥1 selected; error
  text replaces hint when set.

`internal/ui/compose/model_test.go` additions:

- focusAttach skipped on Tab when `len(Attachments) == 0`.
- `AttachAcceptedMsg` appends, dedupes, sets `localDirty`,
  updates `attachLastDir`.
- `Ctrl+O` opens picker with `attachLastDir` (or `$PWD` fallback).
- `d` / `Backspace` / `Delete` on focusAttach removes the focused
  entry; cursor clamps; focus collapses to focusSubject when
  empty.
- Attach row hidden when 0 attachments.

Live tmux verification at 80×24 and 120×40:
- compose with 0 / 1 / 3 attachments,
- picker open on a populated directory,
- picker open at the filesystem root (ascend past start),
- picker on an empty directory,
- picker after a `readDir` error (e.g., `/root` for unprivileged).

## Risks and mitigations

- **`Ctrl+O` keybinding free in textinput / textarea.** Plan
  step 1 is a 5-line probe to confirm it does not conflict with
  bubbles' default keymap on either widget. Fallback: `Ctrl+Y`.
- **SPUA cell-width math.** 📎 (U+1F4CE) is in the SPUA fallback
  range. All width math goes through `uicore.DisplayCells`; row
  composition uses pre-padded children with `strings.Join` per
  ADR-0084. Same discipline as `reader.AttachPicker`.
- **Two decisions in one ADR.** ADR-0177 set precedent for
  tightly-coupled multi-decision ADRs. The footer-hint contract
  is small (one binding fact, no new code) and crystallizes in
  the same pass that introduces the new modal — co-locating the
  ADR keeps history coherent.

## Acceptance

- Compose with no attachments looks identical to today.
- `Ctrl+O` opens the picker; nav is fluent at 120×40 and at the
  bare-minimum 80×24.
- Multi-select `a` attaches all selected files; Enter on a file
  with no selection attaches that one and closes.
- Attach row appears only when populated; cursor + d/Backspace
  removes; focus collapses cleanly.
- `make check` is green; voice scan is clean; live tmux captures
  match the wireframes added by the plan doc.
- ADR-0179 lands; `ui-invariants.md` carries the footer-hint
  binding fact; STATUS.md advances to Pass 9q.
