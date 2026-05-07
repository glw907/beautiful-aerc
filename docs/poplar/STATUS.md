# Poplar Status

**Current pass:** Pass 9k.4 next — UI subpackages + catkin comment sweep.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9h.6 | Scaffold through drafts persistence (ADRs 0001–0165) | done |
| 9.1a/9.1b | Address book mockups + contact edit form (ADRs 0166, 0167) | done |
| 9j | Comment voice infrastructure — §0 rubric, T38–T40 (ADRs 0168, 0169) | done |
| 9k.1 | Comment sweep — mail wire + config; density-floor exemption (ADR-0170) | done |
| 9k.2 | Comment sweep — cache + outbound chain | done |
| 9k.3 | Comment sweep — UI core; T34 demoted to voice-lens (ADR-0173) | done |
| 9k.4 | Comment sweep — UI subpackages + catkin | pending |
| 9l | Compose autocomplete dropdown (To/Cc/Bcc) | pending |
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

## Next starter prompt (Pass 9k.4)

> **Goal.** Final slice of the comment voice sweep:
> `internal/ui/sidebar/`, `internal/ui/compose/`,
> `internal/ui/contacts/`, `internal/ui/movepicker/`,
> `internal/ui/helppopover/`, `internal/catkin/` (~847 comments).
>
> **Method.** Same shape as 9k.1–9k.3. §0(a)/(b)/(c) gate plus the
> paraphrase test on every in-function comment, T39/T40 on every
> godoc, no new comments, commit per-package with `make check`
> green. T34 (semicolon clause-joiner) is voice-lens only per
> ADR-0173 — default to a period, but a considered semicolon is
> fine. Catkin loads its own invariants rule
> (`.claude/rules/catkin-invariants.md`).
>
> **Settled.** §0 + T38–T40 (0168), SPDX gone (0169), density-
> floor exemption (0170), T34 voice-lens-only + voice rules apply
> to all Claude-authored docs (0173).
>
> **Still open.** None — pure implementation pass.
>
> **Approach.** Write `docs/superpowers/plans/YYYY-MM-DD-comment-
> sweep-ui-subpackages.md` with one task per package. Standard
> pass-end ritual via `poplar-pass`.

## Queued

- **#30** — `Sidebar.View` render cache (8.5c overlay pattern). Pickup-of-opportunity.
- **9l** prompt details: see archived 2026-05-07-address-book-mockups.md Task 9.
