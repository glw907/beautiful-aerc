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
