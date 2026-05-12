# Poplar Status

**Current pass:** Pass 33 — mouse support (reader + attachments
+ scroll). Pass 32 closed ADR-0217 (v2 declarative `tea.View`
fields): `ProgressBar` priority ladder, `ReportFocus` + new-mail
toast, `KeyboardEnhancements` + `GatedBinding`-aware compose
chord hints.

**Beta soak deferred.** Pre-beta rules apply; soak entry gated
on a full audit cycle returning no findings.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 31 | Scaffold through Audit B.2 (ADRs 0001–0216) | done |
| 32 | v2 declarative `tea.View` fields (ADR-0217) | done |
| 33 | **Mouse support** — reader + attachments + scroll | next |
| 34 | Mouse support (sidebar + cross-pane) — optional split from 33 | gated |
| 35 | Native OAuth for Gmail / Outlook IMAP (#42, BYO client ID) | gated |
| 36 | **Audit C** — feature surface | gate |
| 37 | **Audit D** — database (schema ladder, tx boundaries, FTS5, UIDVALIDITY, on-disk shape) | gate |
| 38 | **Audit Final** — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

### Next starter prompt (Pass 33)

> **Goal.** Wire `tea.MouseMode` and `tea.MouseClickMsg` /
> `tea.MouseWheelMsg` so the reader scrolls on wheel, attachment
> chips open on click, and link runs route through xdg-open on
> click. Sidebar and cross-pane mouse is Pass 34.
>
> **Scope.** Reader viewport wheel + click-to-launch on
> harvested URL runs and attachment chips. No keybinding
> changes; mouse is additive.
>
> **Settled (do not re-brainstorm):** `tea.View.MouseMode` is
> the declarative carrier (mirrors ADR-0189b /-0217). Per-row
> hit-testing lives in `reader.Model`.
>
> **Still open — brainstorm these:**
> - MouseMode value — CellMotion vs AllMotion. Trade-off:
>   throughput vs hover affordances (none today).
> - Click bubbling — whose `Update` claims a click? Reader vs
>   App, and how does the link-picker overlay interact?
> - Footnote ribbon click target — bare URL vs `[^N]` glyph;
>   what's the SPUA-safe column math?
>
> **Approach.** Brainstorm the open questions, write a plan doc
> at `docs/superpowers/plans/YYYY-MM-DD-mouse-reader.md`, then
> implement. Standard pass-end checklist applies.
