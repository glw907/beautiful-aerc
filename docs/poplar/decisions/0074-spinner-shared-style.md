---
title: Shared spinner constructor for placeholder surfaces
status: accepted
date: 2026-04-25
---

## Context

Pass 2.5b-4 added a viewer-loading spinner with three lines of
inline configuration: `spinner.New()`, set the variant to `Dot`, set
the style to `styles.Dim`. Pass 3 will need the same placeholder for
folder loads, and Pass 9 will need it for send progress. Three copies
of those three lines, each at risk of drifting (different variant,
slightly different shade).

## Decision

`internal/ui/styles.go` exports a single `NewSpinner(t)` constructor:

```go
func NewSpinner(t *theme.CompiledTheme) spinner.Model {
    sp := spinner.New()
    sp.Spinner = spinner.Dot
    sp.Style = lipgloss.NewStyle().Foreground(t.FgDim)
    return sp
}
```

Every placeholder spinner in poplar goes through this helper. The
viewer is the first consumer; future folder-load and send-progress
placeholders adopt it as they land.

The constructor takes a `*theme.CompiledTheme` rather than a `Styles`
value so it can be called before a `Styles` is built (e.g., during
component construction in tests) and so the helper does not pin a
particular `Styles` field.

## Consequences

- **One source of truth for placeholder spinner appearance.** Future
  passes add consumers, not configuration.
- **No `Styles.Spinner` field.** The spinner is a `bubbles/spinner`
  model, not a lipgloss style. Stuffing it into `Styles` would muddle
  the type's role as "lipgloss styles only."
- **Variant changes need one edit.** If the Dot variant ever feels
  wrong, swap to `spinner.Line` (or whichever) in the constructor and
  every consumer follows.
