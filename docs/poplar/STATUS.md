# Poplar Status

**Current pass:** Pass 12 — `.ics` viewer (#37). Pass 11
landed List-Unsubscribe; #36 closed.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 9z | Scaffold through bubble adoption (ADRs 0001–0182) | done |
| 10a | Outbox delivery controls — undo send (#35 part 1; ADR-0183) | done |
| 10b | Schedule send + sidebar Outbox (#35 part 2; ADR-0184) | done |
| 11 | List-Unsubscribe (#36) | done |
| 12 | `.ics` viewer (#37) | pending |
| 13 | Search (#38) | pending |
| 14 | First-run wizard (#27) + OAuth refresh + config template (#29) | pending |
| 15 | Polish II — popover dim (#14) + items surfaced during 10–14 | pending |
| 16 | **v0.9.0 prep** — feature freeze, docs sweep, README, tag | pending |
| Beta soak | Bug-fix releases; data formats frozen; features queue on `1.1` | pending |
| v1.0.0 | Tag when soak settles | pending |
| 1.1 | Neovim companion (#6); raw RFC822 (#21); other post-beta | post-beta |

## Next starter prompt (Pass 12)

> **Goal.** `.ics` calendar invite viewer (BACKLOG #37) — detect
> `text/calendar` / `application/ics` parts in incoming messages,
> parse them, and render a chip + popover in the viewer. Display
> only; no RSVP or CalDAV integration in scope.
>
> **Scope.** New `internal/icalendar/` package wrapping
> `github.com/arran4/golang-ical` (MIT). Parse the first
> `VEVENT` from the part bytes; expose a `ParseInvite` function
> returning a value type with summary, start/end times, location,
> organizer, and attendee count. Wire into the body-fetch Cmd
> alongside attachments; result rides on `reader.BodyLoadedMsg`.
> Viewer chip row gains an invite chip (extend the attachment
> chip pattern from ADR-0138). `Enter` on the chip opens an
> overlay (`InvitePopover`) with the full event detail. No key
> binding beyond `Enter` to open and `Esc`/`q` to close; overlay
> follows the existing modal cascade.
>
> **Settled (do not re-brainstorm):** Display only — no RSVP,
> no CalDAV write. Library choice: `arran4/golang-ical`.
> `internal/icalendar/` is the new domain package. The chip
> extends the existing chip row (ADR-0138 pattern). Overlay
> uses `ModalShell`.
>
> **Still open — brainstorm these:** How to handle multi-event
> `.ics` files (show first, or list?); time formatting (local
> tz vs UTC?); what to show when parsing fails (hide chip or
> show error chip?); chip label text.
>
> **Approach.** Brainstorm the open questions, write a plan at
> `docs/superpowers/plans/YYYY-MM-DD-ics-viewer.md`, then
> implement. Standard pass-end checklist (~8 tasks).
