# Poplar Status

**Current pass:** Pass 9.1 next — address autocomplete from CardDAV (#34).

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.5 | Scaffold through drafts persistence (ADRs 0001–0164) | done |
| 9h.6 | IMAP PushDraft + race-immune drafts + superseded banner (ADR-0165) | done |
| 9.1 | Address autocomplete from CardDAV (#34) | pending |
| 9.4 | Email signatures + multiple identities (#32) | pending |
| 9i | Claude Tidy implementation | pending |
| 9.5 | Attachments-richer compose UI (#24) | pending |
| 9.2 | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9.3 | List-Unsubscribe one-click, RFC 8058 (#36) | pending |
| 9.7 | Calendar invite (.ics) viewer (#37) | pending |
| 9.8 | Full-account / cross-folder search (#38) | pending |
| 9.6 | First-run wizard (#27) + OAuth refresh + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9–9.8 | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9.1)

> **Goal.** Address autocomplete from CardDAV in compose To/Cc/Bcc
> fields (#34).
>
> **Scope.** Read CardDAV addressbooks via `emersion/go-webdav` /
> `emersion/go-vcard` (already vendored). Background-refresh
> contacts cache; expose a lookup surface compose can query as
> the user types. UI: inline dropdown under the focused header
> input; `Tab`/`Enter` accepts top match, arrows navigate.
> **Out:** address-book editing (post-1.0 contacts initiative),
> sync conflict UX, multi-account contact merging.
>
> **Settled.** CardDAV is the only v1 contact source — no LDAP,
> no Google Contacts API. Storage shape (per-account contacts
> table vs. file cache) is still open.
>
> **Open.** Where the contacts cache lives (cache.Account schema
> v8 vs. separate sqlite). Sync cadence (polling interval vs.
> WebDAV ETag/sync-collection). Match scoring (prefix only vs.
> token-aware). Autocomplete UI (inline below input vs.
> popover overlay).
>
> **Approach.** Brainstorm the open questions, write
> `docs/superpowers/plans/YYYY-MM-DD-address-autocomplete.md`,
> implement. Standard pass-end checklist applies.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
