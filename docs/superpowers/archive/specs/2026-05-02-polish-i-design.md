# Polish I — Pass 8.3

**Date:** 2026-05-02
**Status:** approved

## Problem

Pass 8.3 closes three behavior bugs that bite at the 80×24 polish bar
and on plain-text-formatted mail:

- **#23** — HTML→plain-text fuses words across inline element boundaries
  ("Safari toSafari 18.6", "to:Dave_99504@yahoo.comThanks,Dave Johnson").
- **#26** — At 80 cols the message-list pane truncates senders to 22
  cells and squeezes subject to 6 cells. Date column is `Mon
  2006-01-02` (14 cells) regardless of width. Sidebar floor is 24
  cells per ADR-0096; sender/subject/date budgets don't adapt.
- **#9** — While a search filter is committed and the viewer is open,
  `n`/`N` should walk the filtered row set, fetch the next message
  body, and behave well under real backend latency.

## Scope

In: `internal/filter/html.go` (#23); `internal/ui/msglist.go`,
`internal/ui/account_tab.go`, `internal/ui/sidebar.go`,
`internal/ui/date_format.go`, plus a new layout-mode helper (#26);
`internal/ui/account_tab.go` + `internal/ui/cmds.go` cancel-prior
plumbing (#9).

Out: body cache (Pass 8.4b), eager prefetch (Pass 8.4b), full
chrome rework (separate ADR — see "Future levers" below).

## #23 — HTML word-fusion

### Decision

Insert a single space at every inline-element boundary in the
HTML→plain-text converter, then run a final whitespace-normalization
pass that collapses runs of `\s+` into a single space. The boundary
insertion happens during tag stripping; the normalization absorbs
the cosmetic cost.

### Rationale

The "post-process to re-introduce spaces around fused alphanumeric
runs" alternative requires regex-style detection of fused tokens,
which is heuristic and prone to false positives on URLs, paths,
and identifiers that legitimately contain mixed alphanumerics
without surrounding whitespace. Boundary insertion at parse time
is precise: every `<br>`, `<a>`, `<span>`, `<b>`, `<i>`, `<em>`,
`<strong>`, `<code>`, etc. boundary becomes a space; the
normalizer collapses cases where adjacent inline tags would have
produced multiple spaces.

### Test plan

Table-driven tests in `internal/filter/html_test.go` covering:
- `<a href="x">link</a>text` → `link text`
- `Safari to<br>Safari 18.6` → `Safari to Safari 18.6`
- `to:<a>Dave_99504@yahoo.com</a><br>Thanks,<br>Dave Johnson` →
  `to: Dave_99504@yahoo.com Thanks, Dave Johnson` (note: also
  verifies the colon-suffix case from the original bug)
- `<p>para 1</p><p>para 2</p>` → preserves block-level paragraph
  separation (existing behavior)
- `the   minimum` (HTML with `&nbsp;` etc.) → `the minimum` (single
  space after normalization)

Add the two original bug-trigger emails as integration fixtures
under `internal/filter/testdata/` and assert on the parsed output.

## #26 — Responsive message-list and sidebar

### Decision

A continuous formula governs sidebar width, sender width, and
fixed-overhead cells; date and flag columns are gated by discrete
thresholds. Three coherent UI tiers emerge — Spartan / Intermediate
/ Full — but the formulas (not the tiers) are the implementation
primitive.

```
sidebar = clamp(round(14 + (W - 80) * 0.2),  14, 30)
sender  = clamp(round(22 + (W - 80) * 0.125), 22, 32)
flags   = (W >= 90)
date    = 0   if W < 90
        = 3   if 90 <= W < 100   // compact relative ("now", "5m", "Apr", "'24")
        = 5   if W >= 100        // short absolute ("04-30", "3:04p")
icons   = (sidebar >= 20)         // sidebar icons; ≈ W >= 108
fixed   = 8 + (4 if flags else 0)
subject = (W - sidebar - 2) - fixed - sender - date
```

`round` is half-away-from-zero (Go's `math.Round`): 0.5 rounds up
to 1, −0.5 to −1. Determines sidebar/sender exact values at
edge widths.

| Tier | W | What's on |
|------|---|-----------|
| **Spartan** | 80–89 | Name + subject. No flags, no date, no sidebar icons. |
| **Intermediate** | 90–107 | + flags + date (compact 90–99, short 100+). No sidebar icons. |
| **Full** | 108+ | + sidebar icons. All chrome on. |

Worked table:

| W | sidebar | icons | flags | sender | date | **subject** |
|---|---------|-------|-------|--------|------|-------------|
| 80  | 14 | off | off | 22 | 0 | **34** |
| 89  | 16 | off | off | 23 | 0 | 40 |
| 90  | 16 | off | on  | 23 | 3 | **34** |
| 99  | 18 | off | on  | 24 | 3 | 40 |
| 100 | 18 | off | on  | 25 | 5 | **38** |
| 108 | 20 | on  | on  | 26 | 5 | **43** |
| 120 | 22 | on  | on  | 27 | 5 | 52 |
| 140 | 26 | on  | on  | 30 | 5 | 65 |
| 160 | 30 | on  | on  | 32 | 5 | 79 |
| 180 | 30 | on  | on  | 32 | 5 | 99 |

### Rationale

- **Sender slope (0.125)** captures three coverage cliffs from real
  Fastmail-Inbox sampling (n=2000): 22 cells → 86%, 28 cells → 92%,
  32 cells → 95%. Slopes hit each cliff at a natural width (22@80,
  28@128, 32@160).
- **Sidebar slope (0.2)** covers 14→30 over W=80→160. Floor 14
  (icons-off, label budget 8 = fits "Archive"=7); ceiling 30 (label
  budget 18 with fancy+unread, comfortable for nested customs).
- **Subject takes 67% of new width** (1 − 0.2 − 0.125). Appropriate:
  subject median is 39 cells, sender median is 17.
- **Threshold breakpoints** (90 / 100 / 108) are evidence-backed by
  research into actual terminal sizes (no hard data, but emulator
  defaults + btop's W=100 structural threshold + Windows Terminal's
  120×30 default + anecdotal modal-cluster of 113–120 cols all
  point to 100 / 120 as natural inflection points).
- **Compact 3-cell date format** (`now`, `5m`, `Apr`, `'24`) closes
  the asymmetry where W=90–99 has flags but no date. Pattern
  matches Slack / Twitter / iOS Mail compact mode.

### Compact-mode tradeoffs

- W < 90: no flag column → "answered" and "read+flagged" states
  inspectable in viewer only.
- W < 100: relative-only date (or none) → precise timestamp in
  viewer header.
- W < 108: no sidebar icons → role conveyed by folder name.

These are documented in the ADR (the new ADR supersedes ADR-0096).

### Implementation shape

1. **New file `internal/ui/layout.go`** — pure functions for the
   formula. Single source of truth:

   ```go
   type LayoutMode struct {
       Sidebar    int
       Sender     int
       Date       int  // 0, 3, or 5
       FlagColumn bool
       Icons      bool // sidebar icons
   }

   func ComputeLayout(termWidth int) LayoutMode { ... }
   ```

   Table-driven tests covering boundaries (79, 80, 89, 90, 99,
   100, 107, 108, 110, 159, 160) and far-out widths (220, 320).

2. **`MessageList`** consumes `LayoutMode` instead of the current
   `mlSenderWidth` / `mlDateWidth` / `mlFlagWidth` constants. The
   `mlFixedWidth` constant becomes a derived value in the row
   builder. Sender, date, and flag-column rendering branch on the
   `LayoutMode`.

3. **`Sidebar`** consumes `LayoutMode.Sidebar` (replaces existing
   `sidebarWidthFor`) and `LayoutMode.Icons` (drops the icon
   block when off).

4. **`AccountTab`** computes `LayoutMode` once per
   `WindowSizeMsg` and threads it into both children. No layout
   work in `View()`.

5. **New file `internal/ui/date_format.go`** gains
   `formatRelativeDateCompact(t, now) string` — always returns 3
   cells right-padded. `displayDate` selects format from
   `LayoutMode.Date`:
   - `0`: returns `""` (caller must skip date column entirely)
   - `3`: `formatRelativeDateCompact`
   - `5`: existing `formatRelativeDate` truncated/adapted to 5
     cells (`04-30` / `3:04p`)

   Note: full ISO `Mon 2006-01-02` (14 cells) is dropped entirely
   from the codebase — luxury that subject cells need more.

### Test plan

- Unit: `ComputeLayout` table tests across all breakpoints.
- Unit: `MessageList.View` and `Sidebar.View` honor each LayoutMode.
- Live: tmux capture at 80×24, 90×24, 100×24, 120×40, 160×40.
  Verify rendered widths match the worked table.
- Live: resize from 80 to 220 and back, confirm no visual glitches
  at transitions (W=89↔90, 99↔100, 107↔108).

## #9 — Viewer n/N filter-walk + cancel-prior

### Decision

The filter-walk is essentially free — `MoveCursor → moveBy`
already skips `hidden=true` rows, and the search filter sets
`hidden=true` on non-matching rows. Confirm under live backend
with a test, no new logic.

For prefetch: implement **cancel-prior** semantics. When a body
fetch is in flight and the user presses `n`/`N` again, cancel
the in-flight Cmd before issuing the new one. Avoids wasted
backend round-trips when the user mashes through messages.

Eager prefetch (read-ahead ±1) is **deferred to Pass 8.4b**, when
the body cache lands and prefetched results have a durable place
to live.

### Rationale

- Cancel-prior is orthogonal to caching: it's a Cmd-cancellation
  pattern that survives intact when the body cache arrives in
  Pass 8.4b.
- Wins on the actual pain case: slow IMAP backends where each
  body fetch is 1–3 seconds. Mashing `n` 5 times with cancel-prior
  is one round-trip total, not five queued.
- A viewer-local `map[UID]ParsedBody` (eager prefetch without a
  cache) duplicates the invalidation problems Pass 8.4b is
  designed to solve (folder change, mark-read, IDLE update).
  Solving them twice is wasteful.

### Implementation shape

1. **`AccountTab` gains `bodyFetchCancel context.CancelFunc`**
   stored alongside the in-flight UID. On a new `openMessage`
   call (from `Enter`, `n`, or `N`), if the cancel func is
   non-nil, call it before issuing the new Cmd.

2. **`loadBodyCmd`** signature gains a `context.Context`
   parameter. The Cmd respects context cancellation — if
   cancelled before the backend call returns, it returns no
   message (or an internal `bodyCancelledMsg` that the
   AccountTab discards).

3. **Stale-result guard** already exists (compares
   `viewer.CurrentUID()` against `bodyLoadedMsg.UID`). With
   cancel-prior, late results from a cancelled fetch should
   never arrive, but the guard remains as defense-in-depth.

4. **No filter-walk code changes.** Add a test confirming that
   `MoveCursor` skips hidden rows and that n/N during a committed
   filter advances within the visible-filtered set.

### Test plan

- Unit: existing `TestMessageList_MoveByMixedHidden` (or add
  one) confirms `moveBy` skip behavior.
- Unit: `TestAccountTab_NextMessage_DuringFilter` sets up a
  filter that hides every other row, opens viewer on first
  visible, presses `n`, asserts cursor advances to next visible
  (not next-by-index).
- Unit: `TestAccountTab_CancelPrior` uses a controllable mock
  backend that blocks `loadBody` until signalled. Open
  message A; press `n` to advance to B before A's body
  arrives; assert A's fetch context was cancelled and only B's
  body resolves.
- Live: tmux capture against Fastmail JMAP, confirm `n` mashing
  on slow folder produces one final body fetch.

## Future levers (deferred, not in 8.3)

- **Drop decorative chrome at narrow widths** — right border, top
  frame, possibly the sidebar/pane divider. Up to +2 horizontal
  cells and +1 vertical row at 80×24. Reopens ADRs 0025–0030 (the
  chrome family) — deserves its own pass + ADR. Log as backlog
  item.
- **Eager prefetch ±1** — Pass 8.4b. Becomes natural once body
  cache exists.
- **Wide-tier full ISO date** — explicitly rejected. The 9 cells
  to display weekday + year aren't worth the complexity; row
  position conveys recency.

## Sources of evidence

- Sender / subject distributions: 2000 messages from
  geoff-login@907.life Fastmail Inbox, sampled 2026-05-02 via JMAP.
- Terminal-size research: emulator defaults (kitty, alacritty,
  wezterm, iTerm2, Windows Terminal, GNOME Terminal, Konsole,
  Ghostty), Windows Terminal default 120×30, btop's W≥100
  structural breakpoint, glow's 120-cell render cap, NAO NetHack
  observed terminal sizes (cluster 113–120, tail 180–220), WezTerm
  Discussion #2311. No hard survey data exists for actual runtime
  terminal sizes.
