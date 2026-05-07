---
title: SPDX headers removed from .go files; LICENSE is canonical
status: accepted
date: 2026-05-07
---

## Context

Every `.go` file under `cmd/` and `internal/` carries a one-line
header:

```go
// SPDX-License-Identifier: MIT
```

The audit counted 247 such lines across 154 non-test files. The
header conveys one fact already conveyed by `LICENSE` at the repo
root: the project is MIT-licensed.

Three options were considered:

1. **Keep the header on every file.** Some Linux-distribution
   toolchains and SBOM scanners read SPDX-License-Identifier per
   file. This is the SPDX-WG recommendation for projects that
   ship with mixed licenses or vendor third-party code at file
   granularity.
2. **Keep the header only on vendored files.** Restricts the
   marker to the case where it actually adds information — a
   file that came from somewhere else under a different (or
   same) license.
3. **Drop the header entirely; rely on `LICENSE`.** Three peer
   TUI applications surveyed (charmbracelet/glow, dlvhdr/gh-dash,
   derailed/k9s) carry no SPDX headers. Each places a single
   `LICENSE` file at the repo root and stops there.

Poplar is a single-binary application with one license. Vendored
files (`internal/ui/uicore/overlay.go`,
`internal/mailauth/xoauth2.go`,
`internal/mailauth/keepalive/*.go`) already carry a richer
provenance comment naming source repo, commit, and license — more
informative than SPDX, and orthogonal to it.

## Decision

Adopt Option 3.

- Strip `// SPDX-License-Identifier: MIT` from every `.go` file
  under `cmd/` and `internal/`. Strip the immediately-following
  blank line in the same edit so the file's first content line
  is the next declaration (typically `package` or, on vendored
  files, the provenance block).
- Vendored files keep their existing provenance comment block.
  The block names source repo + commit + license, which is
  strictly more useful than the SPDX line for downstream
  consumers tracking origin.
- `LICENSE` at the repo root remains canonical and unchanged.
- `scripts/voice-check.sh` gains a T41 scan that fails on any
  re-introduction of `^// SPDX-License-Identifier:` under
  `cmd/` or `internal/`. The scan installs in the same commit
  that completes the sweep, so the gate stays green from the
  start.

## Consequences

- 247 lines (header + blank line × ~154 files) removed from the
  source tree. Pre-beta posture (ADR-0105) makes the churn a
  free move.
- Per-file license metadata for SBOM consumers comes from the
  repository-level `LICENSE` plus the `go.mod` module path. No
  poplar consumer is known to require per-file SPDX.
- Future regressions trip `make check` immediately via the new
  T41 grep.
- Vendored provenance comments survive — they were the only
  files carrying file-level license information that wasn't a
  pure restatement of the repo license, and SPDX was redundant
  with them.
- If a future change introduces dual-licensed code or vendored
  code under a different license, the provenance comment
  pattern (source + commit + license) extends naturally. SPDX
  would be reintroduced only if a downstream consumer required
  it; revisit then.
