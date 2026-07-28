# ADR-0009: CalDAV calendar engine with a JSCalendar-shaped model

Date 2026-07-27. Status: accepted (Phase 4); the RSVP mechanism
branch resolves when the calendar-scoped probe runs.

## Context

Calendar is first-class v1 (C4 directive). Fastmail's calendar API
is CalDAV until JMAP calendars leaves the RFC Editor queue
(verified in queue 2026-07-20); the upgrade must be a seam, not a
rewrite (C11). iTIP has no Go library; recurrence overrides are
caller-side everywhere.

## Decision

The event model is poplar's own, shaped toward JSCalendar
vocabulary (start, duration, structured recurrence overrides,
participants keyed by address) so the JMAP upgrade thins to a
backend swap plus mapping. Each event retains its raw ICS blob;
edits round-trip the parsed component and rewrite only the CA-4
modeled properties, preserving unknown properties verbatim. The
`occurrence` table indexes the expanded recurrence window (13
months back, 18 forward, sliding), with poplar-owned
EXDATE/RDATE/RECURRENCE-ID splicing over the RRULE library. TZID
resolution: IANA match → vendored CLDR Windows-zone table →
float-to-local with visible notice. `internal/calendar/itip`
hand-rolls REQUEST/REPLY/CANCEL parse and construction with
RFC 5546 SEQUENCE discipline; outbound iTIP mail rides the
outbox. The RSVP answer path carries both mechanisms (CalDAV
PARTSTAT write triggering the server reply, or poplar sending
iMIP itself); the pending Fastmail probe picks, and the
double-send fixture guards the losing branch.

## Alternatives considered

- **Model events as parsed iCal property bags**: welds the store
  to RFC 5545's shape and makes the JMAP upgrade a data
  migration; the raw-blob-plus-model hybrid gets fidelity and the
  seam.
- **Expand recurrence at query time**: recurrence expansion with
  overrides is the most defect-prone code in every surveyed
  client; a materialized, rebuildable window makes agenda reads
  trivial and QA-2-safe.
- **khal's split store** (files plus shadow cache): two stores to
  reconcile; rejected with ADR-0002's reasoning.
- **Wait for JMAP calendars**: the draft has been "nearly done"
  for years; v1 does not wait on the RFC Editor queue (C11 bets
  forward through the seam instead).

## Consequences

Grid and agenda views read the occurrence index only. Unknown
property preservation makes poplar a good CalDAV citizen with
other clients on the same account. The iTIP layer is pure and
fixture-tested; its mail-side assembly is ordinary outbox work.

## Revision 2 (2026-07-27, post-review)

- **The queryable columns now implement the JSCalendar claim**:
  `start_local, tzid, duration_secs, is_all_day, is_floating` on
  the event row; occurrences carry `start_local` and `local_date`
  beside the UTC sort instants, so all-day and floating events
  render on the correct local date (revision 1 stored UTC
  instants, which cannot represent either).
- **The window is a cache with a miss path**: out-of-window
  navigation or date-jump triggers a bounded on-demand expansion
  with a visible progress state; the boundary is a named UI
  state; the slide runs in the background bulk lane, never on
  startup; a tzdata update invalidates and re-expands.
- **The RSVP mechanism is a `Capabilities` fact** resolved by the
  pending probe (requirements section 16 item 1), so the branch
  survives a second backend; poplar-sends-iMIP is the default if
  the probe stays blocked.
- **The JMAP upgrade delta, named**: the raw ICS blob and
  unknown-property round-trip lose their purpose, `href`/`etag`
  go null, `uid` survives, the occurrence index and the
  JSCalendar-shaped columns are unchanged. Named risk: JSCalendar
  2.0 is still a draft, so the target vocabulary can move.
