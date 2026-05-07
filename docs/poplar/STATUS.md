# Poplar Status

**Current pass:** Pass 9k.3 next — UI core comment sweep.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a | Address book mockups — package + popover + Contacts mode (ADR-0166) | done |
| 9.1b | Contact edit form — Person/Business, modal + right-pane modes (ADR-0167) | done |
| 9j | Comment voice infrastructure — §0 rubric, T38–T40, SPDX removal (ADRs 0168, 0169) | done |
| 9k.1 | Comment sweep — mail wire + config; density-floor exemption for header-shaped packages (ADR-0170) | done |
| 9k.2 | Comment sweep — cache + outbound chain (`cache`, `compose`, `content`, `filter`, `tidy`, `humanize`, `backoff`, `theme`) | done |
| 9k.3 | Comment sweep — UI core (`internal/ui`, `uicore`, `account`, `messagelist`, `reader`) | pending |
| 9k.4 | Comment sweep — UI subpackages + catkin (`sidebar`, `compose`, `contacts`, `movepicker`, `helppopover`, `catkin`) | pending |
| 9l | Compose autocomplete dropdown (To/Cc/Bcc) — finishes the address book mockup sequence started in 9.1a/9.1b | pending |
| 9m | CardDAV ingest — swap fixtures for real contacts cache (#34) | pending |
| 9n | Email signatures + multiple identities (#32) | pending |
| 9o | Claude Tidy implementation (was 9i) | pending |
| 9p | Attachments-richer compose UI (#24) | pending |
| 9q | Outbox delivery controls — undo + schedule send (#35) | pending |
| 9r | List-Unsubscribe one-click, RFC 8058 (#36) | pending |
| 9s | Calendar invite (.ics) viewer (#37) | pending |
| 9t | Full-account / cross-folder search (#38) | pending |
| 9u | First-run wizard (#27) + OAuth refresh + config template fix (#29) | pending |
| 10 | Polish II — popover dim (#14); items surfaced during 9j–9u | pending |
| 11 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag `v0.9.0` | pending |
| **Beta soak** | Bug-fix releases on master; data formats frozen; new features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |
| 2.5b-train | Tooling: mailrender training capture | opportunistic |

## Next starter prompt (Pass 9k.3)

> **Goal.** Third slice of the comment voice sweep: UI core.
> `internal/ui/` (App layer), `internal/ui/uicore/` (worst single-
> package density in the tree), `internal/ui/account/`,
> `internal/ui/messagelist/`, `internal/ui/reader/`.
>
> **Method.** Same as 9k.1/9k.2: §0(a)/(b)/(c) gate plus the
> paraphrase test on every in-function comment, T39/T40 shape on
> every godoc, ≤1 ADR/RFC cite per godoc, no new comments, commit
> per-package with `make check` green. Watch for em dashes (T33),
> semicolon clause-joiners (T34), and label-colon godoc openers
> (T39) — these are the regression patterns 9k.2 hit repeatedly.
>
> **Settled.** §0 + T38–T40 (ADR-0168). SPDX gone (ADR-0169).
> Density-floor exemption for header-shaped / small-public-API
> packages is locked (ADR-0170); the filter pipeline shape (small
> named pipeline stages with required godocs) is a fresh data
> point that the exemption rationale fits implementation-heavy
> packages too when the public surface dominates the body.
>
> **Still open.** None — pure implementation pass. UI passes load
> `docs/poplar/bubbletea-conventions.md` before touching `View()`
> or layout godocs. Largest slice in the comment-sweep series —
> watch for split-on-fatigue triggers.
>
> **Approach.** Write `docs/superpowers/plans/YYYY-MM-DD-comment-
> sweep-ui-core.md` with one task per package. Standard pass-end
> ritual via `poplar-pass`.

## Then (Pass 9k.4)

> UI subpackages + catkin — `internal/ui/sidebar/`,
> `internal/ui/compose/`, `internal/ui/contacts/`,
> `internal/ui/movepicker/`, `internal/ui/helppopover/`,
> `internal/catkin/`. ~847 cmt. Catkin has its own invariants
> rule (`.claude/rules/catkin-invariants.md`) — load it before
> touching the editor surface.

## Then (Pass 9l) — was 9.1c

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
