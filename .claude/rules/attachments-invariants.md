---
description: Binding facts for poplar's attachment handling — backend, viewer chip row, picker, save target
paths:
  - "internal/ui/reader/attachpicker*.go"
  - "internal/ui/compose/attachpicker*.go"
  - "internal/ui/reader/viewer*.go"
  - "internal/mail/attachment*.go"
---

# Poplar Attachment Invariants

Binding facts for the attachment surface that spans
`internal/mail/` (wire types), `internal/cache/` (storage and
eviction), and `internal/ui/` (viewer chip row + picker overlay).
Loaded when editing the picker, viewer, or `internal/mail/`
attachment types. The decision index in
`docs/poplar/invariants.md` maps each fact back to its ADR(s).

Cache-side storage facts live in `.claude/rules/cache-invariants.md`
(schema v5 `attachments` table, lazy bytes, `MaxAttachmentSize`
backstop).

## Backend wire shape

- `mail.Attachment` carries `PartID`, `Filename`, `MIMEType` (lowercased
  `type/subtype`, params dropped), `Size`, `ContentID` (unwrapped from
  `<>`), and `Disposition`. PartID is protocol-native (JMAP `partId`,
  IMAP section `"2"` / `"2.1"`) and opaque to consumers.
  `mail.Backend.Attachments(uid)` returns metadata for non-body parts;
  `FetchAttachment(uid, partID)` returns decoded bytes. JMAP impl
  issues `Email/get` for `bodyStructure`, walks the part tree, and
  caches partID→blobID per Backend instance for follow-up downloads
  via `cli.Download`. IMAP impl issues `UID FETCH BODYSTRUCTURE` then
  `UID FETCH BODY[<part>]`. Top-level `text/plain` and `text/html`
  parts are dropped (displayable body, not attachments); the
  classification rule lives in `mail.ClassifyDisposition` and trusts
  `Content-Disposition` first, falls back to `ContentID != ""` →
  inline, defaults to attachment.

## Save target

- `[ui] download_dir` is the attachment-save target. Resolved at
  `LoadUI` time: explicit value (tilde-expanded via
  `config.ExpandHome`) wins, else `$XDG_DOWNLOAD_DIR`, else
  `<UserHomeDir>/Downloads`. Empty resolution surfaces as an
  `ErrorMsg` on save (no silent fallback to `/tmp` or cwd).
  `saveAttachmentCmd` `MkdirAll(0o700)` and resolves collisions via
  `-N` suffix before the extension, capped at 999.
