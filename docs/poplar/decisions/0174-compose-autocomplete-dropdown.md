---
title: Compose autocomplete dropdown shape and seam
status: accepted
date: 2026-05-07
---

## Context

The 9.1a address-book mockups landed `contacts.FixtureSuggestions`,
the row schema (`contacts.Suggestion`), and the spec for an inline
autocomplete dropdown on the To/Cc/Bcc compose fields. Pass 9l wires
the live dropdown — anchored under the focused header field, rewriting
the textinput on accept. The seam needs to swap to the cache-backed
query in 9m without churning compose internals.

## Decision

`compose.Dropdown` is a value-type sub-model owned by `compose.Model`,
not an App-level overlay. The seam is a function:
`SuggestFn func(prefix string) []contacts.Suggestion`, threaded as a
positional argument through `compose.New` and `compose.Open`. App
passes `contacts.FixtureSuggestions` directly; 9m swaps the function
pointer for a cache-backed query without touching compose.

Behavior:

- Renders only when focus is To/Cc/Bcc and the focused field's
  trailing fragment (text after the last comma, leading whitespace
  trimmed) is at least 2 characters and `SuggestFn` returns rows.
- Cursor: Up/Down with wrap. Tab and Enter accept. Esc dismisses.
- Accept rewrites the focused field's trailing fragment as
  `Name <email>, ` so the next address types in cleanly. Cursor lands
  at end. Earlier addresses (everything up to and including the last
  comma) survive verbatim.
- Splices positionally into `compose.View` immediately after the
  focused-header row. No overlay positioning math. The dropdown
  consumes editor rows via the existing height-truncation path —
  acceptable because focus is on a header field at that moment.
- Row format: `Name <email>` for both kinds, with a dim ` · org`
  suffix for person rows when `Suggestion.Org != ""`. Org rows omit
  the suffix.
- Up to 7 rows (cap matches `FixtureSuggestions`).

## Consequences

- 9m swaps `contacts.FixtureSuggestions` for a cache-backed query at
  the three call sites in `app.go`. The compose package needs no
  further changes for that swap.
- The dropdown's `Update` handles only Up/Down; compose intercepts
  Tab/Enter/Esc upstream. This is the cleanest separation: the
  dropdown is purely a result-set + cursor, with compose owning the
  field rewrite.
- The trailing-fragment rule (last comma + trim) is the textinput
  caret rule for autocomplete; it does not consult cursor position.
  Acceptable because the textinput is single-line and append-only in
  the typeahead flow.
- Body height shrinks visually when the dropdown is open. Future
  passes may move to true overlay positioning; for now positional
  splice keeps the size contract trivially correct.
