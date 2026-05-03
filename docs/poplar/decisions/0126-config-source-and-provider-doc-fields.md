---
title: Drop config.Source enum and Provider documentation fields
status: accepted
date: 2026-05-03
---

## Context

Two unused-but-documented surfaces in `internal/config`:

- `Source` (typed enum: `SourceFlag`/`SourceEnv`/`SourceDefault`) was
  the second return of `Resolve`, intended to let callers branch on
  *how* the path was chosen. Every cmd-layer caller discarded it with
  `_`. Only `Load` used it internally to decide first-run template
  behavior, and that decision can be expressed as `flagPath != ""`.
- `Provider.Name` duplicated the map key. `Provider.AuthHint` and
  `Provider.HelpURL` were intended as user-facing onboarding hints, but
  no CLI command, backend, or UI surface ever read them — only test
  assertions did.

## Decision

- Collapse `Source` away. `Resolve(flagPath string) (string, error)`.
  `Load`'s first-run branch tests `flagPath != ""` directly.
- Drop `Provider.Name` (redundant with map key), `Provider.AuthHint`
  and `Provider.HelpURL`. The corresponding test assertions are
  removed; the preset table in `providers.go` shrinks accordingly.

Cascading edits in `cmd/poplar/`: `config_cmd.go`,
`config_discover_folders.go`, and `diagnose.go` drop the `_` from
their `Resolve` calls.

Also dropped from `AccountConfig`: `Headers`, `HeadersExclude`,
`CheckMail`, and `Aliases`. None had a TOML decode site or a
consumer.

## Consequences

The public `config` API is what its consumers use, nothing more.
Onboarding hints (auth method, help URL) when they're needed will land
with their consumer (e.g., a first-run wizard) — at which point the
fields earn their place because something reads them.
