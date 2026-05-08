# Poplar Status

**Current pass:** Pass 9m next — CardDAV ingest, swap fixture
suggestions for real contacts cache.

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
| 9m | CardDAV ingest — swap fixtures for real contacts cache (#34) | pending |
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

## Next starter prompt (Pass 9m)

> **Goal.** Swap `contacts.FixtureSuggestions` for a real
> CardDAV-backed contacts cache feeding the compose autocomplete
> seam (`SuggestFn`) and the `i`-popover lookup.
>
> **Scope.** New CardDAV ingest path (`emersion/go-webdav` +
> `emersion/go-vcard`), per-account contacts cache schema, sync
> command, and the cache-backed `SuggestFn` + `LookupByEmail`
> wired into App in place of the fixture pool. Reference issue
> #34. The `compose.Dropdown` shape is fixed (ADR-0174) — only
> the function pointer changes.
>
> **Settled.** ADR-0174 locks the dropdown contract; only the
> `SuggestFn` pointer changes. Phone validation upgrade and vCard
> ingest of saved-form contacts ride along.
>
> **Still open — brainstorm:** schema (separate `contacts.db` vs.
> tables in the per-account cache); sync model (full pull vs.
> CTAG/ETag incremental); ranking with real data (recency /
> frequency, 7-row cap re-evaluated); save destination when
> multiple address books exist.
>
> **Approach.** Brainstorm, write
> `docs/superpowers/plans/YYYY-MM-DD-carddav-ingest.md`, implement.
> Standard pass-end ritual via `poplar-pass`.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
