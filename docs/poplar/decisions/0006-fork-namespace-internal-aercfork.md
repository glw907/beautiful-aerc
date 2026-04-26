---
title: Fork namespace: internal/aercfork/
status: superseded by 0075
date: 2026-04-09  # Pass 1
---

## Context

Makes the fork boundary visible (ours vs aerc's),
simplifies cherry-picks (clear mapping to aerc source tree), and
avoids name collisions with existing `internal/` packages like
`internal/jmap/` (fastmail-cli's JMAP client).

## Decision

All forked aerc code lives under `internal/aercfork/`
rather than directly in `internal/`.

## Consequences

**Update 2026-04-12 (pivot):** `internal/aercfork/` was renamed to
`internal/mailworker/` as part of the poplar pivot. The rationale
for keeping the fork in a namespaced subtree is unchanged — only
the subtree name changed so the project's own mail-related
packages (`internal/mail`, `internal/mailjmap`) read as a coherent
family. Provenance is preserved in top-of-file comments on every
`.go` file plus the `internal/mailworker/README.md` history note.

**Superseded 2026-04-25 by ADR-0075.** `internal/mailworker/` is
deleted in Pass 3 along with the aerc fork itself. Vendored
snippets (`auth/xoauth2.go`, `keepalive/`, `xgmext/`) move under
poplar-owned packages with provenance comments preserved.
