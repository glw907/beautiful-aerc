---
title: ComposeTab — inline compose surface, focus model, send/cancel
status: accepted
date: 2026-05-06
---

## Context

Pass 9h needed an inline compose surface to reach feature-complete
outbound mail: a place to type `c` (new), `r`/`R` (reply, reply-all),
or `f` (forward) and send the result through the cache outbox built
in Pass 9g. Catkin (ADR-0144 family) already exists as the body
editor and `compose.AssembleMIME` (ADR-0156) already turns a `Draft`
into MIME bytes; the missing piece was the App-level surface that
ties them together.

A side question: how should compose interact with the rest of the
App? Vim norms say "buffer, not popup," and poplar's keybinding
philosophy already rules out modal command lines. The viable shapes
were a top-level tab next to AccountTab, an `AccountTab` mode field,
or an App-level mode that swaps the right pane out from under the
sidebar.

## Decision

ComposeTab is an App-owned inline compose surface. While
`App.compose != nil`, App routes keys into ComposeTab and renders
its `View()` in place of AccountTab's right pane (sidebar and
chrome stay drawn; no overlay, no `tea.ExecProcess`). ComposeTab
itself is presentation-only — it owns five `bubbles/textinput`
header fields (To/Cc/Bcc/Subject) and a `compose.Editor` body, and
emits `ComposeSendMsg` / `ComposeCancelMsg` for App to translate
into cache ops.

Focus model: `Tab` / `Shift+Tab` cycles To→Cc→Bcc→Subject→Body and
wraps. `Esc` is a focus toggle only (Body→Subject; any header→Body)
and never closes compose. `Ctrl+X` sends; `Ctrl+C` cancels (opens a
discard `ConfirmModal` if dirty, closes immediately when clean).
Per ADR-0076, text-entry surfaces are exempt from the modifier-free
rule, so these chords are deliberate and coexist with Catkin's
own `Ctrl+B/I/K/L/Q/Space` body bindings.

App routes the send path through a new function-pointer seam,
`TidyFn` on App, defaulting to identity. Pass 9i swaps in Claude
Tidy. The seam is a function pointer rather than an interface
because there is exactly one runtime impl plus the test seam — an
interface here is the single-impl tell `go-conventions` warns about.

`AccountTab` grows a small `RenderWithRightPane(rightPane string)`
accessor so App can inject a non-AccountTab right pane while
preserving AccountTab's row-by-row sidebar join (the SPUA-A-safe
shape per ADR-0084). `assembleColumns` was extracted from
`AccountTab.View()` to support both call paths.

Single-instance for Pass 9h. Drafts persistence (multi-compose) is
9h.5; address autocomplete is 9.1; signatures + identities is 9.4.

## Consequences

- The "ComposeTab" name is misleading — there's no tab UI in poplar.
  Pass 9h.1 will rename it (and `AccountTab`) as part of a wider
  organizational sweep before v1.0. Until then, the type keeps the
  Pass 9h name to minimize churn during follow-up compose passes
  (9h.5 / 9.1 / 9.4 / 9.5).
- App's Update gains four new Msg cases (ComposeSendMsg,
  ComposeCancelMsg, composeSentMsg, composeSeededMsg) and three
  key arms (Compose, Reply/ReplyAll/Forward). The cmds.go growth
  is one of the triggers that motivates the 9h.1 reorg.
- `mail.Backend` gains an `IsJMAP() bool` predicate so the cache
  outbox can branch protocol shapes — see ADR-0160.
- The `r`/`R`/`f`/`c` keys are now wired (previously unbound). The
  help popover flips them to `wired: true`.
