---
title: Release model — v0.9.0 beta → soak → v1.0.0
status: accepted
date: 2026-05-02
---

## Context

Poplar's pre-1.0 stance (CLAUDE.md) optimizes for clean code over
stability: refactor and rename freely, no compat shims. That works
while the only user is the maintainer. It stops working the moment
poplar is released to the world — strangers running `v0.9.0` need
on-disk data to survive a `v0.9.3` upgrade or they'll churn off.

The project needs an explicit transition from "free refactor" to
"data formats frozen" so the same codebase can be both clean and
trustworthy without an ambiguous middle.

## Decision

Three release phases with explicit rules.

**Pre-beta** (now → end of Pass 10). The current pre-1.0 stance
applies in full: refactor freely, rename freely, no compat shims.
On-disk data the user can't easily regenerate (mail caches, OAuth
refresh tokens) is the only sacred thing. Master is the working
branch.

**Beta soak** (Pass 11 ships → `v1.0.0`). Pass 11 prepares the
freeze — docs sweep, README, release notes, then tags **`v0.9.0`**.
The `0.x` prefix conveys "pre-stable" per modern Go-CLI norms (gh,
lazygit, glow, the Charm libs all did this); no `-beta.N` SemVer
prerelease suffix. From `v0.9.0` onward:

- Master accepts bug fixes only. No new features.
- On-disk data formats are frozen — config schema, cache schema
  (when it lands), credential storage layout. Migrations across
  beta releases must be automatic and lossless.
- Bug fixes ship as `v0.9.1`, `v0.9.2`, … as discovered.
- New features queue on a `1.1` branch (or on plan docs without
  branches if they need design first).
- Soak ends when no new user-reported bugs land for ~2 weeks
  (heuristic, not a hard rule).

**Post-1.0** (`v1.0.0` ships). Standard SemVer rules. Backwards-
compatible additions in `v1.x.y`; breaking changes wait for
`v2.0.0`. The `1.1` branch becomes the default development line.

## Consequences

- The pre-beta refactor budget has a deadline: anything ugly or
  inconsistent must be cleaned before `v0.9.0` because it can't
  be cleaned cheaply during soak. Pass 10 ("Polish II") is where
  the last opportunistic cleanup happens.
- Pass 11's main deliverable is the data-format audit: every file
  poplar reads or writes (config, cache, OAuth tokens, anything
  in `~/.config/poplar/` or `~/.cache/poplar/`) gets a schema
  version field if it doesn't already, and a documented migration
  path. Without this the freeze isn't credible.
- The `1.1` branch can open the moment `v0.9.0` ships, so feature
  work isn't blocked on the soak. Bug-fix commits can be
  cherry-picked between branches as needed.
- "When does the beta end" is intentionally fuzzy. The 2-week
  no-new-bug heuristic avoids both "ship 1.0 the day after 0.9"
  (no real soak) and "wait forever for zero bugs" (impossible).
