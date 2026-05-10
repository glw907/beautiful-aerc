---
title: charm.land/v2 reframes (paste; chrome + cursor already absorbed by 13.2a)
status: accepted
date: 2026-05-10
---

## Context

ADR-0189a left three architectural reframes scoped to 13.2b:
declarative chrome on `tea.View`, App-level cursor hoist via
`Cursor() *tea.Cursor` accessors, and `tea.PasteMsg` arms in compose
(with the catkin URL-paste wrapping that 13.2a deleted). Late in
13.2a's consolidation, the chrome and cursor reframes were folded
into the substrate work — `cmd/poplar/root.go` already drops
`tea.WithAltScreen()`, `App.View()` already sets `v.AltScreen` and
`v.WindowTitle` declaratively, every cursored subpackage exposes
`Cursor() *tea.Cursor`, `App.frameCursor()` already walks the focus
chain (compose → contacts.Form → sidebar search), and every
textinput/textarea calls `SetVirtualCursor(false)`. The bubbletea-
conventions doc was rewritten against the realized state.

That left 13.2b with paste only.

## Decision

Land `tea.PasteMsg` arms — both layers — and treat the chrome and
cursor reframes as already-merged. No alibi commits to re-do work
13.2a delivered; no scaffolding to keep the original 13.2b shape
alive.

**Catkin (`internal/catkin/paste.go`, `catkin.go`).** New
`tea.PasteMsg` arm in `Model.Update` splices the payload at the
cursor in one buffer mutation and records exactly one undo entry
via a new `undoRing.push` (record without coalescing — paste edits
must never be merged with surrounding intra-word typing). When the
payload is a single whitespace-free token starting with `http://`,
`https://`, or `mailto:`, and the cursor sits inside a word, the
word is wrapped as `[word](url)`; otherwise the payload inserts
literally. `urlWrapTarget` is the pure decision helper.

**Compose (`internal/ui/compose/model.go`).** New
`tea.PasteMsg` arm in `Model.Update` routes by focus.
Address fields (To/Cc/Bcc) parse the payload through
`content.ParseAddressList` and rewrite the trailing in-progress
fragment as a chip block of `Name <email>, ` (or bare `email, `)
tokens, atomically — the autocomplete dropdown stays quiet because
no per-rune key cycle runs. Subject and body delegate to the
textinput and editor respectively; catkin's PasteMsg arm handles
the body case. `commitAddrField` and `markEdited` factor the
chip-write and dirty-stamp shape that `acceptSuggestion` already
used.

## Consequences

The realized scope of Pass 13.2 (split into 13.2a + 13.2b)
matches its specification: tree is v2-native, no v1 substrate
left, `tea.PasteMsg` handled at every cursored seam, URL-paste
wrapping restored, undo is well-defined across paste boundaries.
Compose paste no longer thrashes the autocomplete dropdown.

The three named deferrals from 0189a stand:
- `internal/ansix/` audit (does v2 `lipgloss.Width` cover SPUA?)
  → Pass 13.3 or 15.
- Per-subpackage `Styles` restructuring → Pass 15.
- Color profile + `isDark` threading via `term.Resolve` →
  Pass 14.1 or 15.

Pass 14 (first-run wizard) is now unblocked.

ADR-0189a's "what does NOT land in 13.2a" section is stale on
chrome and cursor. The conventions doc is the authoritative
post-13.2 record; this ADR documents the actual seam.
