# Poplar Status

**Current pass:** Pass 9.1b next — contact edit form.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a | Address book mockups — package + popover + Contacts mode (ADR-0166) | done |
| 9.1b | Contact edit form (Person + Business, modal + right-pane modes) | pending |
| 9.1c | Compose autocomplete dropdown (To/Cc/Bcc) | pending |
| 9.2 | CardDAV ingest — swap fixtures for real contacts cache (#34) | pending |
| 9.4 | Email signatures + multiple identities (#32) | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending |
| 9.3 | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9.7 | List-Unsubscribe one-click, RFC 8058 (#36) | pending |
| 9.8 | Calendar invite (.ics) viewer (#37) | pending |
| 9.9 | Full-account / cross-folder search (#38) | pending |
| 9.6 | First-run wizard (#27) + OAuth refresh + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.9 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9.1b)

> **Goal.** Land the contact edit form (Person + Business). Two
> render contexts: centered modal over dimmed mail chrome (i-popover
> no-match), or right-pane replacement in Contacts mode (n/e in list).
>
> **Scope.** New `internal/ui/contacts/form.go` + `form_test.go`
> using `bubbles/textinput`/`textarea`. Kind toggle, repeating
> email/phone rows with label cyclers and position-as-primary,
> note, save-destination radio, full validation. App owns
> `form *contacts.Form` lifecycle (route keys while non-nil;
> layered-modal vs. right-pane render keyed on
> `OpenFormMsg.FromPopover`; dirty cancel gated through
> `ConfirmModal`; `ContactSaveMsg` log+discard until 9.2).
> Update wireframes + keybindings docs.
>
> **Settled:** Package shape and Msg contracts locked by ADR-0166.
> Implementation reference: archived plan Task 8
> (`docs/superpowers/archive/plans/2026-05-07-address-book-mockups.md`);
> visual contract: spec §"Contact edit form".
>
> **Still open:** None (pure implementation pass).
>
> **Approach.** Write `docs/superpowers/plans/YYYY-MM-DD-contact-form.md`
> from archived Task 8, then implement. Standard pass-end ritual.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
- **9.1c** — Compose autocomplete: implement `internal/ui/compose/suggest.go` + `compose/model.go` splice; wire `SuggestFn` to `contacts.FixtureSuggestions` until 9.2 swaps in the cache-backed query. Reference: archived plan Task 9.
