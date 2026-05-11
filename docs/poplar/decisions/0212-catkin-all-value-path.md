---
title: Catkin all-value path
status: accepted
date: 2026-05-11
---

## Context

Catkin's `Model` and `Buffer` carried pointer-receiver mutators
(`SetValue`, `SetSize`, `Focus`, `Blur`, `SetStyles`,
`SetTidyHighlights`, `RegisterAnnotator`, `SetUserWordlistPath`).
The contagion source is `bubbles/v2/textarea.Model`, itself
pointer-shaped. The 2026-05-11 audit flagged this as an
unenforced Elm boundary: any caller with a `*catkin.Model` could
mutate it from a Cmd closure or a View method, and the
`elm-conventions` rule "mutations only in Update" was satisfied
in spirit but not by types. Compose held catkin via
`mailcompose.Editor` purely to get a stable pointer; the wizard
held `catkin.Model` directly, exposing the inconsistency.

## Decision

`catkin.Model` and `catkin.Buffer` are value types. Runtime
mutations route through value-returning `With*` setters called
from the parent's Update; mount-time configuration uses a fluent
builder. The wrapped `textarea.Model` is sealed inside `Buffer`
and never escapes the package.

Surface:

- Mount: `catkin.New().WithAnnotator(a).WithStyles(s).WithUserWordlistPath(p)`
- Runtime: `WithValue`, `WithSize`, `WithWidth`, `WithFocus`,
  `WithBlur`, `WithTidyHighlights`
- Update: `func (m Model) Update(msg tea.Msg) (Model, tea.Cmd)`
  (signature unchanged; handler body mutates a local value)

The brainstorm considered a Msg-vocabulary alternative
(`SetValueMsg` etc.); setters won on four grounds: matches the
bubbles convention; keeps catkin's Msg surface scoped to genuine
external events; allows single-statement paired mutations
(tidy's text + ranges); satisfies the Elm invariant without
ceremony.

## Consequences

- Compose can hold `catkin.Model` directly. `mailcompose.Editor`
  + `CatkinEditor` no longer have a structural reason to exist
  and delete in Pass 28 (ADR-0213).
- Hidden mutation from a Cmd closure or View method is now a
  type error.
- The textarea-via-value-receiver idiom (mutate a local field
  through a pointer method on an addressable copy, then return
  the copy) is load-bearing for the package and is documented in
  `Buffer.WithValue`.
