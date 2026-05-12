# Poplar Status

**Current pass:** Pass 32 — v2 declarative View fields
(ProgressBar + ReportFocus + KeyboardEnhancements). Pass 31
closed Audit B.2 (ADR-0216) with one inline fix —
`App.updateKey` split into `routeOverlayKey` + `updateGlobalKey`
— and one note-only observation (six >600-line UI files, each
named, deferred to the `ui-all-value` ROADMAP project).
**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 30 | Scaffold through Audit B.1 (ADRs 0001–0215) | done |
| 31 | Audit B.2 — general structural integrity (ADR-0216) | done |
| 32 | **v2 declarative View fields** — ProgressBar + ReportFocus + KeyboardEnhancements | next |
| 33 | Mouse support (reader + attachments + scroll) | gated |
| 34 | Mouse support (sidebar + cross-pane) — optional split from 33 | gated |
| 35 | Native OAuth for Gmail / Outlook IMAP (#42, BYO client ID) | gated |
| 36 | **Audit C** — feature surface | gate |
| 37 | **Audit D** — database (schema ladder, tx boundaries, FTS5, UIDVALIDITY, on-disk shape) | gate |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 32)

> **Goal.** Wire the three `tea.View` fields poplar has access to
> but doesn't set: `ProgressBar` (OSC 9;4 for sync / outbox /
> attachments), `ReportFocus` + `FocusMsg`/`BlurMsg` (pause IDLE
> + JMAP push on blur, refresh on focus), and
> `KeyboardEnhancements` (Kitty keyboard protocol — disambiguates
> Ctrl+I/M/H from Tab/Enter/Backspace for catkin's chords,
> unlocks `IsRepeat` and `ShiftedCode`/`BaseCode`).
>
> **Scope.** All three set per-frame on `App.View()`'s `tea.View`
> with graceful fallback. Mouse is Pass 33, not this one.
>
> **Settled (do not re-brainstorm):** `tea.View` is the per-frame
> declarative chrome surface (ADR-0189b). The ROADMAP entry
> `v2-view-fields` describes the sub-features.
>
> **Still open — brainstorm these:**
> - ProgressBar value source — OSC 9;4 carries one value; need
>   a tie-breaker across simultaneous long-running ops.
> - ReportFocus resume — re-arm IDLE refresh or full re-issue?
>   Same lens as the dead-handle audit (#53).
> - KeyboardEnhancements feature-detect + catkin binding table —
>   adjust in place or fork a popover view?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/2026-05-13-v2-view-fields.md`,
> then implement. Standard pass-end checklist applies.
