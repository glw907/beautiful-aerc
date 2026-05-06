---
title: Catkin targeted golangci-lint sweep — fix inline, no permanent gate
status: accepted
date: 2026-05-05
---

## Context

Pass 9d.3 ran golangci-lint over `internal/catkin/` with a wider
linter set than the project default (`errcheck`, `staticcheck`,
`unused`, `gocritic`, `revive`, `errorlint`, `unparam`, `nilerr`).
The sweep flagged three issues across 5237 LOC: an unused `lines`
parameter on `reflowGroup`, an interface-required parameter on the
`fakeAnnotator.Annotate` test helper, and a `tea.Cmd` return on
`handlePopoverKey` that is always `nil`.

## Decision

Fix all three findings inline. Do not add a path-scoped
`lint-catkin` Makefile target. Three findings on a settled package
do not justify a per-package gate; if a wider linter set ever
becomes a permanent commit gate, it should land tree-wide as its
own pass.

`reflowGroup`'s `lines` parameter is dropped (caller updated).
`fakeAnnotator.Annotate`'s `src` is renamed `_` — the parameter is
fixed by the `Annotator` interface signature. `handlePopoverKey`'s
return shape is `(bool, Model)`; the lone caller now passes `nil`
into `maybeScheduleAnnotateAfterMutation` directly.

## Consequences

`internal/catkin/` is clean against the eight-linter set. Future
catkin work that wants the same gate runs the one-liner from the
plan starter prompt. A tree-wide gate decision is deferred — the
existing `.golangci.yml` (errcheck, govet, ineffassign, staticcheck,
unused, gosimple, unparam) remains the documented baseline.
