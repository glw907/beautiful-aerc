# Poplar Status

**Current pass:** Pass 9.1c next — compose autocomplete dropdown.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a | Address book mockups — package + popover + Contacts mode (ADR-0166) | done |
| 9.1b | Contact edit form — Person/Business, modal + right-pane modes (ADR-0167) | done |
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

## Next starter prompt (Pass 9.1c)

> **Goal.** Compose autocomplete dropdown for To/Cc/Bcc fields,
> driven by the contacts fixture pool until 9.2 swaps in the
> CardDAV-backed cache.
>
> **Scope.** New `internal/ui/compose/suggest.go` carrying the
> dropdown sub-model (anchor below the active address field;
> j/k cursor; Enter/Tab accept; Esc dismiss; close on focus
> change). Splice into `compose/model.go` so address-field
> input drives `SuggestFn(prefix string) []contacts.Suggestion`
> queries; debounce-on-type (≤16ms) is fine. Wire `SuggestFn` to
> `contacts.FixtureSuggestions` (build helper from existing
> fixture pool, flattening `Contact → []Suggestion` per email).
> Update wireframes + keybindings docs.
>
> **Settled:** `contacts.Suggestion` value type and the
> contacts package shape are locked by ADR-0166. Implementation
> reference: archived plan Task 9
> (`docs/superpowers/archive/plans/2026-05-07-address-book-mockups.md`).
>
> **Still open:** None (pure implementation pass).
>
> **Approach.** Write `docs/superpowers/plans/YYYY-MM-DD-compose-autocomplete.md`
> from archived Task 9, then implement. Standard pass-end ritual.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
