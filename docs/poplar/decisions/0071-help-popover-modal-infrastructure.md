---
title: Help popover modal infrastructure
status: accepted
date: 2026-04-25
---

## Context

Pass 2.5b-5 introduces the first modal overlay in poplar — the help
popover bound to `?`. Three architectural decisions had to be made:
where the modal lives in the model tree, how key routing changes
when it's open, and how it composes visually over the existing view.

## Decision

**Ownership: `App`.** Two new fields land on the root model:
`helpOpen bool` and `help HelpPopover`. The existing `viewerOpen
bool` is the context selector — when `?` is pressed, `App.Update`
constructs the popover for `HelpViewer` if the viewer is open, else
`HelpAccount`. `HelpPopover` is a render-only model fragment
(`View(width, height int) string`); it has no `Init` or `Update` of
its own.

**Key stealing.** When `helpOpen` is true, every `tea.KeyMsg` is
intercepted by an early branch in `App.Update` before any other
routing. Only `?` and `Esc` are handled (both close); every other
key returns `(m, nil)` and is silently swallowed. This is stricter
than the search shelf's modal stealing (search accepts printable
runes for the prompt) — help has no input surface, so the simplest
correct behavior is to swallow everything else. `q` while help is
open is also swallowed; the deliberate divergence from "q clears
search" is that help is a view, not a state to escape.

**Render composition.** When `helpOpen` is true, `App.View` returns
`m.help.View(m.width, m.height)` directly — the underlying account
layout and chrome are skipped entirely. Centering uses
`lipgloss.Place`. Background dimming is **out of scope for v1**:
lipgloss has no native opacity, the popover's accent-colored title +
rounded box + centered placement give enough visual distinction, and
ANSI-level color stripping of the underlying view is fragile. Logged
in BACKLOG to revisit if user testing flags it.

**Rounded box with embedded title.** The popover's box is built with
`lipgloss.NewStyle().Border(RoundedBorder, false, true, true, true)`
(no top edge, all other sides rounded). The top edge is drawn
separately by `renderTopEdge` so the title `╭─ Message List ───╮` can
be embedded — mirrors the existing `top_line.go` chrome trick.
`BorderTop(false)` was rejected: it strips the bottom corners too in
the current lipgloss version.

## Consequences

- The popover is the prototype for future modals (compose draft
  picker, etc.). The pattern — root-owned bool + sentinel struct +
  early Update branch + early View return — is the template.
- No background dim means a future polish pass could add it without
  changing the modal infrastructure; only `View` changes.
- Responsive layout for narrow terminals is also out of scope — the
  popover has a fixed natural width derived from content (~62 cols
  account, ~58 viewer). `lipgloss.Place` clips gracefully on
  narrower terminals. Logged in BACKLOG.
- The "first modal" patterns established here (full key stealing,
  full View takeover) match the help popover's read-only nature.
  Modals with input surfaces (e.g., compose) will need a different
  routing rule.
