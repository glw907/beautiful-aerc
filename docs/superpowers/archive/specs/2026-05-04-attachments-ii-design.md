# Pass 8.7 — Attachments II (viewer surface)

**Status:** design accepted 2026-05-04
**Issue:** #24 (v1 blocker)
**Inherits from:** Pass 8.6 backend / cache (ADR-0135–0137)

## Goal

Surface attachment metadata in the viewer, give the user a
keyboard-driven way to open or save individual attachments, and add
the chip row layout that makes their presence visible without a
modal interaction.

## Scope

In:

- `internal/ui/viewer.go` — chip row region between header panel
  and body; attachment state on the `Viewer` model.
- `internal/ui/attachpicker.go` (new) — modal picker, mirrors
  `LinkPicker`. App-owned, viewer-context-only.
- `internal/ui/cmds.go` — `attachmentsLoadedMsg` (metadata fetch),
  `OpenAttachmentMsg` / `SaveAttachmentMsg` plumbing,
  `attachmentsFetchCmd`, `openAttachmentCmd`, `saveAttachmentCmd`.
- `internal/ui/app.go` — overlay cascade entry for the attachment
  picker, key dispatch for `@`, `OpenAttachPickerMsg` handling,
  toast routing for save outcomes.
- `internal/ui/account_tab.go` — batch `attachmentsFetchCmd`
  alongside `bodyFetchCmd` in the viewer-open flow; route
  `attachmentsLoadedMsg` into the viewer.
- `internal/ui/icons.go` — add `Attachment` field to `IconSet`
  (Simple + Fancy variants), wire into chip rendering.
- `internal/config/ui.go` — add `DownloadDir` field +
  `[ui] download_dir` TOML key with `$XDG_DOWNLOAD_DIR` →
  `$HOME/Downloads` default resolution.
- `cmd/poplar/root.go` — thread `DownloadDir` into `ui.NewApp`.
- `docs/poplar/keybindings.md` — add `@` (attachment picker) and
  the picker's internal `o`/`s`/`Enter` table.
- `docs/poplar/wireframes.md` — viewer + attachment-picker frames.
- `.claude/rules/ui-invariants.md` — Viewer / Overlays additions.
- `docs/poplar/invariants.md` — Architecture (Viewer state) +
  decision-index entry.
- ADRs documenting the picker key, download-dir resolution, chip
  region placement.

Out:

- HTML body inline-image rendering / `cid:` rewriting. Poplar's
  body renderer is plain-text only; `cid:` references inside HTML
  are stripped during the HTML→plain pass in
  `internal/filter/html.go` and never reach the viewer. Attachment
  metadata stays cached for a future image-preview surface;
  Pass 8.7 doesn't read `ContentID`.
- Compose-side attach (Pass 9.5).
- Per-row attachment indicator in the message list. The chip
  row in the viewer carries the surface for v1; deferring the
  list-row indicator to a later pass keeps Pass 8.7 focused on
  the read-side flow. Log separately when it lands.

## Settled decisions

1. **Picker key: `@`** — single non-modifier key, mnemonic for
   "attachment", currently unused. Symmetric with `Tab` for the
   link picker; both invoke a viewer-context-only modal.
2. **Open via `xdg-open` on a temp file** — same fire-and-forget
   pattern as link launch. Bytes are fetched lazily through
   `cache.FetchAttachment`, written to `os.TempDir()` at
   `poplar-<uid>-<filename>`, handed to `xdg-open`. No tempfile
   cleanup in v1; OS handles it.
3. **Save defaults to `[ui] download_dir`, no prompt** — poplar
   has no `:` mode and no textinput surface in v1. Default
   resolution: `$XDG_DOWNLOAD_DIR` if set, otherwise
   `$HOME/Downloads`. Filename collisions resolve with
   `name-1.ext`, `name-2.ext`, ... Toast surfaces the resulting
   path. Failures route through the standard `ErrorMsg` banner.
4. **Chip row hidden when empty** — render zero rows when the
   message has no attachments. Header panel sits flush against
   body. Matches the "no link footnotes → no rule" pattern.
5. **Eager metadata fetch** — `attachmentsFetchCmd` batched in
   the same `tea.Batch` as `bodyFetchCmd` when the viewer opens.
   Single visible "loading…" phase. The chip row appears with
   the body. Cache hits return instantly; misses block briefly
   before the body is ready anyway.
6. **No CID rewriting** — see scope-out above.

## Architecture

### Viewer state

`Viewer` gains four fields:

```go
attachments []mail.Attachment // metadata only
attachReady bool              // attachmentsLoadedMsg landed
chipRow     string            // pre-rendered chip block
chipHeight  int               // rows the chip block occupies
```

`Open(msg)` resets all four. `attachReady` flips when the
`attachmentsLoadedMsg` for the current UID arrives. `SetSize`
re-renders `chipRow` if `attachReady` is set. `Phase()` continues
to gate viewer readiness on the body alone — chips render once
their fetch lands, independently of body, but the spinner phase
covers the time both are in flight.

`SetAttachments([]mail.Attachment) Viewer` is the new accessor —
analogous to `SetBody`. Idempotent; stale UIDs filtered at the
`AccountTab` boundary like `bodyLoadedMsg`.

### Chip row region

A new region between `v.panel` (headers) and the body viewport.
Layout in `Viewer.layout()`:

1. Render headers as today.
2. If `attachReady && len(attachments) > 0`: render chip row to
   `contentWidth`, store in `v.chipRow`, set `v.chipHeight =
   lipgloss.Height(chipRow)`.
3. Body viewport height = `v.height - lipgloss.Height(v.panel) -
   v.chipHeight`.

`View()` joins `v.panel`, `v.chipRow` (if non-empty), body. The
body's per-line right-pad logic continues to apply; chip row uses
the same `clipPaneBg`-style fill so it matches the body's surface
background.

### Chip rendering

Each chip is `<icon> <N>. <name> (<size>)` where:

- `<icon>`: `IconSet.Attachment` glyph (Simple `📎` or Fancy
  Nerd Font equivalent — concrete codepoints picked during
  implementation).
- `<N>`: 1-based index, max 9 (`1`–`9` are addressable from the
  picker; chips beyond 9 render but require `j/k` cursor in the
  picker).
- `<name>`: filename, truncated mid-name with `…` if a single
  chip exceeds line width.
- `<size>`: `humanizeBytes` (e.g. `2.4 KB`, `1.1 MB`).

Chips separated by two spaces. Wrapping: greedy fill — accumulate
chips on a line until the next would overflow, then start a new
line. Lipgloss `Width` math (icon-bearing strings → `displayCells`).

### Attachment picker (`AttachPicker`)

Modal overlay, mirrors `LinkPicker`'s structure. App-owned. Lives
in `internal/ui/attachpicker.go`. Uses `ModalShell` for box chrome
(matches the other modals — ADR-0129).

State:

```go
type AttachPicker struct {
    open        bool
    items       []mail.Attachment
    cursor      int
    styles      Styles
    shell       ModalShell
    keys        AttachPickerKeys
    width       int
    height      int
    cache       *attachPickerCache // ADR-0130 escape hatch
}
```

Keys (single-key, modifier-free, no multi-key):

| Key | Action |
|-----|--------|
| `j` / `k` | Cursor down / up |
| `1`–`9`   | Default action (open) on Nth item |
| `Enter`   | Default action (open) on cursor |
| `o`       | Open via `xdg-open` |
| `s`       | Save to download dir |
| `Esc`, `q`, `@` | Close picker |

`@` toggles the picker (open → close). `q` is swallowed inside
the picker (consistent with help / link / move pickers — modals
are views, not states to escape).

### Cmds

```go
// attachmentsFetchCmd resolves metadata via cache. Stale-UID
// drops are handled at the AccountTab boundary on the resulting
// attachmentsLoadedMsg.
func attachmentsFetchCmd(c *cache.Account, uid mail.UID) tea.Cmd

// attachmentsLoadedMsg carries the resolved metadata.
type attachmentsLoadedMsg struct {
    uid mail.UID
    items []mail.Attachment
    err   error // routed to ErrorMsg if non-nil
}

// openAttachmentCmd fetches bytes via cache.FetchAttachment,
// writes to os.TempDir(), invokes xdg-open. Fire-and-forget.
func openAttachmentCmd(c *cache.Account, uid mail.UID, att mail.Attachment) tea.Cmd

// saveAttachmentCmd fetches bytes, writes to downloadDir with
// collision resolution, returns toastMsg with the final path.
func saveAttachmentCmd(c *cache.Account, downloadDir string, uid mail.UID, att mail.Attachment) tea.Cmd
```

Failures from any of the three return `ErrorMsg{Op: "<verb>", Err: ...}`
with verbs `"fetch attachments"`, `"open attachment"`, `"save attachment"`.

### Save semantics

`saveAttachmentCmd`:

1. Resolves `downloadDir`: app-owned `string` from config; expand
   `~`; ensure exists (`os.MkdirAll` with 0700).
2. Sanitizes filename: replace path separators with `_`; if empty
   (some attachments have no name), use `attachment-<partID>`.
3. Collision resolution: if `name.ext` exists, try `name-1.ext`,
   `name-2.ext`, capped at 999.
4. Fetches bytes via `cache.Account.FetchAttachment`.
5. `os.WriteFile(path, body, 0600)`.
6. Emits a toast `Saved to <path>` (no undo hint, no inverse —
   write is a side effect).

### Open semantics

`openAttachmentCmd`:

1. Resolves filename like save (sanitize, fall back to
   `attachment-<partID>`).
2. Builds path `os.TempDir() + "/poplar-<uid>-<filename>"`.
3. Fetches bytes; writes file with 0600.
4. Spawns `xdg-open <path>`. Fire-and-forget; no result wait.

### Config

```go
type UIConfig struct {
    // ...
    DownloadDir string
}
```

`[ui] download_dir = "/path/to/downloads"`. Default resolution
order:

1. Explicit `[ui] download_dir` from `config.toml`.
2. `$XDG_DOWNLOAD_DIR` env var.
3. `$HOME/Downloads`.

`DefaultUIConfig()` resolves at config-load time so the value
threaded into the App is concrete, not lazy.

### Wiring

`AccountTab.openMessage` (the function that runs on `Enter` /
`n` / `N`) currently builds `tea.Batch(bodyFetchCmd(...),
markReadCmd(...), v.SpinnerTick())`. It gains
`attachmentsFetchCmd(...)` in the same batch.

`AccountTab.Update` gains a case for `attachmentsLoadedMsg`:

```go
case attachmentsLoadedMsg:
    if msg.uid != m.viewer.CurrentUID() {
        return m, nil // stale
    }
    if msg.err != nil {
        return m, errorCmd("fetch attachments", msg.err)
    }
    m.viewer = m.viewer.SetAttachments(msg.items)
    return m, nil
```

`Viewer.handleKey` gains a case for `keys.OpenAttachPicker` (`@`)
that emits `OpenAttachPickerMsg{Items: v.attachments, UID: v.msg.UID}`.
Inert if `len(v.attachments) == 0`.

`App.Update` adds:

- `OpenAttachPickerMsg` — open the picker, store the UID for
  later save/open dispatch.
- `AttachPickerClosedMsg` — close the picker.
- `OpenAttachmentMsg{UID, Att}` — dispatch `openAttachmentCmd`.
- `SaveAttachmentMsg{UID, Att}` — dispatch `saveAttachmentCmd`.
- Overlay cascade addition: attachment picker sits adjacent to
  link picker in priority. Confirm > conflict > outbox > help >
  link picker > attach picker > move picker. (Attach picker
  below link picker because both are viewer-only and only one
  opens at a time, but the ordering is documented for safety.)

### Tests

- `viewer_test.go` — chip row renders, hidden when empty, body
  height shrinks correctly with chips present, `SetAttachments`
  idempotent, layout responds to `SetSize`.
- `attachpicker_test.go` — open/close, cursor nav, key
  dispatch (1–9, Enter, o, s), inert when empty, render at
  120×40 + minimum width.
- `cmds_test.go` — `attachmentsFetchCmd` happy path + cache
  miss + error; `saveAttachmentCmd` collision resolution;
  `openAttachmentCmd` filename sanitization. Real `os.TempDir()`
  + temp `download_dir` (`t.TempDir()`).
- `app_test.go` — overlay cascade priority, key dispatch
  routing, toast on save.

### Live UI verification

`tmux` workflow per `.claude/docs/tmux-testing.md`:

- 120×40 + 80×24 capture of viewer with 0 / 1 / 5 / 12
  attachments.
- Picker render at both sizes.
- Save → toast → confirm file appears at expected path.
- Open → file appears in `os.TempDir`, `xdg-open` fires (visual
  confirmation; mailcap routing is the OS's job).

## ADRs to write

1. **Attachment chip row** — placement between header panel and
   body; layout owned by `Viewer.layout`; hidden when empty.
2. **Attachment picker key & shape** — `@` opens the picker;
   `o`/`s`/`Enter` actions; modifier-free single keys.
3. **Download dir resolution** — `$XDG_DOWNLOAD_DIR` →
   `~/Downloads` default; collision suffix `-1`, `-2`, ...; no
   prompt in v1.

## Out-of-scope follow-ups (not blocking 8.7)

- Per-row attachment indicator in `MessageList`.
- HTML inline-image preview / `cid:` resolution.
- Compose-side attach (Pass 9.5 by plan).
- Path picker / textinput-driven save target (post-Catkin, 1.x).
