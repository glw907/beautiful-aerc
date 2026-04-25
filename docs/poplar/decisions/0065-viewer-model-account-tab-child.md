---
title: Viewer model — AccountTab child, headers pinned + body viewport
status: accepted
date: 2026-04-25
---

## Context

Pass 2.5b-4 (message viewer prototype). The viewer needed an
ownership model, a state machine for async body fetch, and a
layout that integrates with the persistent sidebar + chrome (ADR
0025) without taking over the screen.

## Decision

The Viewer is a child of AccountTab held by value. It owns no
backend reference: body-fetch and mark-read Cmds are constructed at
the AccountTab level and the result delivered as `bodyLoadedMsg`.
AccountTab guards stale messages by comparing `bodyLoadedMsg.uid`
against `viewer.CurrentUID()` and dropping mismatches.

State machine: closed → loading (on Open) → ready (on bodyLoadedMsg
matching CurrentUID) → closed (on q/esc). The loading phase shows a
centered `bubbles/spinner` placeholder; the ready phase composes
RenderHeaders pinned at the top with the body in a `bubbles/viewport`
below it. Headers stay visible while the body scrolls.

The viewer holds Styles + `*theme.CompiledTheme` to re-render on
every WindowSizeMsg. Re-render runs synchronously on the Update
goroutine — acceptable at prototype scale, revisited only if the
parser/render pipeline becomes a render-path bottleneck.

## Consequences

- Establishes the modal-overlay pattern for upcoming prototypes
  (help popover, folder picker, link picker): a child model owned
  by AccountTab, key routing viewer-first when open, chrome
  context bubbled via Cmds (`ViewerOpenedMsg`/`ViewerClosedMsg`/
  `ViewerScrollMsg`).
- AccountTab now needs the theme passed into its constructor so
  it can construct the viewer. App propagates theme down.
- Mark-read flow is optimistic: AccountTab flips the local seen
  flag via `MessageList.MarkSeen` before the backend MarkRead Cmd
  resolves. Failed backend writes are silently dropped until
  Pass 2.5b-6 wires the toast surface.
- Search keys (`/`, mode cycle, history) and folder jumps
  (`I/D/S/A/X/T`, `J/K`) are inert while the viewer is open — the
  AccountTab key dispatcher routes every key to the viewer first.
