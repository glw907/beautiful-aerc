# Poplar Status

**Current pass:** Pass 44 landed async backend connect (ADR-0242).
Pass 45 (catkin render-time soft wrap) is next.

**Dogfood phase, pre-beta rules in force.** Geoff is the sole
user; soak enters when a second user lands or Geoff calls feature
freeze. Pass 35.1 still pending Gmail/Outlook creds.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 44 | Scaffold through async backend connect (ADRs 0001–0242) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 45 | Catkin render-time soft wrap (Typora/iA Writer model) | next |
| 46+ | Dogfood-driven fixes + quality (rolling) | queued |
| Beta soak | Gated on second user or feature freeze | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 45)

> **Goal.** Move catkin's soft-wrap to the Typora / iA Writer
> model: source pristine, renderer word-wraps visually each
> paint, and a wrapped continuation of a quoted line carries
> `> ` (× source `QuoteDepth`) on every visual row. Live editing
> in a long quoted paragraph stops overflowing; resize stops
> being required to re-fit. Roadmap: `catkin-soft-wrap`.
>
> **Scope.** `internal/catkin/render.go` + tests, plus any
> downstream consumer assuming character-cell wrap. Adjacent
> fixes inline.
>
> **Settled.**
> - Source pristine; typing never mutates for wrap. `Reflow`
>   becomes a manual command (or wrap-on-send in
>   `mailcompose.AssembleMIME`), no longer auto.
> - `mailcompose` 72-cell pre-wrap of seeded quotes stays.
> - `LineContext.QuoteDepth` + `buildPrefix` (`reflow.go`) drive
>   the prefix; reuse them.
> - Arrows move by visual row (bubbles/textarea default).
> - Out: list-item continuation rules, code-fence wrap,
>   `Reflow`→manual demotion (separate or follow-up).
>
> **Open — brainstorm before coding.**
> - `ansi.Wordwrap`: does it preserve span styling across the
>   break, or is an ANSI-aware splitter needed? Read its source.
> - Cursor block: `insertCursorBlock` runs pre-wrap. After
>   word-aware re-prefix, does the cursor land on the right
>   visual row with styling intact?
> - `applyAnnotationsToLine` runs pre-wrap — confirm visual wrap
>   doesn't tear ANSI runs.
> - Scroll-off works in source rows; with visual wrap, does the
>   3-row margin need a source→visual map?
> - Long unbreakable tokens (URLs > budget): match
>   `mailcompose/seed.go wrapWords` — own visual row, overflow OK.
>
> **Approach.** Brainstorm, write a plan at
> `docs/superpowers/plans/2026-05-19-catkin-soft-wrap.md` naming
> bubbles analogues per `bubbletea-conventions.md`, verify with
> a live tmux capture (Gold Nugget Mailchimp reply is good
> stress). Pass-end ritual applies.
