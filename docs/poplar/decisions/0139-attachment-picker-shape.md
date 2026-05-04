---
title: Attachment picker overlay shape
status: accepted
date: 2026-05-04
---

## Context

The chip row surfaces what's attached but doesn't dispatch. The
viewer needs a key that opens an actionable list. Existing terminal
clients use a wide range — `v` (mutt), `S` (alpine), `a`. Poplar's
existing model (link picker) wires `Tab` → digit-keyed launcher,
which sets a precedent.

## Decision

`@` from the viewer opens an `AttachPicker` overlay. The picker
mirrors `LinkPicker`: `j/k` cursor, digits `1`–`9` jump-and-act,
`Esc`/`q`/`@` close. New keys not present in the link picker:

- `Enter` / `o` — open via `xdg-open` on a tempfile written under
  `os.TempDir()` as `poplar-<uid>-<filename>`.
- `s` — save to `[ui] download_dir` with collision suffixing
  (`name-1.ext`, `name-2.ext`, capped at 999).

`@` closes the picker as well as opens it (toggle; same as `Tab`
on the link picker).

The picker is App-owned. Viewer emits `OpenAttachPickerMsg{UID,
Items}`; App opens the overlay, dispatches `OpenAttachmentMsg` /
`SaveAttachmentMsg`, fires the corresponding Cmd. Successful saves
emit `attachmentSavedMsg{path}` which sets a one-shot
`opSaveAttachment` toast (suppresses `[u undo]`; the file is on
disk).

The overlay slot in the cascade is between link and move:
confirm > conflict > outbox > help > link > attach > move.

## Consequences

- One more single-key viewer binding. `@` is unused elsewhere and
  has visual mnemonic ("at" → attachments). Consistent with link
  picker discoverability.
- Reuses link picker helpers (`visibleLinkRows`,
  `clampScrollOffset`, `centerOverlay`); attachpicker.go is ~200
  lines of mostly mechanical mirror.
- Open uses the existing `URLOpener` seam; a future test fake
  swaps `xdg-open` without forking the open path.
