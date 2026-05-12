# Plan — Pass 33: mouse support (reader)

Spec: `docs/superpowers/specs/2026-05-11-mouse-reader-design.md`.

## Tasks

### 1. content: add `ribbonRows` return to RenderBodyWithFootnotes

Add a third return `ribbonRows []int` to
`internal/content/render_footnote.go`. Walk the ribbon emit loop
(after the rule line) and append each ribbon row's 0-based line
index into the rendered string. All existing call sites
(`internal/ui/reader/model.go:layout`, tests) update to receive
the new return. Add a focused test in `render_footnote_test.go`
asserting `ribbonRows[i]` lines, when stripped of ANSI, match
`^\[\d+\]: `.

### 2. reader: define hitZone + hits table

In `internal/ui/reader/model.go`, add unexported `hitKind` enum
(`hitChip`, `hitFootnote`), `hitZone` struct (fields per spec),
and `hits []hitZone` field on `Model`. Reset in `Open`/`Close`.

### 3. reader: populate chip hits during renderChipRow

Refactor `renderChipRow` to a method that returns
`(rendered string, rows int, hits []hitZone)`. Walk attachments
on each wrapped line; track the cell offset using
`v.measurer.Width` per chip and per separator. Append a
`hitZone{kind: hitChip, index: i, rowStart: lineIdx, rowEnd:
lineIdx+1, colStart, colEnd}` per chip. `layout()` stores them
on `v.hits`.

### 4. reader: populate footnote hits from ribbonRows

In `layout()`, after `RenderBodyWithFootnotes` returns, iterate
`ribbonRows` and append `hitZone{kind: hitFootnote, index: i,
rowStart: row, rowEnd: row+1, colStart: 0, colEnd: contentWidth}`
to `v.hits`.

### 5. reader: mouse Update handler

Extend `reader.Model.Update` to handle `tea.MouseMsg`. Unexported
helpers: `mouseInChipRow(m) bool`, `mouseInBody(m) bool`,
`bodyOriginY() int`, `hitChip(m) (hitZone, bool)`,
`hitFootnote(bodyRow int) (hitZone, bool)`. Wheel inside body →
forward to `v.viewport`. Click on chip → emit
`OpenAttachmentMsg{UID, Item: v.attachments[i]}` (verify exact
shape; today the picker emits this — reuse it). Click on
footnote ribbon → return `openURLCmd(v.links[i])` (the existing
Cmd in `internal/ui/cmds.go`). Click in body but no hit → forward
to viewport (so terminal-side selection still works when nothing
is clickable).

### 6. App: route MouseMsg

In `internal/ui/app.go`, add a `case tea.MouseMsg:` arm before
the existing key dispatch. New method `updateMouse(msg tea.MouseMsg)
(App, tea.Cmd)`:

- If any overlay open (cascade: confirm, conflict, outbox, help,
  linkPicker, attachPicker, movePicker, form/popover, compose
  AttachPicker / SchedulePicker, reschedule): return `m, nil`.
- Else if `m.viewerOpen && m.acct.Viewer().Phase() == reader.PhaseReady`,
  forward via a new `account.Model.UpdateViewerMouse` method (or
  reuse `UpdateViewer` if mouse fits the existing signature).
- Else return `m, nil`.

### 7. App: declarative MouseMode

In `internal/ui/app_view.go`, set `v.MouseMode = tea.MouseModeCellMotion`
in the `view` helper, every frame. No conditional.

### 8. Tests

- `reader/model_test.go`: table-driven `hitChip` / `hitFootnote`
  cases at width 80 and width 120, with and without SPUA-A
  glyphs in attachment names (`v.icons.Attachment` varies).
- `app_test.go`: dispatch test confirming a `MouseClickMsg` on
  a ribbon row routes to `openURLCmd`. Overlay-open absorb test.
- `content/render_footnote_test.go`: ribbon row index assertion.

### 9. Live tmux verification

80×24 and 120×40 captures: open a message with ≥2 attachments
and ≥3 harvested URLs. Verify wheel scrolls, chip click opens
attachment, ribbon click opens URL. Save captures under
`docs/poplar/captures/pass-33-*.txt`.

### 10. ADR-0218 + invariants update

Write `docs/poplar/decisions/0218-mouse-reader.md` codifying:
- `MouseMode = MouseModeCellMotion` declared in `App.view`.
- Mouse dispatch cascade (App → reader; overlays absorb).
- Footnote ribbon row is the click target, not inline `[^N]`.
- `content.RenderBodyWithFootnotes` returns `ribbonRows`.

Update `docs/poplar/invariants.md` Architecture / Elm
section with the MouseMode + cascade facts, and the Viewer
section with the hit-test invariants. Update
`docs/poplar/decisions/INDEX.md`.

### 11. STATUS.md

Mark Pass 33 done. Write the Pass 34 starter prompt (sidebar +
cross-pane mouse) per the format in the `poplar-pass` skill.

### 12. Pass-end consolidation

`make check`, archive plan + spec (`git mv`), commit + push +
`make install`.

## Notes

- `bubbles/v2/viewport` already handles `tea.MouseWheelMsg`
  natively — `Update` switches on it and adjusts `YOffset`. We
  don't reimplement scroll math; we just route the event.
- Click coordinates from bubbletea are 0-based and in the **full
  terminal frame**, not pane-local. Reader must subtract the
  panel + invite + chip-row heights and the global frame origin
  (topLine = row 0 → content starts at row 1). The account
  region also has its own left offset (sidebar width). Compute
  via `App.acct.RightPaneOrigin()` or inline equivalent.
- `tea.MouseMotionMsg` and `tea.MouseReleaseMsg` are ignored —
  not dropped, just no-op cases in reader's switch. Safer than
  filtering at the App level in case Pass 34 needs them.
- No keybinding hint additions: mouse is invisible in help and
  footer per ADR-0072 (wired/unwired applies to keys only).
