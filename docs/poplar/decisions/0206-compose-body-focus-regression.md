---
title: Restore editor.Focus() in compose setFocus(focusBody)
status: accepted
date: 2026-05-11
---

## Context

Pass 9n (commit `7d5dbe6`, "per-account identities + signatures")
added `focusFrom` to the compose Tab cycle. The diff collapsed the
existing `case focusBody: _ = c.editor.Focus()` arm together with
the new (empty) `case focusFrom:` into one case label:

    case focusBody, focusFrom:

That dropped the `Focus()` call. `bubbles/v2/textarea.Update`
returns early on `!m.focus`, so once any field had stolen focus
(`To` is focused at construction in `newModel`), Tab or Esc into
the body left the underlying textarea blurred and every keystroke
was silently discarded. Catkin's auto-blink Cmd kept rendering the
block cursor, masking the failure visually. The bug shipped in
`v0.9.0` and was first noticed while iterating on the compose
hero screenshot — VHS-driven keystrokes after Tab into body
landed nowhere.

## Decision

Split the cases. `setFocus(focusBody)` re-calls
`c.editor.Focus()`; `setFocus(focusFrom)` remains a no-op (the
From cycler is rendered chrome, not an input widget).
`internal/ui/compose/model_test.go` carries a regression test
(`TestComposeFocusBodyAcceptsKeystrokes`) that Tabs through the
focus cycle to `focusBody` and asserts `c.editor.Focused()` plus
one accepted keystroke.

## Consequences

First post-tag bug fix on the `v0.9.x` line. No schema, no user-
visible feature changes — restores intended behavior. The
collapsed-cases shape is a tell for similar bugs: any `case A, B:`
that started life as two distinct arms deserves a second look
during future refactors.
