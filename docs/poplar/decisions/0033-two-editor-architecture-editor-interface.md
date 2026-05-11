---
title: Two-editor architecture with Editor interface
status: accepted
date: 2026-04-11
---

## Context

Catkin provides an out-of-the-box experience for
everyone. Neovim embedding is the 1.1 killer feature — inline nvim in
the right panel while sidebar and chrome stay visible. No terminal
email client does this today. The `Editor` interface is designed in v1
so the neovim implementation slots in without refactoring.

## Decision

Compose supports two editor backends behind an `Editor`
interface: Catkin (v1 default) and neovim via `--embed` RPC (v1.1).
Config selects the editor. The compose panel, header region, lifecycle,
and send pipeline are shared.

## Consequences

No follow-on notes recorded.

- Superseded — implementation strategy. ADR-0213 deleted the
  `mailcompose.Editor` interface that this ADR proposed as the
  v1.1 adapter seam. The neovim adapter remains a post-1.0 goal;
  its shape will be designed fresh against value-typed
  `catkin.Model` when concrete requirements exist.
