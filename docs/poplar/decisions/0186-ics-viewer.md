---
title: .ics calendar invite viewer — display-only inline block
status: accepted
date: 2026-05-09
---

## Context

Mail occasionally carries `text/calendar` / `application/ics`
parts: meeting requests, publish dumps, cancellations, replies.
Today poplar surfaces these as opaque attachment chips — the
recipient can save the file but cannot tell what the event is
without leaving poplar. BACKLOG #37 asked for an inline viewer.

Prior art across the matrix: Outlook / Thunderbird / Evolution
render structured cards with Accept/Tentative/Decline buttons;
Apple Mail shows a compact banner with handoff to Calendar.app;
mutt has no native handling; **aerc** — the closest TUI peer —
renders Summary + Organizer + Start/End + Location + Attendees
as a formatted plain-text block in the pager. RSVP exists in
aerc as `:accept`/`:decline` commands but the *display* is text.
alpine, K-9, Geary: no native rendering as of 2026.

The TUI norm is plain text in the pager. Outlook-grade
interactive cards are GUI-client convention; for ~5 lines of
always-relevant data, an interactive popover is overhead the
user has to learn.

## Decision

- New domain package `internal/icalendar/` wrapping
  `github.com/arran4/golang-ical` (MIT). Exports `Invite`
  value type, `ParseInvite([]byte) (Invite, error)`, and
  `ErrNoEvent`.
- **First VEVENT only.** Multi-event PUBLISH dumps surface only
  the first; the next pass can revisit if users hit it.
- **Display only.** No RSVP, no iTIP REPLY generation, no CalDAV
  write-back. `Invite` is the seam for a future RSVP /
  `internal/calendar/` pass; the value type is the input shape.
- Surface is an inline block between the viewer header panel and
  the chip row, always visible when an invite is present. No
  chip in the chip row (the underlying `.ics` part still appears
  there for save-to-disk via the existing attachment path). No
  popover, no overlay, no new key binding.
- `[CANCELLED]` row above Summary on `Method == "CANCEL"`
  (case-insensitive), styled `ColorWarning`.
- Times rendered in `time.Local`. Same-day, cross-day, all-day,
  and zero-start cases each have a distinct format. Outlook's
  dual-tz affordance is GUI-client polish; aerc and Apple Mail
  show local only.
- `Recurrence` humanizes only `FREQ` + `INTERVAL` for DAILY /
  WEEKLY / MONTHLY / YEARLY. Anything containing BYDAY,
  BYMONTHDAY, COUNT, UNTIL, EXDATE etc. drops to `""` and the
  Repeats row is omitted — better silence than printing
  iCalendar source.
- Wiring: the body-fetch Cmd's part walk gains a `text/calendar`
  / `application/ics` branch; bytes feed `ParseInvite`; the
  resulting `*icalendar.Invite` rides on `reader.BodyLoadedMsg`.
  Reader holds it as state; `renderInviteBlock` is a pure
  function from `(*Invite, IconSet, Styles, width) → (rows,
  height)`. Layout becomes panel → invite → chips → body, with
  `bodyHeight = height - panelHeight - inviteHeight - chipHeight`.
- Icon: `📅` (fancy, `nf-md-calendar_check` U+F0E68 in the
  Nerd Font SPUA-A range) / `i` (simple, single ASCII rune,
  width 1).
- Four new reader styles: `InviteIcon` (AccentPrimary),
  `InviteSummary` (FgBright bold), `InviteField` (FgBase),
  `InviteCancelled` (ColorWarning bold). No new palette slots.

## Consequences

**Unlocks.** Recipients read meeting invites without leaving
poplar. Pairs naturally with a future RSVP pass and a future
`internal/calendar/` surface — `Invite` is the input shape.

**Forecloses.** The block is layout-fixed. A future RSVP pass
appends rows in the same band; chip+popover migration is still
possible but the block-style baseline is what users will know.

**Risk.** `arran4/golang-ical` is the de-facto Go iCalendar
library but not stdlib. The `Invite` value type is the seam —
swap parsers without touching UI code if the library proves
inadequate post-v1. Standard go.mod dep, no vendoring.

**Schema.** None. No backend changes, no cache changes. One new
domain package; one new `reader.Model` field; one new
`BodyLoadedMsg` field.

**Test surface.** Table-driven tests in `internal/icalendar/`
with seven `.ics` fixtures (Google REQUEST, Outlook with TZID,
recurring weekly, all-day, CANCEL, multi-event PUBLISH first-
only assertion, malformed). Reader golden tests cover invite
present / absent / cancelled / all-day / cross-day / recurring /
truncation + the height-arithmetic invariant.
