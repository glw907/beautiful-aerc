---
title: External editor only
status: superseded by 0034
date: 2026-04-09
---

## Context

Building an inline editor in bubbletea is a massive
effort for marginal benefit. nvim-mail already provides the exact
compose UX we want. Simplifies the UI significantly.

## Decision

No built-in compose editor. Always launch `$EDITOR`
(nvim-mail) via `tea.ExecProcess`.

## Consequences

Superseded by [ADR-0034](0034-inline-compose-over-terminal-takeover.md): inline
compose landed inside bubbletea after all, riding the `catkin`
markdown editor. `$EDITOR` is no longer the only path.
