---
title: Cache II policy — lazy population, single max-size backstop
status: accepted
date: 2026-05-03
---

## Context

Pass 8.4 originally specced LRU eviction with `last_accessed` tracking,
age-based and per-folder caps, and a background sweeper. Cache II's actual
landed design diverged from that spec after brainstorm and planning: the
additional complexity (LRU index maintenance on every read, two-axis eviction,
a separate goroutine) was not justified by the use cases modeled.

## Decision

The body cache uses lazy population: a cache miss fetches from the backend and
stores; a cache hit returns stored bytes directly. The single size control is
`[cache] max-size` in `config.toml` (default 2 GB). When an insert would push
total stored size over the cap, `storeBody` first runs a size short-circuit
(cheap `SELECT SUM(size)`), then evicts the oldest messages by `messages.sent_at`
inline — no background goroutine, no per-folder caps. The `last_accessed` column
and `bodies_lru` index from the v3 schema are dropped in schema v4. Age-based
pruning is available via `poplar cache evict --older-than DUR` for users who want
it; the size backstop is the only automatic enforcement.

## Consequences

Simpler code: no LRU bookkeeping on every read, no sweeper lifetime management.
The backstop runs only when needed (size short-circuit keeps it cheap in the common
case). Heavy long-lived caches use the manual CLI for age-based pruning. Async
prefetch deferred to Cache III.
