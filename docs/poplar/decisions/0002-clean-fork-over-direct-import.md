---
title: Clean fork over direct import
status: superseded by 0075
date: 2026-04-09
---

## Context

Aerc doesn't maintain a stable library API — internal
packages change without warning. A fork with upstream tracking
(cherry-pick protocol fixes) is more stable than chasing breaking
`go get -u` updates.

## Decision

Fork aerc's worker code rather than importing aerc as
a Go dependency.

## Consequences

**Superseded 2026-04-25 by ADR-0075.** The "Go JMAP landscape too
thin" premise no longer holds: `rockorager/go-jmap` covers the
full surface poplar needs and is already a dependency. Pass 2.9
research showed the fork's value-add over the underlying
libraries is mostly aerc's worker idiom, which the synchronous
`mail.Backend` then has to bridge back. Replaced by direct
calls to `emersion/go-imap` v1 and `rockorager/go-jmap`.
