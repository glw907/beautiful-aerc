---
title: Split aerc lib/ into focused packages
status: superseded by 0075
date: 2026-04-09  # Pass 1
---

## Context

aerc's `lib/` is a grab-bag of unrelated utilities.
Splitting by concern makes dependencies explicit and avoids pulling
in UI-specific code (messageview, dirstore, etc.) that lives in the
same package.

## Decision

aerc's monolithic `lib/` package was split into focused
packages: `auth/` (OAuth), `keepalive/` (TCP), `xdg/` (paths),
`log/` (logging), `parse/` (headers).

## Consequences

**Superseded 2026-04-25 by ADR-0075.** The split packages
disappear with the fork in Pass 3. The two pieces poplar still
needs (`auth/xoauth2.go`, `keepalive/`) move to
`internal/mailauth/` as small vendored snippets with provenance
comments preserved.
