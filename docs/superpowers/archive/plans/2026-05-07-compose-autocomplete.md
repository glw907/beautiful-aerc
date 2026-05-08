# Pass 9l — Compose autocomplete dropdown

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task.

**Goal.** Wire the To/Cc/Bcc autocomplete dropdown sketched in the
9.1a plan to live compose. New `compose.Dropdown` sub-model
anchored under the focused header field, fed by a `SuggestFn` seam
defaulting to `contacts.FixtureSuggestions`. 9m swaps the seam for
the cache-backed CardDAV query.

**Architecture.** The dropdown is a value-type sub-model owned by
`compose.Model`, not an App overlay — it's a compose surface, so
overlay positioning math doesn't apply. `compose.View` splices the
dropdown's pre-padded rows into the row stream immediately after
the focused-header row. The body height is unchanged in `SetSize`;
the dropdown rows shave editor lines off the bottom of the View
through the existing `len(rows) > c.height` truncation. That's the
pre-beta tradeoff the 9.1a plan flagged: positional, no overlay,
visible body shrinks when the dropdown is open. Acceptable because
focus is on a header field at that moment.

The seam is a function-typed parameter on `New` / `Open`. App
passes `contacts.FixtureSuggestions` directly. No interface — the
function shape is the contract (ADR-0163).

**Settled at plan time.**

- Prefix is the trailing fragment of the focused field after the
  last comma, with leading whitespace trimmed. `"Alice <a@b>, bo"`
  → prefix `"bo"`. < 2 chars → empty dropdown.
- Accept rewrites the focused textinput by replacing the trailing
  fragment with `Name <email>, ` (RFC 5322 angle-addr, comma+space
  trailer so the next entry types in cleanly). Cursor lands at end.
- Up/Down wrap. Tab and Enter both accept. Esc dismisses (clears
  rows; next textinput edit re-runs `SetPrefix` and may repopulate).
- Dropdown row format: `Name <email>` for the head, then a dim
  ` · org` suffix when `Suggestion.Org != ""`. Org rows omit the
  suffix (Org is empty for `IsOrg`). Cursor row uses
  `styles.SelectedRow` (lipgloss reverse).
- Up to 7 rows (matches `FixtureSuggestions` cap).
- Dropdown styling lives in `compose.Styles`. New fields:
  `DropdownRow`, `DropdownRowSelected`, `DropdownOrg`. Constructor
  picks neutral palette slots from `theme.CompiledTheme`.
- Tab/Shift+Tab routing: when dropdown is non-empty and focus is
  in {To, Cc, Bcc}, Tab/Enter accept the highlighted row instead
  of advancing focus. Empty dropdown falls through to the existing
  focus-cycle / send paths.

## Task 1 — `Dropdown` sub-model + tests (TDD)

**Files:** create `internal/ui/compose/suggest.go`,
`internal/ui/compose/suggest_test.go`. Modify
`internal/ui/compose/styles.go` for the new style fields.

- [ ] Write `suggest_test.go` covering: empty when prefix < 2;
      populated when ≥ 2 and `SuggestFn` returns rows; Up/Down
      wrap; `Selected()` returns highlighted row; row render
      format for person vs org; `View()` row count matches
      `len(rows)`; `SetPrefix("")` clears.
- [ ] Run `go test ./internal/ui/compose/...` — expect FAIL.
- [ ] Implement `Dropdown` (value type), `NewDropdown(fn SuggestFn)
      Dropdown`, `SetPrefix(p string) Dropdown`, `Update(msg) (Dropdown, tea.Cmd)`,
      `View() string`, `Empty() bool`, `Selected() (contacts.Suggestion, bool)`.
      `Update` handles Up/Down only; Tab/Enter/Esc are caller-
      driven (compose intercepts those before delegating).
- [ ] Add `DropdownRow`, `DropdownRowSelected`, `DropdownOrg` to
      `compose.Styles`; populate in `NewStyles`.
- [ ] Tests pass.

## Task 2 — Splice into `compose.Model`

**Files:** modify `internal/ui/compose/model.go`,
`internal/ui/compose/model_test.go`.

- [ ] Add `suggest Dropdown` field. Thread `SuggestFn` through
      `New(styles, self, suggest)` and `Open(styles, self, draftID, draft, suggest)`.
- [ ] Add `currentPrefix() string` helper: trailing fragment of the
      focused To/Cc/Bcc field after the last `,`, trimmed.
- [ ] After every `Update` that mutates the focused header field,
      refresh `m.suggest = m.suggest.SetPrefix(currentPrefix())`.
- [ ] In `Update`, before the existing `tea.KeyMsg` switch, if
      `!m.suggest.Empty()` and focus ∈ {To, Cc, Bcc}: route
      Up/Down into `m.suggest.Update`; on Tab/Enter accept the
      selected suggestion, rewrite the focused textinput, clear
      the dropdown; on Esc clear the dropdown.
- [ ] Acceptance helper rewrites the field by replacing only the
      trailing fragment; existing entries before the last comma
      survive verbatim.
- [ ] Test: dropdown populates after typing, accept rewrites
      field, cursor positioned at end, dropdown clears, Tab
      no longer accepts when dropdown empty (falls through to
      focus advance).
- [ ] In `View`, when `!m.suggest.Empty()` and focus ∈ {To, Cc, Bcc},
      splice `strings.Split(m.suggest.View(), "\n")` after the
      focused-header row; each row passes through `padRow` for
      width contract.

## Task 3 — App wiring

**Files:** modify `internal/ui/app.go`.

- [ ] At each `uicompose.New` / `uicompose.Open` call site, pass
      `contacts.FixtureSuggestions` as the `SuggestFn` argument.
      Three call sites total (compose, seeded reply/forward,
      open-draft).

## Task 4 — Verify, commit, ship

- [ ] `make check` green.
- [ ] tmux live-verify at 80×24 and 120×40: open compose, type
      `ali` in To, dropdown shows; Down/Up navigate; Tab accepts
      and rewrites; Esc dismisses; comma-then-`bo` shows new
      suggestions for the trailing fragment only.
- [ ] Pass-end ritual via `poplar-pass`: ADR-0174
      (compose-autocomplete: dropdown shape, prefix rule, accept
      rewrite contract, fixture seam), update invariants Compose
      section + decision index, archive plan, commit, push,
      install.

## Out of scope

- Cache-backed query (9m). The seam stays in place; only the
  function pointer swaps.
- `i`-popover from compose, contact-create-from-unknown-sender.
  These are 9m+ once the cache backs the query.
- Smart resizing of the body when the dropdown opens. Truncation
  is fine for the v1 shape.
