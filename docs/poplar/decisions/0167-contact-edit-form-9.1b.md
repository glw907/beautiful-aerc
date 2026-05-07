---
title: Contact edit form (Pass 9.1b)
status: accepted
date: 2026-05-07
---

## Context

ADR-0166 landed the Pass 9.1a address-book mockups: package shape,
i-popover, Contacts mode shell, T9 sidebar, list. Pass 9.1b owes
the contact edit form — the most complex sub-model in the
addressbook initiative — under the same fixture-only data path.
The spec at `docs/superpowers/specs/2026-05-06-addressbook-design.md`
§"Contact edit form" pins the field set, validation matrix, and
two render contexts: centered modal (over dimmed mail chrome,
opened from the i-popover's "no contact" affordance) and
right-pane replacement (in Contacts mode, opened by `n` or `e`).

## Decision

A single `contacts.Form` value type owns both render contexts.
`fromPopover bool` selects the path: when true, `View()` returns a
ModalShell-bordered box that App composites via `PlaceOverlay` over
a dimmed frame. When false, `View()` returns the form's body+footer
rows without a `┌─┐` frame, letting the existing Contacts-mode
chrome supply borders. Both share one renderer; the only branch is
the outer chrome.

Focus is a single `focusIdx int` resolving against a freshly-built
`focusList()` — kind toggle, name fields per kind, then per-row
quartets `(input, cycler, ★, −)` for emails and phones, then add
buttons, note, and save destination. Mutations that change the
list shape (kind toggle, add/remove row) recompute the list and
clamp via `applyFocus()`. Tab/Shift+Tab cycle. Cyclers respond to
Space/← /→/Enter; star buttons promote-to-primary on Enter; minus
buttons remove on Enter (disabled when only one email remains).

Dirty tracking is derived: `f.initial` is snapshotted from the
form's own `currentContact()` after construction, and `dirty()`
compares the live state against that snapshot. No shadow `dirty
bool` to keep in sync.

`Ctrl+S` is the save chord (text-entry exempt per ADR-0076).
Validation runs on save: Person requires First or Last; Business
requires Name; ≥1 email row; every email parses via
`net/mail.ParseAddress`; saveIdx in range. Failure sets
`f.err` and refocuses; success emits `ContactSaveMsg{Contact,
SaveTo}`. `Esc` always emits `ContactCancelMsg{Dirty}`. App owns
`form *contacts.Form` and a `pendingFormDiscard bool` discriminator
(mirroring the existing `pendingComposeSave bool` pattern), so
`ConfirmModalYesMsg` routes Yes-confirm to clear-form vs. the
empty-folder and save-draft paths.

In Contacts mode, the column-assembly pads sidebar/list rows to
`contentH` so a tall form (taller than the data list) doesn't
collapse the body height. This was an existing 9.1a bug surfaced
by the form; fixed inline.

ContactSaveMsg is logged-and-discarded for 9.1b. The cache layer
that turns the saved Contact into a CardDAV outbox op lands in
9.2 alongside vCard ingest.

## Consequences

**Unlocks.** The address-book initiative now has all three
fixture-driven UI surfaces in place (popover, mode, form). Pass
9.2 can swap the fixture pool for a CardDAV-backed cache without
touching the form. Pass 9.1c can implement the compose
autocomplete dropdown against the same Suggestion type.

**Forecloses.** `Form` is value-typed (mutating methods return a
new Form), but App holds `*contacts.Form` so Update can mutate
through the pointer — same pattern as `*Popover` from 9.1a. A
keep-on-cancel micro-refactor would require either making
ContactCancelMsg carry the form back, or changing App's storage
to a value with explicit "open" flag. Neither is a clear win at
this stage.

**Phone validation.** Phones accept any non-empty string in 9.1b.
Strict E.164 normalization waits for `phonenumbers` (spec §Library)
in 9.2. This is a deliberate "don't engineer the unblockable" call
under the pre-beta posture: the form ships, validation is honest
about what it checks today, and the upgrade is local to
`Form.Validate`.

**Inline fix to the Contacts-mode column assembly.** The 9.1a
implementation took `min(contentH, len(sb), len(list))` as the body
height, which collapsed when the data was shorter than the
viewport. Fixed inline to pad up to `contentH` per ADR-0166's
note that 9.1a was scaffolding subject to follow-up tightening.
