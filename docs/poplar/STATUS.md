# Poplar Status

**Current pass:** Pass 9k.1 next — mail wire + config comment sweep.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a | Address book mockups — package + popover + Contacts mode (ADR-0166) | done |
| 9.1b | Contact edit form — Person/Business, modal + right-pane modes (ADR-0167) | done |
| 9j | Comment voice infrastructure — §0 rubric, T38–T40, SPDX removal (ADRs 0168, 0169) | done |
| 9k.1 | Comment sweep — mail wire + config (`mail`, `mailimap`, `mailjmap`, `mailauth`, `config`, `term`, `cmd/poplar`) | pending |
| 9k.2 | Comment sweep — cache + outbound chain (`cache`, `compose`, `content`, `filter`, `tidy`, `humanize`, `backoff`, `theme`) | pending |
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

## Next starter prompt (Pass 9k.1)

> **Goal.** First slice of the comment voice sweep: bring
> `internal/mail/`, `internal/mailimap/`, `internal/mailjmap/`,
> `internal/mailauth/` (light), `internal/config/`,
> `internal/term/`, and `cmd/poplar/` into compliance with the §0
> write-time rubric and the T38–T40 structural tells installed in
> Pass 9j. ~865 comment lines in scope; pre-sweep densities run
> mail 21.1%, mailimap 13.7%, mailjmap 11.4%, config 10.8%, term
> 15.9%, mailauth 28.8% (mostly vendored — carve-out applies).
> Target is application-code density (~7–9%) with zero
> paraphrase-of-next-5-lines comments and zero markdown-shaped
> godocs.
>
> **Why this slice first.** Mail wire surface is the highest-
> impact contributor-recruitment surface — protocol code is
> what other devs read first when evaluating the project.
> Backend logic stays in one review window. Config and term are
> small-but-shape-setting; cmd/poplar at 21 cmt is essentially
> a freebie that ends a pass cleanly.
>
> **Scope.**
>
> 1. `internal/mail/` (~156 cmt, 21.1%) — wire types and
>    classification. Highest-density of the wire packages and
>    most-read by contributors.
> 2. `internal/mailimap/` (~287 cmt, 13.7%) — IMAP backend.
>    Largest single-package comment count in this slice.
> 3. `internal/mailjmap/` (~198 cmt, 11.4%) — JMAP backend.
> 4. `internal/mailauth/` — vendored. **Light touch only.**
>    Per ADR-0169 the provenance comment block stays. Trim only
>    the non-vendored helper comments (e.g. `dialRawTCP`,
>    `auth.go` glue) where §0 applies.
> 5. `internal/config/` (~152 cmt, 10.8%) — TOML loader,
>    provider registry, validation.
> 6. `internal/term/` (~51 cmt, 15.9%) — capability detection
>    (NF autodetect, CPR probe). Small surface.
> 7. `cmd/poplar/` (~21 cmt, 2.6%) — already at target density;
>    a quick pass-through to confirm the §0 rubric holds and the
>    pass ends with a clean check on the entry-point package.
>
> **Method.** For each package, in the order above:
> - Walk every `.go` file (skip `*_test.go` unless a test
>   carries a markdown-shaped godoc — tests have their own voice
>   per the deliberate-out-of-scope note in Pass 9j's plan).
> - For each comment: apply §0(a)/(b)/(c). If (a) or (b), delete.
>   If (c), keep but rewrite if shape trips T39/T40 semantically
>   (the narrow grep-tier forms are already enforced by
>   `make check`).
> - Apply the paraphrase test mechanically on every in-function
>   comment.
> - Compress label-colon godocs and reference-stuffed paragraphs
>   into prose. One ADR/RFC cite per godoc; pick the strongest.
> - **Don't** touch vendored provenance blocks.
> - **Don't** add comments. Density goes down, never up.
> - Commit per-package so each diff is reviewable in one window.
>   Run `make check` at every commit; the new T39/T40a/T41 scans
>   will catch most regressions automatically.
>
> **Acceptance.**
> - Per-package density at or below 9% (vendored mailauth excluded).
> - Zero paraphrase comments — verified by spot-check, not grep.
> - Zero label-colon godoc paragraphs (semantic form, beyond what
>   the grep catches).
> - At most one ADR/RFC cite per godoc.
> - `make check` green at every commit.
> - One ADR (suggest 0170) summarizing the slice's findings if
>   anything notable surfaced (e.g., a recurring shape that
>   should become a new tell). Otherwise no ADR — pass-end ritual
>   only.
>
> **Settled.** §0 rubric, T38–T40 entries, and the §9b
> calibrated pairs (ADR-0168). SPDX is gone (ADR-0169).
> Mail-stack architecture is locked (ADRs 0075, 0098, 0099,
> 0101, 0104). Vendored provenance carve-out is locked.
>
> **Still open.** Whether 9k.1 surfaces a recurring shape worth
> promoting to T41+. Decide at pass end.
>
> **Approach.** Write `docs/superpowers/plans/YYYY-MM-DD-comment-
> sweep-mail-config.md` with one task per package. Standard
> pass-end ritual via `poplar-pass`.

## Next-next starter prompt (Pass 9k.2)

> Cache + outbound chain. `internal/cache/` (~344 cmt, 14.0% —
> largest single package), `internal/compose/`, `internal/content/`,
> `internal/filter/` (21.3%), `internal/tidy/`, plus the tinies
> (`internal/humanize/`, `internal/backoff/`, `internal/theme/`).
> ~788 cmt total. Same method as 9k.1.

## Then (Pass 9k.3)

> UI core — `internal/ui/` (App layer), `internal/ui/uicore/`
> (28.1%, worst single-package density in the tree),
> `internal/ui/account/` (15.5%), `internal/ui/messagelist/`
> (19.6%), `internal/ui/reader/`. ~1135 cmt — largest slice;
> watch for split-on-fatigue triggers. The bubbletea size
> contract and idiomatic-bubbletea conventions are in
> `docs/poplar/bubbletea-conventions.md` — load before touching
> View() or layout godocs.

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
