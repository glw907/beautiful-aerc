---
title: Address book mockups — package shape, i-popover, Contacts mode shell
status: accepted
date: 2026-05-07
---

## Context

Pass 9.1 brings poplar's address-book UI online. The original plan
(`docs/superpowers/archive/plans/2026-05-07-address-book-mockups.md`)
covered four surfaces — i-popover, Contacts mode three-column shell,
contact edit form, compose autocomplete dropdown — plus screenshots
and ADR in one pass. After Tasks 1–7 landed cleanly, fatigue tells
appeared (bikeshed-heavy reviews, two architecturally separate
subsystems in one ADR). Per pre-beta posture, the pass split inline:
9.1a lands the contacts package + the popover/mode surfaces; 9.1b
will land the form; 9.1c will land the compose autocomplete.

This ADR locks the package shape and the popover/Contacts-mode
visual decisions. Form-side and autocomplete decisions land in their
own ADRs alongside 9.1b/9.1c.

## Decision

### Package shape

`internal/ui/contacts/` is one package holding every address-book UI
surface: `Contact`/`Email`/`Phone`/`Suggestion`/`Kind` value types
(`types.go`), the fixture pool + `LookupByEmail` (`fixtures.go`),
the per-package `Styles` (`styles.go`), the pure `RenderDetailCard`
function used by both the popover and Contacts mode's right column
(`detail.go`), the `Popover` sub-model (`popover.go`), the T9
`Sidebar` sub-model (`sidebar.go`), the `List` sub-model
(`list.go`), cross-package Msg types (`msgs.go`), and the package
key bindings (`keys.go`). The four sub-models share the `Contact`
value type and the detail-card renderer; splitting them across
helppopover/movepicker-shaped subpackages would scatter five files
with no real seam.

Pass 9.1 fixtures are a Go literal slice in `fixtures.go`. No
`//go:embed`, no JSON. CardDAV ingest + cache-backed lookup land in
9.2.

### i-popover

App-owned overlay at the bottom of the cascade (confirm > conflict
> outbox > help > linkpicker > attachpicker > movepicker > popover).
`i` from the account view (no overlays open) extracts (DisplayName,
Email) from the cursor message's From header via `parseSender` ↔
`content.ParseAddressList`, emits `OpenPopoverMsg`, and the App
handler calls `contacts.LookupByEmail(contacts.Fixtures(), email)`
(case-insensitive) to populate the match. Re-press of `i` or `Esc`
emits `ClosePopoverMsg`.

Two render paths via `uicore.ModalShell`: full `RenderDetailCard`
(match) or "No contact in address book." plus `n add contact · Esc
dismiss` footer (no-match). Title `"Sender"`. Pressing `n` on the
no-match path emits `OpenFormMsg{FromPopover: true}` pre-filled with
the unknown sender's display name + email; 9.1b lands the form
handler.

### Contacts mode three-column shell

App owns `contactsMode bool` plus `contactsSidebar contacts.Sidebar`,
`contactsList contacts.List`, `contactsStyles contacts.Styles`. `C`
from the account view emits `EnterContactsModeMsg`; `M` while in
mode emits `ExitContactsModeMsg`. While `contactsMode`, App routes
keys into Sidebar then List, syncing list cursor to sidebar's
selection letter when it changes. `q` quits poplar (matches
account-view behavior).

Layout is row-by-row three-column composition (`Sidebar | List |
RenderDetailCard`) using `uicore.PadOrTruncate` and `strings.Join`
per ADR-0084 — no `JoinHorizontal` when SPUA cell width may differ
from 1. Sidebar fixed at 14 cells (matches mail-mode floor,
ADR-0109). List/Detail split the remainder. Top chrome row reads
`CONTACTS · All sources`; per-source filtering is 9.2 work.
Footer `j/k cursor · J/K group · a–z jump · n new · e edit · M mail
· q quit` via the new `ContactsContext` in `internal/ui/footer.go`.

### Sidebar binning + per-letter cursor

Eight T9 groups in fixed order (`ABC`, `DEF`, `GHI`, `JKL`, `MNO`,
`PQRS`, `TUV`, `WXYZ`), one row per group with right-aligned count,
blank row between groups. Binning uses `firstSortLetterMode(c,
SortLastName)` — Family for KindPerson, Name for KindOrg. `J/K`
walk groups (clear active letter); `a`–`z` jump per-letter,
inferring the group. The active group's matching letter renders
with a `┃` tick (`Styles.LetterTick`). Group-only selection (J/K)
highlights the entire group label with `Styles.CursorRow`.

### List binning + sort modes

`List` uses `bubbles/viewport` for vertical scroll. `SortFirstName`
(default) and `SortLastName` are constructor-time. `j`/`k` cursor;
`n` emits `OpenFormMsg{Initial: zero}`; `e` emits
`OpenFormMsg{Initial: cursor}`; `D` is intercepted but inert
(delete lands 9.3 per the spec keybinding table).
`SetSelectionLetter(rune)` from sidebar scrolls the cursor to the
first row whose sort key starts with that letter.

### 9.1b / 9.1c wiring contracts

These exported types are the lockable seams the form pass and the
autocomplete pass will consume:

- `contacts.OpenFormMsg{Initial Contact, FromPopover bool}`
- `contacts.ContactSaveMsg{Contact Contact, SaveTo string}`
- `contacts.ContactCancelMsg{Dirty bool}`
- `contacts.OpenPopoverMsg{DisplayName, Email string}`
- `contacts.LookupByEmail(all []Contact, addr string) (Contact, bool)`
- `contacts.Sidebar.SelectionLetter() rune` /
  `Sidebar.SelectionGroup() int`
- `contacts.List.Cursor() Contact` /
  `List.SetSelectionLetter(rune) List`
- `contacts.RenderDetailCard(c Contact, s Styles, width int) string`
- `contacts.FixtureSuggestions(prefix string) []Suggestion` (9.1c
  swaps in a real `SuggestFn` once cache-backed lookup lands)

App holds `OpenFormMsg` routing as a no-op pass-through in 9.1a
(popover dismisses, form not yet built); 9.1b implements `Form` and
takes over the message.

## Consequences

The address-book package shape is locked. 9.1b owns `form.go` +
`form_test.go` plus the App lifecycle (modal-or-right-pane render,
ConfirmModal gate on dirty cancel, ContactSaveMsg routing). 9.1c
owns `internal/ui/compose/suggest.go` + the compose splice.

The fixture pool will be replaced by a CardDAV-backed contacts
cache in 9.2. The wiring contracts above mean the surfaces in this
pass do not change shape — only the data source flips from
`contacts.Fixtures()` to a cache-backed accessor. `LookupByEmail`'s
slice argument absorbs that swap cleanly.

`contacts.Fixtures()` is allocated on every `OpenPopoverMsg`. At
fixture scale this is trivially cheap; once 9.2 lands the cache,
the lookup turns into an indexed query rather than a linear scan,
so this becomes irrelevant.

Pre-beta posture endorsed splitting Pass 9.1 inline. The original
plan archived without further amendment per the immutable-archive
rule; 9.1b and 9.1c get fresh plan documents derived from the
archived plan's Tasks 8 and 9.
