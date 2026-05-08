# Poplar Status

**Current pass:** Pass 9m.1 next — CardDAV write-back: form save
round-trip via outbox, ETag round-trip, deletion UI.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a/9.1b | Address book mockups + contact edit form (ADRs 0166, 0167) | done |
| 9j | Comment voice infrastructure — §0 rubric, T38–T40 (ADRs 0168, 0169) | done |
| 9k.1 | Comment sweep — mail wire + config; density-floor exemption (ADR-0170) | done |
| 9k.2 | Comment sweep — cache + outbound chain | done |
| 9k.3 | Comment sweep — UI core; T34 demoted to voice-lens (ADR-0173) | done |
| 9k.4 | Comment sweep — UI subpackages + catkin | done |
| 9l | Compose autocomplete dropdown — fixture-backed To/Cc/Bcc (ADR-0174) | done |
| 9m | CardDAV ingest — swap fixtures for real contacts cache (ADR-0175) | done |
| 9m.1 | CardDAV write-back — form save round-trip via outbox | pending |
| 9n | Email signatures + multiple identities (#32) | pending |
| 9o | Claude Tidy implementation | pending |
| 9p | Attachments-richer compose UI (#24) | pending |
| 9q | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9r–9t | List-Unsubscribe (#36), .ics viewer (#37), full-account search (#38) | pending |
| 9u | First-run wizard (#27) + OAuth refresh + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14) + items surfaced during 9j–9u | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 9m.1)

> **Goal.** Round-trip the contact-edit Form: form save → cache
> upsert → outbox PUT → CardDAV server. Round-trip the form
> discard via the existing outbox DiscardOp.
>
> **Scope.** Extend `cache.OpKind` with `KindContactPut` and
> `KindContactDelete`; outbox payload carries vCard bytes for
> these kinds. Drainer dispatches via the CardDAV client added in
> 9m. Form's "Save to" cycler now affects the destination address
> book (uses `contacts.AddressBook` value type).
>
> **Settled.** Storage shape (vCard blob + projection columns) is
> 9m's; no schema migration. Default-addressbook config pin
> already implemented. Conflict matrix (auth/not-found/transient)
> mirrors mail outbox semantics. Phone validation already in
> place via `phonenumbers.Parse`.
>
> **Still open — brainstorm:** vCard regeneration on edit (fully
> rebuild from projection vs. patch the stored blob); ETag
> round-trip across local edits; deletion UI in the Form.
>
> **Approach.** Brainstorm, write
> `docs/superpowers/plans/YYYY-MM-DD-carddav-writeback.md`,
> implement. Standard pass-end ritual via `poplar-pass`.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
