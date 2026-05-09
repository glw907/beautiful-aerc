# Pass 9z — Bubble adoption

The Pass 9y verdict
(`docs/poplar/research/2026-05-08-bubble-consolidation-verdict.md`)
named two carry-forward items from the harvest catalogs. A
re-scan of the Top-3 sections surfaced one more genuine adoption
that the verdict didn't escalate. Pass 9z lands that one and
explicitly defers the others under named flip conditions.

## The four patterns surfaced

The harvest catalogs each carried a Top-3 section naming the
patterns most worth incorporating. The honest cross-catalog
tally is four (list-family had three; chrome-family one; table/
form none). Status of each:

| # | Pattern | Status | Where |
|---|---------|--------|-------|
| 1 | Filter-match rune highlight (`lipgloss.StyleRunes`) | already adopted | 9x.1 — movepicker non-cursor rows |
| 2 | `Inline(true).Inherit(other)` for layered styles | **adopt now** | movepicker cursor row |
| 3 | Per-state styles named by combined state | defer (verdict holds) | sidebar |
| 4 | `shouldAddItem` progressive-truncation loop | defer (verdict holds) | helppopover |

## Adopting #2 — `Inline(true).Inherit(other)` for cursor-row filter-match

`internal/ui/movepicker/model.go:315` skipped match-rune
highlight on the cursor row to dodge the SGR-composition
problem (an outer `Cursor.Render` wrap gets cleared by the
internal SGR resets `lipgloss.StyleRunes` emits between styled
spans, leaking partial coloring across the row). The catalog
named the cleaner solution: bake the outer style into every
StyleRunes span via `Inline(true)` on the unmatched style and
`Inherit(matchStyle)` on the matched style. Every rune carries
the cursor fg; matched runes additionally carry the match
underline; the outer wrap handles only marker + pad.

`Cursor` (fg only) and `Match` (underline only) compose without
conflict. The non-cursor branch keeps a zero-value base style so
unmatched runes stay unstyled.

This closes the UX gap where a filtered cursor row hid the *why*
of its match.

## Deferred (verdicts hold)

### `bubbles/help` `shouldAddItem` progressive truncation (chrome-family §3)

Helppopover (`internal/ui/helppopover/model.go:226`) falls back to
a whole-box `tooNarrow` string. The whole-box swap is the right
shape for a modal grid where the named-group layout is the
component's identity — once a group disappears, the popover is no
longer recognizably itself.

**Flip condition:** helppopover gains a progressive-narrow mode
that drops the rightmost group before falling back. Not on the
table.

### Per-state combined-state style naming in sidebar (list-family §3)

`internal/ui/sidebar/styles.go:12-14` declares three named slots
(`SidebarSelected` / `SidebarFolder` / `SidebarUnread`); the
fourth combination (cursor on unread) composes via
`uicore.ApplyBg`. The catalog's bar was "state combinations grow
past 3"; sidebar still has three primary states, so the named-
slot rewrite buys nothing today.

**Flip condition:** a fourth primary state appears (e.g., stale
or syncing) that combines with cursor and unread.

## Adjacent cleanups

None pending. All three catalogs' cross-cutting sections converge:
`uicore.ListBodyRows` extraction stays at three call sites with
three different reserve constants — below the four-consumer bar.
Both contacts/list and messagelist are full-pane lists with a
different layout path; they don't add a fourth modal-list
consumer. The dead `contacts.Styles.Border` field already landed
in 9x.3.

## Outcome

One small adoption (movepicker cursor-row highlight). No new ADR
— ADR-0182 is the controlling decision and the deferral
conditions live in the verdict doc. STATUS marks Pass 9z done.
Next pass is Pass 10 — outbox delivery controls.
