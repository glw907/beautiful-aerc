---
title: Inherited global WorkerMessages channel
status: superseded by 0075
date: 2026-04-09  # Pass 2
---

## Context

Refactoring to per-worker channels requires changes
across the entire forked worker codebase. Single-account use (Passes
2-10) is unaffected. Will need to be addressed for multi-account
support in Pass 11.

## Decision

Keep aerc's package-level `types.WorkerMessages` channel
for now. Known limitation: multiple adapters would race on this
channel.

## Consequences

**Superseded 2026-04-25 by ADR-0075.** The global channel
disappears with the aerc fork in Pass 3. The Pass 11 multi-
account problem becomes trivial: each account holds its own
`mail.Backend` instance with no shared state.
