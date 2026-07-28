# ADR-0010: go-webdav transport, go-ical parse, bake-off fallback

Date 2026-07-27. Status: accepted (Phase 4); the serialization
bake-off runs before the calendar build pass.

## Context

C9 requires maintained dependencies at current releases. The
survey found: go-webdav/caldav is the de facto Go CalDAV client
(sync-collection, conditional PUT; no scheduling, no free/busy);
its API surfaces go-ical types, which makes go-ical the
transport-forced parse layer; go-ical has no tagged release and a
long-open VTIMEZONE weakness; golang-ical tags releases and has
better METHOD/PARTSTAT ergonomics; rrule-go is dormant with a
DST-fixing fork.

## Decision

Transport: go-webdav/caldav, pinned by pseudo-version if
untagged. Parse: go-ical at the transport boundary (forced), with
poplar's own TZID resolution layer above it (the library's
weakness is exactly the hazard CA-1 already assigns to poplar).
Serialization for iTIP construction: go-ical first; if the
bake-off (Fastmail-exported fixtures, CA-4 modeled property set,
byte-fidelity on unknown properties) shows it cannot round-trip,
golang-ical serves the iTIP layer only, and the two never mix in
one code path. Recurrence: the DST-fixing rrule fork as default
candidate, decided by a DST fixture set. Free/busy and scheduling
REPORTs, if CA-12's seams ever need them, are raw WebDAV requests
poplar issues itself.

## Alternatives considered

- **golang-ical everywhere**: abandons the go-webdav type
  integration and re-implements the transport mapping; its
  co-maintainer solicitation is a maintenance flag of its own.
- **Hand-rolled iCalendar parser**: RFC 5545's folding, escaping,
  and parameter quirks are exactly the wheel C9 forbids
  reinventing while a maintained-enough wheel exists.
- **Upstream rrule-go**: three years dormant with known DST bugs;
  the fork exists because of them.

## Consequences

Two small verification gates precede the calendar build pass: the
bake-off and the DST fixture run. Both are cheap, both produce
committed fixtures the build inherits, and the fallback path is
named in advance so a bake-off failure costs a decision update,
not a redesign.
