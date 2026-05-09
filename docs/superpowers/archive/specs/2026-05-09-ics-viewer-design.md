---
title: .ics calendar invite viewer (Pass 12)
status: accepted
date: 2026-05-09
---

## Context

Incoming mail occasionally carries `text/calendar` /
`application/ics` parts — meeting invites (METHOD=REQUEST),
publish dumps (METHOD=PUBLISH), cancellations (METHOD=CANCEL),
and replies. Today poplar surfaces these as opaque attachment
chips: the user can save the file but cannot tell what the
event is without opening it externally. BACKLOG #37 asks for an
inline viewer.

Scope is display-only. RSVP, CalDAV write-back, and calendar
import are out of scope; v1 has no calendar surface and
post-1.0 may grow one. The pass adds reading capability so
recipients can see *what* an invite says without leaving
poplar.

Prior art across major clients (researched 2026-05-09):

- **Outlook / Thunderbird / Evolution**: structured card between
  headers and body; Summary + When + Where always visible;
  Organizer + Attendees collapsed; Accept/Tentative/Decline
  buttons.
- **Apple Mail (macOS)**: compact banner below body with
  Summary + Date/Time + Location; everything else hands off to
  Calendar.app.
- **mutt / neomutt**: no native handling; `.mailcap` pipes
  through user-supplied scripts that emit plain text.
- **aerc** (the closest TUI peer): built-in `calendar` awk
  filter renders Summary + Organizer + Start/End + Location +
  Attendees + Description as a formatted plain-text block in
  the pager. No card, no popover, no interactive surface.
  RSVP exists as `:accept` / `:decline` commands but the
  *display* is text.
- **alpine, K-9, Geary**: no native rendering as of 2026.

The TUI norm is plain text in the pager, not an interactive
component. Outlook-grade cards are GUI-client convention; for
~5 lines of always-relevant data, an interactive popover is
overhead the user has to learn. Poplar follows aerc's
precedent: render an inline block in the viewer layout,
always visible when an invite is present.

## Decision

### Domain package

A new package `internal/icalendar/` wraps
`github.com/arran4/golang-ical` (MIT). The package exports:

```go
type Invite struct {
    Summary       string
    Start, End    time.Time
    Location      string
    Organizer     string
    AttendeeCount int
    Method        string  // REQUEST, PUBLISH, CANCEL, …
    Recurrence    string  // humanized one-liner; "" if none
}

func ParseInvite(b []byte) (Invite, error)

var ErrNoEvent = errors.New("icalendar: no VEVENT in part")
```

`ParseInvite` parses the part bytes and returns the **first**
VEVENT only. Multi-VEVENT files (METHOD=PUBLISH dumps with
several events) surface only the first. ADR records the
choice so a future pass can revisit if users hit it.

`Recurrence` is a humanized rendering of `RRULE` for the
common cases: `FREQ=DAILY`, `WEEKLY`, `MONTHLY`, `YEARLY` at
any `INTERVAL`. Anything fancier (BYDAY, BYMONTHDAY, COUNT,
UNTIL, EXDATE) yields `""` — the v1 line drops the
recurrence field rather than printing iCalendar source.
Empty Summary falls back to `"(no title)"`. Empty Location,
Organizer, Recurrence simply omit the corresponding row.

`ParseInvite` returns `ErrNoEvent` on empty input, no
VCALENDAR, no VEVENT, or any underlying parser error. Callers
distinguish "no invite" from "parse failed" only by logging
intent — both branches hide the block.

### Body-fetch wiring

The existing body-fetch `tea.Cmd` (`internal/ui/cmds.go`)
already walks parts to extract attachments. Extend it:

- During the part walk, detect parts whose Content-Type is
  `text/calendar` or `application/ics` (case-insensitive,
  parameter-tolerant).
- Read the part bytes (capped at the existing attachment-size
  ceiling — invites are tiny; reuse the cap).
- Call `icalendar.ParseInvite`. On success store an `*Invite`;
  on `ErrNoEvent` or any error, store nil.
- Pass through `reader.BodyLoadedMsg`:

  ```go
  type BodyLoadedMsg struct {
      UID         mail.UID
      Blocks      []content.Block
      Links       []string
      Attachments []mail.Attachment
      Unsub       content.Unsubscribe
      Invite      *icalendar.Invite  // new; nil when absent
  }
  ```

The calendar part also stays in the attachments slice so the
chip row still lists it for save-to-disk. Two surfaces, one
underlying part — invite block reads the parsed structure;
attachment chip exposes the raw file.

### Reader layout

`reader.Model` gains `invite *icalendar.Invite` plus the
cached layout fields:

```go
inviteRow    string
inviteHeight int
```

Layout becomes panel → invite → chips → body, top to bottom.
Body height:

```go
bodyHeight := max(1,
    v.height - lipgloss.Height(v.panel) - v.inviteHeight - v.chipHeight)
```

The invite row joins the assembled view in `View()` between
panel and chip row, with the existing chip-row blank-line
separator preserved when both are present.

Render:

```
📅  <Summary, bold, FgBright>
    <When>
    Where: <Location>
    Organizer: <Organizer>
    <N attendees>
    Repeats: <Recurrence>
    [CANCELLED]
```

- Icon: `📅` (fancy) / `[i]` (simple) from
  `uicore.FancyIcons`/`SimpleIcons` — new `Calendar` field on
  the `IconSet` struct, asserted by the existing
  Na/N + SPUA-range tests.
- `When` rendering: local timezone via `time.Local`.
  - Same-day end:
    `Wed 2026-05-14, 3:00 PM – 4:00 PM`
  - Cross-day end:
    `Wed 2026-05-14, 3:00 PM – Thu 2026-05-15, 4:00 PM`
  - All-day (start has zero time component AND end is exactly
    24h later, the iCal all-day convention):
    `Wed 2026-05-14 (all day)`
  - Zero start: render `When: (no start time)` so the row
    still anchors the block.
- `[CANCELLED]` prefix appears on its own row above the
  Summary when `Method == "CANCEL"` (case-insensitive),
  styled `ColorWarning`. The rest of the block renders
  normally; this lets the user see *what* was cancelled.
- Width math: every row goes through `ansix.Width` (icon
  prefix is SPUA when fancy) and `ansix.Truncate` to fit the
  width budget.
- Width budget: full panel width (uncapped, like the panel
  header). Long Summaries truncate with `…`; the auxiliary
  rows truncate too rather than wrapping — keeps the block at
  a predictable height (max 7 rows: optional `[CANCELLED]` +
  Summary + When + Where + Organizer + Attendees + Recurrence).

### Styles

A new section in `internal/ui/reader/styles.go`:

```go
InviteIcon       lipgloss.Style  // AccentPrimary
InviteSummary    lipgloss.Style  // FgBright bold
InviteField      lipgloss.Style  // FgNormal
InviteCancelled  lipgloss.Style  // ColorWarning bold
```

No new palette slots; reuses existing theme tokens. Update
`docs/poplar/styling.md` with the four-row mapping.

### What's *not* in scope

- **No chip in the chip row.** The block is the surface; a
  chip would be redundant signaling. The attachment chip for
  the underlying `.ics` file remains (save-to-disk path).
- **No popover, no overlay, no new key binding.** Always
  visible when present.
- **No RSVP.** No Accept/Tentative/Decline. No iTIP REPLY
  generation. No CalDAV write.
- **No multi-event listing.** First VEVENT only.
- **No description rendering.** Invite descriptions are
  long-form prose that belongs in the body if anywhere; the
  block is a header card. If the sender included a useful
  description they typically also wrote it in the email body.
- **No timezone display beyond local.** Outlook's dual-tz
  affordance is GUI-client polish; aerc and Apple Mail both
  show local only.
- **No recurrence beyond simple FREQ+INTERVAL.** BYDAY etc.
  drop silently.

## Consequences

**Unlocks.** Recipients can read meeting invites without
leaving poplar. Pairs naturally with a future RSVP pass
(post-1.0) and a future `internal/calendar/` surface — the
`Invite` value type is the input shape.

**Forecloses.** The block is layout-fixed; if a future pass
wants RSVP buttons, they go in the same band (extra row(s)
appended). A chip+popover migration is still possible later
but the block-style baseline is what users will know.

**Pass shape.** ~6 tasks, one ADR. Fits the 8–12 task pass
budget with margin. No cross-package schema changes; no
backend changes; one new domain package and one new
`reader.Model` field.

**Risk.** `arran4/golang-ical` is the de-facto Go iCalendar
library (used by go-jmap and others) but it's not a stdlib
package. The `Invite` value type provides the seam: if the
library proves inadequate post-v1 we can swap parsers
without changing UI code. No vendoring; standard go.mod
dep.

**Test surface.**
- `internal/icalendar/` table-driven tests with real-world
  fixtures: Google-issued REQUEST, Outlook-issued REQUEST
  with TZID, recurring weekly, all-day event, CANCEL,
  multi-event PUBLISH (first-VEVENT-only assertion),
  malformed input (`ErrNoEvent`).
- `reader_test.go` golden tests for layout: invite present /
  absent / cancelled / with attachments / without
  attachments. Covers the height-arithmetic invariant.
- Live tmux capture at 80×24 and 120×40 against a real
  Fastmail invite delivered to `geoff@907.life`.

## Tasks

1. `internal/icalendar/` — `Invite`, `ParseInvite`,
   `humanizeRRULE`, `ErrNoEvent`, table-driven tests.
2. Body-fetch Cmd extension in `internal/ui/cmds.go`;
   `BodyLoadedMsg.Invite` field.
3. `reader.Model.invite` + `renderInviteBlock` + layout
   integration; reader styles slots; `uicore.IconSet.Calendar`.
4. Update `docs/poplar/styling.md` with the new style rows;
   update `docs/poplar/wireframes.md` viewer wireframe to
   show the band.
5. Update `docs/poplar/invariants.md` — Viewer §: the
   "Chip row sits between header panel and body" fact
   becomes "Invite block + chip row sit between header panel
   and body, in that order, both optional."
6. ADR-0186 — `.ics` invite viewer (display-only,
   aerc-aligned inline block, first-VEVENT-only). Capture
   tmux renders. Run pass-end checklist.
