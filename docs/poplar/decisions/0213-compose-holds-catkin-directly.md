---
title: Compose holds catkin directly
status: accepted
date: 2026-05-11
---

## Context

`mailcompose.Editor` (interface) and `mailcompose.CatkinEditor`
(impl) existed because catkin was pointer-shaped — compose needed
a stable pointer to hold across Update. The Editor interface
advertised a future neovim adapter (ADR-0033) but was already
inconsistent: the wizard's signature section imported
`catkin.Model` directly. The interface was a single-impl seam
preserved for a hypothetical post-v1 consumer — the exact
anti-pattern `go-conventions` flags.

Pass 27 (ADR-0212) converted catkin to a value type. The
structural reason for the wrapper is now gone.

## Decision

`mailcompose.Editor` interface and `mailcompose.CatkinEditor`
adapter delete. `compose.Model.editor` is `catkin.Model` (value).
The compose package mutates the embedded editor from its own
Update via `c.editor = c.editor.WithX(...)`, mirroring the
wizard's shape.

The tidy result handler applies its paired mutation in a single
statement:

```go
c.editor = c.editor.WithValue(text).WithTidyHighlights(text, ranges)
```

This closes the ordering hazard a Msg-routed alternative would
have opened (two Cmds, no guarantee of arrival order).

## Consequences

- ADR-0033 (neovim editor adapter, post-v1) is not superseded —
  its rationale survives. Only the v1-era *implementation
  strategy* via the Editor interface is dropped. The adapter
  shape will be designed fresh when concrete v1.1 requirements
  exist, per CLAUDE.md's "don't design for hypothetical future
  requirements" clause.
- The compose package no longer depends on `mailcompose` for the
  body editor type. Other `mailcompose` exports (Draft, Identity,
  Signature, AssembleMIME) remain in use.
- One fewer single-impl interface in the tree.
