# Phase 3 grounding survey

**Date:** 2026-07-27
**Phase:** Re-founding Phase 3 (requirements). Charter:
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`.
**Purpose:** Ground the requirements spec in observed practice before
drafting it, per the Phase 3 directive: Protonmail as the polished
incumbent, the actively used TUI clients for the terminal state of the
art, the prior research corpus as mined input, and live evidence for
the speed numbers. A mid-phase directive added the calendar surveys:
calendar is v1 scope, so the incumbent calendar products and the
backend contract were surveyed to the same depth.

**Method:** Seven research dispatches ran against live sources on
2026-07-27 (current docs, changelogs, issue trackers, releases, and
reviews; no findings from model memory alone), plus a live JMAP probe
of the target Fastmail account. Two dispatches mined the archived
2026-05-29 functional spec and the 2026-04/05 research corpus. This
document distills the decision-relevant findings; the requirements
spec (`docs/superpowers/specs/2026-07-27-poplar-requirements.md`)
consumes them.

## The competitive picture

**No terminal client is JMAP-native, and the gap is real.** meli's
JMAP backend is five years old and still second-class: its maintainer
does not dogfood it, the README rates it "functional" (below "full"),
push over EventSource remains unimplemented (poll-only against
Fastmail, issue open since January 2025), and JMAP header caching is
missing where the IMAP backend has it. himalaya's `io-jmap` library is
architecturally clean but thinly exercised. alpine has no JMAP at all,
and its live maintenance effort goes to defending IMAP OAuth against
Google and Microsoft churn, pain a JMAP-native client never buys.
aerc's JMAP backend is the most invested (async, cached, label
semantics), yet aerc still treats HTML mail as a disabled-by-default
w3m filter and ships no undo. A client that builds on JMAP's actual
advantages, session-based delta sync, EventSource push, one JSON API,
occupies ground none of the surveyed clients holds.

**Proton Mail's weaknesses are instructive because they are
self-inflicted.** Its encryption model forces heuristic subject-line
threading that misgroups unrelated mail, and it splits search into
metadata operators versus a separate opt-in local body index, the most
criticized behavior in every 2026 review surveyed. Poplar has neither
constraint: it threads on References per RFC 5322 and searches one
local full-text index. Proton's desktop app also still lacks offline
mode, so poplar shipping offline read and triage in v1 beats the
polished incumbent on its own turf, not a strawman. What Proton gets
right and poplar should copy: an inline calendar-invite card with
one-click accept/tentative/decline that writes the calendar and
replies to the organizer in a single action, per-recipient memory for
the sending identity on reply, uniform toast-with-undo on every
triage action, and a search grammar deeper than `from:`/`has:`.

**The Pine lineage still teaches.** alpine's always-visible,
mode-scoped key-hint footer is cited by name, across decades of
reviews, as the reason its learning curve stays shallow; the footer
never shows a key that is not currently legal. Its Select-then-Apply
bulk model (build a working set by query, run one action across it)
reappears in meli's `select` command and neomutt's tag-then-`;`
idiom: three mature clients converge on query-driven bulk triage as
the terminal norm. alpine's in-app configuration (Setup screens with
inline help, no hand-authored dotfile before first use) is the
onboarding bar; meli (hand-edit TOML first) and himalaya v2 (wizard
that prints config to stdout) are the friction cases. neomutt is the
cautionary pole for poplar's opinionated stance: nothing is bound by
default, HTML needs hand-written mailcap, and 2026 commentary
recommends it only "when inheriting an existing configuration."

**Compose and contacts are open lanes.** Every surveyed TUI delegates
address completion to an external binary (`carddav-query`, `abook`,
`khard`) or lacks it entirely; none does inline autocomplete in the
To: field from a live local store. himalaya v2 removed interactive
compose entirely in favor of a piped external tool. An integrated
markdown compose surface with native autocomplete outdoes the whole
field without inventing anything exotic.

**One live competitor to watch:** `himalaya-tui` (ratatui, JMAP-
capable, in-app composer) is pre-v0.1 as of July 2026 and explicitly
aims at the aerc/mutt/alpine space. Re-check it at each phase gate.

## Field norms the requirements adopt

The prior research corpus (the 2026-05-29 gap analysis and the
2026-04/05 norm surveys) plus the live surveys establish these as
universal norms, and the spec adopts them as requirements rather than
re-litigating them:

- Single-key triage from the list; bulk operations act on the full
  matching set, never only the visible page.
- An undo window for reversible triage, with permanent deletion as a
  distinct, confirmed, non-undoable act.
- Special-folder discovery by JMAP role (RFC 8621 guarantees at most
  one mailbox per role), with archive and delete as distinct verbs.
- Threading by server thread identity with a References walk as
  fallback; a message without references is a thread of one.
- Remote images never load automatically; the render shows a
  placeholder and loads on explicit request.
- Signatures materialize into the editable buffer at compose open;
  nothing is appended silently at send.
- Identity on reply matches the delivered-to address.
- Drafts are server-canonical with last-write-wins; the local store
  is a fast edit buffer, not the record.
- Sync resumes from a persisted watermark (the JMAP state token);
  push jumps ahead of any backfill queue; body backfill fetches
  newest first under an explicit throttle.
- Trash retention defaults to off; emptying trash is a y/n confirm
  with a count, never a typed confirmation.

Two findings from the mining pass are corrections, not norms: the
archived spec named no logging seam (it contradicts the standing
"every user-visible error reaches the log" invariant) and no
onboarding connect-probe (a regression from the archived client,
which shipped one). The spec fixes both. The mining inventory also
flagged charset/MIME decoding (RFC 2047, base64, quoted-printable,
legacy charsets) as never specified anywhere; unhandled, it renders
list mail as mojibake. The spec names it.

## Speed evidence

The latency evidence report grounds the quality attributes. The
defensible thresholds: 100 ms is the canonical "feels instant" line
(Card/Moran/Newell perceptual fusion; Nielsen), and practitioner
evidence (Dan Luu's camera measurements, recent ACM input-latency
work) shows keyboard-heavy users perceive degradation well below it,
so poplar aims under 50 ms keypress-to-paint against a terminal layer
that itself consumes 2 to 13 ms. SQLite FTS5 at one million rows
measures 140 to 200 ms per query in the closest available benchmark,
so the 100k-message search targets carry headroom. Three evidence
gaps mean three targets ship as provisional, to be replaced by a
Phase 4 spike measurement against a real large archive: no published
keypress-to-paint benchmark exists for any bubbletea app, no local
mail-search benchmark exists at the 100k-to-1M scale, and no numeric
JMAP-push-latency study exists (the JMAP-over-IDLE advantage is
architectural, not measured).

## Calendar

**The backend contract is CalDAV, not JMAP, for now.** Fastmail's
developer page states it plainly: calendars are CalDAV until the JMAP
calendars specification is finalized. That specification
(draft-ietf-jmap-calendars, revision 27) is through IESG approval but
held in the RFC Editor queue pending JSCalendar 2.0. The object model
is stable, so the upgrade path is designable now, but v1 speaks
CalDAV. The workable Go stack exists and is maintained:
`emersion/go-ical` (RFC 5545), `emersion/go-webdav/caldav`
(transport; no scheduling helpers), and `rrule-go` (recurrence
expansion). Timezone resolution from VTIMEZONE to IANA zones is the
known hazard every library leaves to the caller.

**RSVP has two paths and Fastmail collapses them.** A webmail RSVP
click updates the server event and the server sends the iTIP reply to
the organizer. Fastmail auto-adds invites from known contacts to the
default calendar without setting a participation status, so the
user's answer is the missing act, exactly the act poplar's reader
performs. Whether a third-party CalDAV participation-status write
triggers Fastmail's server-side reply is standard behavior elsewhere
(RFC 6638) but undocumented at Fastmail: it is the first Phase 4
probe. If it does not fire, poplar sends the iMIP reply itself
through its own outbox, which the archived spec already sketched.

**The incumbent floor is modest and keyboard-friendly.** Proton and
Fastmail converge on day/week/month views with single-key switching
and a single-key jump to today; Fastmail adds an agenda list and a
natural-language date jump (`g`, accepting "last wed"), both patterns
that translate to a terminal better than any mouse-first mini
calendar. Fastmail's per-event free/busy toggle, its separate default
reminders for timed versus all-day events, and the full three-way
scope prompt on recurring-event edits (Proton's partial version is a
documented pain point) round out the floor worth shipping. Neither
incumbent ships a year view. Booking pages, smart scheduling, and
two-way sync to third-party services are deferred product surfaces.

**TUI prior art validates the shape and leaves the integration
open.** ikhal's two-pane month-grid-plus-agenda with vim movement is
the validated terminal calendar layout; calcurse warns against
homegrown sync; no mature Go/bubbletea calendar exists. Mail-side,
the only precedent is "open the other program" (aerc plus ikhal,
mutt mailcap into khal); a unified reader-RSVP-calendar flow inside
one client is unclaimed.

## The target account

A live JMAP probe of the target Fastmail account (2026-07-27) fixes
the v1 envelope: 14 mailboxes, flat, the seven standard roles
(including `scheduled` for send-later) plus seven custom folders, and
roughly 36k messages. The account runs folders-mode, not labels-mode.
The session advertises `urn:ietf:params:jmap:contacts`, so contacts
sync over JMAP without CardDAV. No snoozed-role mailbox exists, which
reads as no snooze usage to preserve. The spec sizes its performance
envelope at 100k messages, roughly three times the current mailbox.

## Open items for Phase 4 probes

1. Does a CalDAV participation-status write trigger Fastmail's
   server-side iTIP reply, and how does it interact with recurring
   events? (Needs a calendar-scoped credential; the current API token
   has no calendar scope.)
2. Fastmail's draft behavior over JMAP: autosave cadence observed
   from the web client, and body-update semantics (`Email/set` versus
   destroy-and-import).
3. JMAP `Identity` behavior for alias auto-selection, verified
   directly rather than inherited from the gap analysis.
4. Free/busy query support over CalDAV at Fastmail.
5. The three provisional speed targets (keypress-to-paint, search at
   scale, push convergence), replaced by spike measurements against a
   real large archive.

## Sources

The seven dispatch reports (Protonmail, aerc/neomutt,
alpine/meli/himalaya, latency evidence, functional-spec mining,
field-norms mining, Proton/Fastmail calendar UX, and the calendar
backend contract) are session artifacts; their load-bearing claims
and URLs are reproduced above and in the requirements spec. The
committed prior corpus remains under `docs/poplar/research/` (the
2026-05-29 gap analysis and the 2026-04/05 norm surveys) and
`docs/superpowers/specs/` (the 2026-05-29 functional spec).
