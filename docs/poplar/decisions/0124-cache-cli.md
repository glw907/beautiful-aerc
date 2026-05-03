---
title: poplar cache CLI surface
status: accepted
date: 2026-05-03
---

## Context

Users need to inspect cache state and perform manual pruning without launching
the TUI. The size backstop handles automatic enforcement, but age-based pruning
and diagnostic inspection require an explicit CLI surface.

## Decision

`cmd/poplar/cache.go` registers a `poplar cache` parent command with three
subcommands. `poplar cache stats` prints per-account diagnostics (header count,
body count + total size, outbox row breakdown by status, on-disk DB file size);
it opens SQLite directly without connecting to the backend (offline-safe).
`poplar cache evict --older-than DUR [--account NAME]` accepts duration strings
parsed by `time.ParseDuration` extended with `d` (days) and `w` (weeks) suffixes,
calls `(*cache.Account).EvictByAge(ctx, cutoff)`, and prints evicted counts.
`poplar cache vacuum [--account NAME]` runs SQLite `VACUUM` via a single-connection
bypass to reclaim free pages after large evictions. All three subcommands resolve
account paths through `cache.Slugify`, `cache.DBPath`, and `cache.OpenDB` —
the canonical exported helpers so path and DSN logic live in one place.

## Consequences

Cache management is fully scriptable. `poplar cache stats` is the primary debug
entry point during incident triage. Body-cache size is bounded by two surfaces:
automatic (the size backstop in `storeBody`) and manual (`poplar cache evict`).
