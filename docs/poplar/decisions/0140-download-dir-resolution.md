---
title: Download directory resolution
status: accepted
date: 2026-05-04
---

## Context

`saveAttachmentCmd` needs a target directory. Three reasonable
sources: explicit config, `$XDG_DOWNLOAD_DIR`, and `~/Downloads`.
A per-save target prompt was considered and rejected — the picker
already has a save key; adding a path prompt would split the
muscle-memory between two interaction patterns.

## Decision

`UIConfig.DownloadDir` resolves at `LoadUI` time:

1. Explicit `[ui] download_dir = "..."` (tilde-expanded via
   `config.ExpandHome`).
2. Else `$XDG_DOWNLOAD_DIR`.
3. Else `<UserHomeDir>/Downloads`.
4. Else empty — `saveAttachmentCmd` returns an `ErrorMsg`.

`saveAttachmentCmd` calls `os.MkdirAll(dir, 0o700)` before writing.
Collisions resolve with `-N` suffixes before the extension, capped
at 999.

## Consequences

- One TOML key, one resolution rule. Consistent with the existing
  config style (no per-action paths).
- `~/Downloads` is the de-facto Linux convention; XDG override is
  there for users who relocate it. macOS and Windows fall through
  the same path via `os.UserHomeDir`.
- Empty `DownloadDir` (no HOME, no XDG, no explicit) is a valid
  config; saves error visibly via the banner. No silent fallback
  to `/tmp` or cwd.
