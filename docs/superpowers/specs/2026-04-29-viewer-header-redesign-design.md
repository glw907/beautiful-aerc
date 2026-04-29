# Viewer Header Redesign

**Date:** 2026-04-29
**Status:** approved

## Problem

The message viewer's header reads as a labeled key/value block with a thin
dim rule below it, on the same surface as the body. The "this is a header
region" cue is too weak — the eye doesn't immediately register the boundary
between header and body. Several design iterations (overline, accent
labels, blank-row variations) didn't land. The goal is to redesign the
header so it reads as a visually distinct zone with **subject as a gentle
hero** and minimum noise, while preserving the single-surface aesthetic of
the right panel.

## Decisions reached during brainstorming

The header treatment uses **indentation as the primary cue** for the
header-vs-body boundary. No bg tint, no full-width rule, no overline. The
subject's prominence comes from typography alone (`FgBright` + bold), and
the metadata reads as a quieter aside indented inward.

### Layout (top-to-bottom in the viewer pane)

```
                                ← (no top blank — subject sits on row 1)
Updates to our partner ads setting control
                                ← blank
  from    Google Ads
  to      geoff@907.life
  cc      Cathy Wright          (only when present)
  date    Fri, Apr 24 2026 10:19 AM
                                ← blank
                                ← second blank (extra breathing room before body)
Dear Google User,

At Google, we believe you should always be in control of your data.
…body continues…
                                ← bottom blank (existing pane-bottom padding)
```

### Specific rules

1. **Top blank removed.** The subject occupies the first content row of
   the viewer pane. This puts it on the same row as the sidebar's
   `geoff@907.life` account label — the two panels share a single
   "title row" elevation. Saves two vertical rows vs the previous
   layout.

2. **Subject** — `t.SubjectTitle` style (`FgBright` + bold, defined in
   `internal/theme/palette.go`). Rendered at the pane's existing 1-cell
   left padding (column 1). Wraps with the existing `wrap()` helper at
   `contentWidth = v.width - 1`. **No overline.**

3. **Metadata block** — From / To / Cc / Bcc / Date, each rendered as a
   single row when present, omitted entirely when empty. Labels are
   lowercase (`from`, `to`, `cc`, `bcc`, `date`), no colons, no bold,
   `FgDim`. Values use the existing `HeaderValue` style (`FgBase`). The
   block is indented **+2 cells** inward from the subject — column 3
   relative to the pane's left edge.

4. **Label column width.** The label column stays 8 cells wide (the
   existing `headerKeyColWidth` constant) so values align. With the +2
   indent and 8-cell label column, values land at column 11.

5. **Two blank rows between metadata and body.** The first absorbs
   the existing "metadata trailing blank"; the second is the new
   "extra breathing room before body content" row.

6. **No rule.** The full-width `─` separator is dropped — the
   indentation step + double blank line is the boundary.

7. **Body indentation unchanged** — body remains at column 1 (flush
   with the subject), so subject and body share a vertical alignment
   and the metadata reads as the only inset element.

8. **Bottom blank row of the viewer pane stays** (existing
   pane-bottom padding).

### Body height accounting

`Viewer.layout()` reserves blank rows the way it does today:

- 0 rows above the rendered headers (no top blank now)
- 2 rows below the rendered metadata (the blank pair before body)
- 1 row at the bottom of the pane

Total reserved: **3 rows** (down from current 3 — net unchanged).
`headerHeight` is `lipgloss.Height(v.headerStr)` where `v.headerStr`
includes the subject row, the blank between subject and metadata, and
the metadata rows. The trailing blank-pair before body is emitted in
`View()` (not in `RenderHeaders`), so the math stays clean.

## Components touched

### `internal/content/render.go`

- `RenderHeaders(h, t, width)` rewritten to emit the new layout:
  - subject as the first line (no overline, no leading blank)
  - blank
  - metadata rows with +2-cell indent and lowercase labels
  - **no trailing rule**
- `renderHeaderKey` — change to lowercase the input key inside the
  helper (so callers can keep passing the canonical capitalization);
  strip the colon. The label column width constant
  (`headerKeyColWidth`) keeps the value alignment.
- `renderHeaderScalar` and `renderHeaderAddresses` — prepend a 2-space
  indent to each emitted line.

### `internal/theme/palette.go`

- Drop `SubjectOverline` (unused after this change).
- `SubjectTitle` stays (`FgBright` + bold).

### `internal/ui/viewer.go`

- `View()`: remove the leading `blank` from `JoinVertical`. Keep
  `blank` between header and body, add a second `blank` so the gap is
  two rows. Keep the trailing pane-bottom `blank`.
- `layout()`: `bodyHeight` stays at `v.height - headerHeight - 3`.

### `docs/poplar/styling.md`

- Drop the `SubjectOverline` row from the surface table.
- Update the "Message viewer" section's prose to match the new
  layout (no overline, no rule, indented metadata).

### Tests

- Existing viewer tests assert "From:" appears in output — needs
  update to assert "from " (lowercase, no colon).
- Header order test stays the same (Subject before From in render
  output; that already holds).
- New test: assert metadata rows are indented (e.g. line containing
  "from " starts with at least 2 leading spaces inside the rendered
  string).

## Out of scope

- The Cc/Bcc plumbing through `MessageInfo` and the JMAP backend
  (already shipped earlier in this session).
- Date formatting from `SentAt` (already shipped).
- Subject-as-title hoisting (already shipped — this redesign refines
  the surrounding chrome, not the title hoisting).
- Any change to body rendering, footnotes, or link handling.

## Acceptance

The viewer renders the Kris Willing email and the Google Ads email
with the layout shown in the ASCII mockup above. The subject sits on
the same screen row as the sidebar's `geoff@907.life` account label
in a 120×40 terminal. Metadata block indents inward, body sits flush
with the subject. No bg tint, no rule, no overline. `make check`
passes.
