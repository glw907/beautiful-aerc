---
title: Catkin owns its renderer; bubbles/textarea is the buffer primitive
status: accepted
date: 2026-05-04
---

## Context

ADR-0076 chose `bubbles/textarea` as the foundation for buffer storage, cursor
management, and edit operations inside Catkin. It did not specify renderer
ownership. Pass 9 had to choose between two approaches:

1. Render through textarea's `View()` — overlay lipgloss styling on top of
   textarea's ANSI escape output.
2. Render directly from the raw buffer — Catkin reads the source string,
   classifies blocks, and produces displayed lines independently.

The compose spec commits Catkin to live markdown styling (iA-Writer-shaped:
`**bold**` displays bold with the asterisks visible) and block-aware paragraph
reflow. Both require the renderer to understand block context — context that
exists in the raw source text, not in textarea's wrapped ANSI output. Parsing
textarea's escape sequence stream to recover that context proved fragile and
brittle against future bubbles releases.

## Decision

Catkin owns its `View()`. The package uses `textarea.Model` for buffer storage,
edit commands, and cursor row/col positioning, but Catkin's `Render()` reads the
raw buffer string, runs `Classify` (the block classifier), and produces
displayed lines independently. Textarea's own `View()` is not called.

The package is library-pure: dependencies are `bubbletea`, `bubbles`, `lipgloss`,
and `muesli/reflow` only — no poplar imports. This makes the package extractable
as `github.com/glw907/catkin`.

## Consequences

- Catkin is now a renderer, not just a buffer wrapper. Future styling (Pass 9a),
  annotation overlays (Pass 9d), and display modes (Pass 9c) live in `render.go`
  and consume the same `Classify` output.

- Cursor positioning math reads the raw buffer via `Buffer.RuneOffset` /
  `SetRuneOffset` — no escape-sequence parsing required.

- `SetRuneOffset` must navigate row before column: `textarea.SetCursor` clamps
  to the current row's length, so row must be set before column.

- `remapCursor` uses non-whitespace rune count for cross-reflow cursor mapping.
  Whitespace boundaries shift during reflow; content characters align. This
  preserves cursor intent across paragraph reflows.

- Lipgloss styles add zero display cells. Display invariants hold across styled
  and unstyled output — a property that becomes load-bearing in Pass 9a when
  lipgloss styles wrap individual tokens.

- Catkin's dependency on textarea is internal. If textarea changes its `View()`
  ANSI shape in a future bubbles release, Catkin is unaffected.
