# Phase 4 library and prior-art survey

**Date:** 2026-07-27
**Phase:** Re-founding Phase 4 (technical design). Charter:
`docs/superpowers/specs/2026-07-19-poplar-refounding-charter.md`.
**Purpose:** The exhaustive prior-art and library survey constraint C9
requires before any design decision: for every subsystem, inventory
the existing libraries, tools, and prior art at their current
releases, verified live. Poplar builds nothing a maintained
dependency already does well, and no version inherits a legacy pin.
**Method:** Eleven research dispatches ran against live sources on
2026-07-27 (GitHub releases, pkg.go.dev, IETF datatracker, project
docs and issue trackers; no findings from model memory alone). One
dispatch probed the live Fastmail JMAP API of the target account,
closing spec section 16 items 3 and 4. The design doc and ADRs
consume this survey; each pinned version below is a candidate the
design confirms, not a decision by itself.

## The candidate stack

| Subsystem | Pick | Version (verified 2026-07-27) |
|---|---|---|
| Toolchain | Go | 1.26 (patch 1.26.5) |
| Store engine | modernc.org/sqlite | v1.54.0 (SQLite 3.53.3, FTS5 in) |
| JMAP | git.sr.ht/~rockorager/go-jmap | v0.5.3 |
| CalDAV transport | emersion/go-webdav (caldav) | v0.7.0 |
| iCalendar | emersion/go-ical or arran4/golang-ical | bake-off (see below) |
| Recurrence | teambition/rrule-go or xyedo/rrule fork | bake-off |
| MIME parse | jhillyerd/enmime/v2 | v2.4.1 |
| MIME assembly | emersion/go-message | v0.18.2 |
| Auth-Results | emersion/go-msgauth (authres, dmarc) | v0.7.0 |
| HTML DOM | golang.org/x/net/html (+ goquery) | v0.57.0 / v1.12.0 |
| HTML→md baseline | JohannesKaufmann/html-to-markdown/v2 | v2.5.2 |
| TUI framework | charm.land/bubbletea/v2 | v2.0.8 |
| Components | charm.land/bubbles/v2 | v2.1.1 |
| Styling | charm.land/lipgloss/v2 | v2.0.5 |
| Reader markdown | charm.land/glamour/v2 | v2.0.1 |
| Markdown AST | yuin/goldmark | v1.8.5 (not the v2 beta) |
| Syntax highlight | alecthomas/chroma/v2 | v2.27.0 |
| Forms | charm.land/huh/v2 | v2.0.3 (if adopted) |
| TUI golden tests | charmbracelet/x/exp/teatest | latest exp tag |
| Keyring | zalando/go-keyring | v0.2.8 |
| D-Bus | godbus/dbus/v5 | v5.2.2 |
| Notifications | esiqveland/notify | v0.14.0 |
| Clipboard | OSC 52 via bubbletea v2 | built in |
| XDG paths | adrg/xdg | v0.5.3 |
| Single instance | gofrs/flock | v0.12.1 |

Everything in the table satisfies `CGO_ENABLED=0`. The bake-offs and
open risks per subsystem follow.

## Store engine and search

modernc.org/sqlite is the pick: pure Go, BSD-3, actively sponsored
(Tailscale), 3,500+ importers, and FTS5, WAL, RTree, and JSON1
compiled in by default. Its known caveat is concurrency discipline:
connections open full-mutex, and the standard mitigation is a
single-writer connection, a separate read pool, and `busy_timeout`.
The design must name that discipline explicitly (QA-2 requires reads
that never block behind sync writes).

ncruces/go-sqlite3 (wasm-based, v0.35.2) is faster in some published
read benchmarks but carries three disqualifying risks today: its FTS5
module is a separate pre-1.0 package with near-zero adoption, the
runtime just migrated wazero→wasm2go, and an open WAL corruption bug
on Windows argues against trusting it under the QA-6 kill harness.
Re-evaluate in a future cycle. mattn/go-sqlite3 is cgo and out.
Non-SQLite stacks (KV store plus bleve) would hand-build exactly the
transactional index-consistency FTS5 provides and only pay off at
10-100x the target corpus.

FTS5 under modernc covers the needed surface: unicode61 and trigram
tokenizers, prefix queries, bm25/snippet/highlight, and
external-content tables so bodies are not stored twice. Migrations:
a hand-rolled `schema_version` runner over `go:embed` SQL files (the
goose embedded pattern without the dependency).

## JMAP

go-jmap (rockorager, MIT, v0.5.3) is the only maintained,
RFC-current Go JMAP library, proven in aerc. Coverage: RFC 8620 core
complete (session, batching, back-references, blobs), RFC 8621 mail
complete (Mailbox, Email, Thread, EmailSubmission, Identity), and an
EventSource push client in `core/push`. It has no contacts or
calendar support, and it is pre-1.0, so the design pins an exact
version. The split it validates is the one poplar wants: the library
supplies wire types and transport; the sync engine (delta
orchestration, caching, reconnect policy) is poplar's own.

JMAP contacts: no Go library types RFC 9610/9553 objects. Poplar
writes its own ContactCard structs; the live probe (below) confirmed
the shape Fastmail serves.

## CalDAV, iCalendar, recurrence, iTIP

The transport is emersion/go-webdav's caldav package (v0.7.0, Oct
2025): sync-collection (RFC 6578) with sync tokens, ETag-conditional
PUT. It has no scheduling (RFC 6638) and no free/busy support;
those, if needed, are raw REPORT requests poplar issues itself.

iCalendar is a two-way bake-off the design opens with. emersion/
go-ical fits the go-webdav ecosystem but has no tagged release and a
long-open VTIMEZONE weakness; arran4/golang-ical (v0.3.5, Apr 2026)
tags releases and has better METHOD/PARTSTAT ergonomics for iTIP
work but is soliciting a co-maintainer. The bake-off runs both over
Fastmail-exported fixtures against the CA-4 modeled property set.

Recurrence: teambition/rrule-go is dormant since 2023; the xyedo/
rrule fork fixes DST bugs. Either way, RECURRENCE-ID override
splicing is caller-side; no library does it. Windows-zone→IANA
mapping is a vendored, generated CLDR table (winianatz and wtz.go
are reference generators, not runtime dependencies), plus
`time/tzdata` embedding.

iTIP is hand-rolled everywhere; no Go library constructs
METHOD:REPLY with correct PARTSTAT or manages SEQUENCE. The best
reference implementation found is Python (purelymailcalendar's
calinvite: strip METHOD before CalDAV storage, preserve it for the
mail path, match replies back to PARTSTAT).

JMAP calendars stayed exactly where Phase 3 left it, verified on the
datatracker: draft-ietf-jmap-calendars-27 sits in the RFC Editor
queue (last touched 2026-07-20) behind JSCalendar 2.0
(draft-ietf-calext-jscalendarbis, still an active draft). CalDAV is
the only viable v1 transport; the one Go JSCalendar library is
hobby-stage. The upgrade seam stays a design obligation, not a
dependency.

## MIME, charsets, authentication results

Parse and assembly split across two libraries, the same split aerc
ships. enmime/v2 (v2.4.1, commits days old) parses inbound mail: it
is built for hostile input, with per-part defect accumulation
(`Envelope.Errors`) that maps directly onto poplar's
honesty-of-failure requirements, tolerant options for missing
boundaries and malformed Content-Type, and built-in charset
handling with a lying-label posture. emersion/go-message (v0.18.2,
quiet but stable, pinned by aerc's maintained fork) builds outbound
MIME, where poplar controls the input and its known
malformed-input issues do not apply. mnako/letters is an active
newer parser worth a corpus-diff spike as a second opinion, not a
primary.

Charset detection for lying labels: saintfish/chardet (ICU-derived,
stale but algorithmically stable) behind x/text's ianaindex for the
declared-label path; `ianaindex` can return nil for recognized but
unimplemented names, which must fall to the documented-default path,
never crash. format=flowed has no Go implementation anywhere;
whatever CO-5 needs is hand-rolled (the allowlist research below
argues it needs less than expected). Authentication-Results parsing
is emersion/go-msgauth v0.7.0, which added ARC support in 2024,
directly covering RD-10's mailing-list false-alarm criterion.

## HTML processing under the rule engine

The rendering pipeline stays poplar's own (C2); the survey confirms
the building blocks. x/net/html is the DOM layer (stay current: a
2026 CVE series patched Parse paths), goquery the selector layer for
rule authors. html-to-markdown/v2 (v2.5.2) is the baseline
conversion layer and the degenerate-case fallback; whether its
plugin hooks admit per-node interception or only whole-document
wrapping is a design-phase question to answer by reading it.
k3a/html2text backs the filtered-plain-text fallback mode
(`WithLinksInnerText` preserves link targets). go-shiori/
go-readability is deprecated; the maintained line is
codeberg.org/readeck/go-readability/v2, and any readability-style
extraction must be version-pinned hard because heuristic tuning
changes shift output between releases, a direct QA-7 determinism
concern. bluemonday (v1.0.26) serves only the open-in-browser export
path.

Two email-HTML diseases have no library cure and become named rules:
layout-table linearization (score tables by cell density, link
ratio, `role="presentation"`, nesting depth; the published
screen-reader heuristics are the prior art) and hidden-content
detection (a narrow inline-style visibility parser over
tdewolff/parse/v2/css for `display:none`, `mso-hide`, `font-size:0`;
full cascade resolution is explicitly out of scope).

## The Charm stack

bubbletea v2 is stable: v2.0.0 shipped 2026-02-24, v2.0.8 current,
v1 frozen. The import paths moved to `charm.land/*/v2`. The
design-relevant deltas: KeyPressMsg/KeyReleaseMsg replace KeyMsg;
the declarative `tea.View` struct replaces imperative screen-mode
commands; a new cell-buffer renderer; and bubbletea now owns all
terminal I/O. lipgloss v2 is pure and deterministic, which fits UX-3
exactly: no automatic terminal sniffing, explicit background
detection through `tea.BackgroundColorMsg`, themes authored as
functions of `isDark bool`. Kitty keyboard enhancements are opt-in;
poplar's modifier-free constraint (C8) means declining them keeps
input behavior identical across terminals, an ADR-worthy call.

glamour v2 (v2.0.1) fits the read path: OSC 8 hyperlinks, lipgloss
v2 wrapping, and `WithChromaFormatter` so the theme package drives
syntax colors. It renders whole documents, so the Catkin
live-markdown editor hand-rolls a goldmark-AST walker emitting
styled segments instead (goldmark v1.8.5; its v2 is beta and not a
day-one dependency). chroma v2.27.0 covers syntax and diff
highlighting. teatest (x/exp) plus x/exp/golden is the UI test
harness; experimental path, no serious alternative, adopt it.
Terminal graphics (kitty/sixel via rasterm or go-termimg) stay a
capability-gated SHOULD; no mainstream bubbletea app ships inline
images yet. Instructive apps at scale: crush (Charm's own, v2
reference), gh-dash (list-heavy navigation nearest poplar's shape).

## Platform integrations

| Integration | Linux | macOS |
|---|---|---|
| Keyring | zalando/go-keyring over D-Bus Secret Service, pure Go | library shells out to `/usr/bin/security`; blocked by C3, degrade by name to the ST-1 0600-file fallback |
| Notifications | esiqveland/notify over godbus, pure Go | no compliant path exists; degrade by name |
| Clipboard | OSC 52 via `tea.SetClipboard` (kitty chunks to 8MB; VTE drops >4096 codepoints; tmux needs passthrough config) | same OSC 52 path |
| Opener | `xdg-open` subprocess, the C3 carve-out | `open`, same carve-out |
| Paths | adrg/xdg (covers XDG_STATE_HOME; stdlib does not) | same |
| Single instance | gofrs/flock, `LOCK_EX\|LOCK_NB`; kernel releases on any death, no stale-lock handling | same API |

One assumption from Phase 3 fell: golang.design/x/clipboard went
cgo-free in v0.8.0 (purego on macOS, pure-Go X11 and Wayland
data-control on Linux). It is a live candidate for in-process
clipboard behind the OSC 52 primary, pending a spike on the gate
box (raw-mode coexistence, compositor coverage, and whether
purego's dlopen fits the project's static-binary definition). The
OSC 52 size ceiling needs a designed degrade state per ER-3; silent
truncation is disallowed.

## Toolchain

Pin Go 1.26 (`go 1.26` in go.mod; current patch 1.26.5, released
2026-07-07; the workstation's 1.26.3 needs a patch bump).
Load-bearing and stable in that line: testing/synctest (sync-engine
tests under virtualized time), os.Root (attachment and temp-file
sandboxing), the Green Tea GC as default (10-40% GC overhead
reduction, relevant to QA-2 tails), slog.NewMultiHandler, and the
waitgroup and hostport vet analyzers for the Phase 5 gate.
`B.Loop()` benchmarks plus benchstat are the perf-harness idiom.
CGO_ENABLED=0 checklist: pure-Go resolver reads resolv.conf (no
NSS), macOS cert verification works cgo-free via the
Security.framework wrapper, `_ "time/tzdata"` embeds zones.

## Outgoing HTML (CO-5 grounding)

The research settles the allowlist posture before the round-trip
probe: inline styles only, no `<style>` block (Gmail mobile strips
it), no class or id attributes (Gmail strips those in several
paths). Elements: p, h1-h6, strong, em, s/del, a[href], ul/ol/li,
blockquote, pre, code, hr, simple content tables, div/span as
containers. CSS: font-weight/style, text-decoration, color,
background-color (small spans only), margin, padding, border,
border-left, monospace font-family stacks, text-align,
white-space:pre as enhancement only (Outlook and Gmail mobile
ignore it; `<pre>` semantics carry the load). No position, no
negative margins, no floats. Dark mode: emit no body background,
tolerate inversion, optional `color-scheme` meta as best-effort.
Structure: doctype, `<meta charset="utf-8">`, single max-width
wrapper div.

The plain-text part drops format=flowed: Gmail has never honored
it, and 2022-era practitioner consensus abandoned it. Fixed 72-78
column wrapping at generation time, code fences preserved verbatim
and never reflowed. This satisfies CO-5's no-reflow criterion with
less machinery than a flowed encoder.

Generation shape: goldmark AST → a custom renderer emitting the
allowlisted inline-styled tags. No premade Go markdown→email-HTML
pipeline exists worth adopting (hermes is a template engine for a
different product shape; go-premailer solves a problem the
inline-first design avoids). Remaining CO-5 probe items: a real
send/receive diff in Gmail and Fastmail web for white-space
degradation, syntax-color spans under Gmail dark-mode inversion,
and Fastmail's actual sanitization allowlist read from a received
message's DOM.

## Live probe findings (target account, 2026-07-27)

The probe ran against the live Fastmail account (Identity/get,
ContactCard/get, and a create-update-destroy draft cycle in Drafts
only; nothing sent, account verified clean after).

**Session facts.** Mail accountId `u74694077` (a second internal
account is present and must be filtered by capability, confirming
the RFC 9610 note that contacts may live on a different accountId).
Limits: maxCallsInRequest 50, maxObjectsInGet 4096, maxObjectsInSet
4096, maxConcurrentRequests 10, maxSizeUpload 250MB.
`eventSourceUrl` is present and templated
(`.../jmap/event/?types={types}&closeafter={closeafter}&ping={ping}`),
so push is real; the historic 401 report against it still needs one
authenticated-stream check during the build.

**Identity (CO-3).** Identity/get returns only the four explicitly
configured compose identities, `mayDelete:false` on the primary, no
wildcard and no catch-all entry. It is a curated list, not an alias
directory. Reply-identity auto-selection therefore needs an
address-matching fallback (exact, then domain-suffix match against
delivered-to) beyond the Identity list; CO-3's wildcard-alias
criterion is met by poplar's matcher, not by server data.

**Contacts (CT-1).** Fastmail serves RFC 9610 only: ContactCard/get
works (186 cards; fields observed: name.components, name.full,
emails[].address/contexts/pref, phones, addressBookIds, kind, media,
uid, updated), and the legacy Contact/get returns unknownMethod.
The store models ContactCard natively.

**Draft semantics (CO-6, section 16 item 3).** The load-bearing
find: an Email/set update touching bodyValues/textBody returns
success (`updated: {id: null}`) while silently writing nothing; the
state token does not advance and the body is unchanged. Fastmail
neither applies nor rejects immutable-property writes. Naive
autosave code that checks only for errors would believe saves
succeeded while losing every keystroke. The working pattern,
confirmed atomic in one Email/set call: create the replacement
draft and destroy the original together (single state transition).
Email/changes coalesces a create+destroy pair inside one window
into nothing, which is correct for autosave (intermediate revisions
vanish from sync history) and must be documented so it is not read
as a missed-changes bug. Call latency ran 0.38-0.50s uniformly,
evidence for keeping JMAP entirely off the interactive path (C1).

## What poplar builds itself

The C9 inverse list, each item confirmed to have no maintained
Go dependency doing it well:

1. The rendering rule engine, fired-rule traces, and fact-inventory
   check (C2, by design), including the layout-table classifier and
   the hidden-content visibility parser.
2. The sync engine: delta orchestration over go-jmap types,
   watermark persistence, reconnect and backoff policy, resync on
   state reset.
3. JWZ References-walk threading as an isolated, I/O-free function
   (TH-1's fallback; notmuch's algorithm, no maintained Go port).
4. iTIP semantics: REPLY/REQUEST/CANCEL construction, PARTSTAT,
   SEQUENCE discipline (CA-4, CA-6).
5. RECURRENCE-ID override splicing over the RRULE expander (CA-1).
6. The Windows-zone→IANA vendored CLDR table and its regeneration
   script (CA-1).
7. RFC 9610/9553 ContactCard structs (CT-1).
8. The fixed-width plain-text wrapper with fence preservation
   (CO-5; format=flowed dropped).
9. The Catkin live-markdown renderer over goldmark's AST (CO-1).
10. The migration runner (schema_version over embedded SQL).
11. The reply-identity matcher over the curated Identity list
    (CO-3).

## Architecture lessons adopted from the field

From the prior-art dispatch (Mailspring, aerc, notmuch, Thunderbird
Panorama, mujmap/lieer, meli, khal/vdirsyncer), the lessons the
design doc encodes:

1. Fat-table hybrid schema (Mailspring): a JSON model column plus
   indexed scalar columns for exactly the fields the UI queries;
   schema evolution mostly avoids migrations.
2. Every mutation splits into an immediate local write plus a
   durable, retryable remote task row (Mailspring); the UI never
   waits, crash mid-send resumes.
3. Mint poplar's own primary keys; server ids are replaceable side
   columns (Panorama's 32-bit nsMsgKey debt is the cautionary tale).
4. Backend workers talk to the UI only through posted messages
   (aerc); network I/O never runs in the update loop.
5. Two watermarks per account, server state token plus local
   revision counter; server state expiry triggers a normal full
   resync, never an error path (mujmap/lieer).
6. Conflicts default to local-wins when the protocol cannot express
   what changed remotely (mujmap), bounded by SY-3's server-state
   ordering rule.
7. A small fixed set of long-lived workers, not per-folder
   goroutines (Mailspring).
8. Calendar stores raw ICS blob plus a normalized occurrence index
   in the same store; khal's separate shadow cache is the pattern
   to avoid.
9. FTS is rebuildable derived state (meli); index corruption is a
   non-event, never data loss.
10. JWZ threading runs only where the server supplies no thread id
    (notmuch/aerc).

## Open items carried into the design

1. CalDAV RSVP probe and free/busy probe: blocked on the
   calendar-scoped token (spec section 16 items 1 and 2).
2. The measurement spike replacing provisional QA-1/2/3 numbers,
   on the modernc engine with the fat-table schema shape.
3. go-ical vs golang-ical bake-off on Fastmail fixtures.
4. rrule-go vs xyedo/rrule verification against DST fixtures.
5. html-to-markdown v2 hook granularity (per-node vs whole-doc).
6. EventSource authenticated-stream check (the 2021 401 report).
7. CO-5 round-trip send/receive diff (needs the build, lands as a
   Phase 5 pass-3 artifact per the spec).
8. golang.design/x/clipboard spike on the gate box, and the OSC 52
   size-ceiling degrade design.
9. mnako/letters as a parse second-opinion fuzz target.

## Sources

The eleven dispatch reports are session artifacts; their load-bearing
claims and URLs are reproduced above. Primary sources: GitHub
releases and issue trackers for every pinned library, pkg.go.dev,
the IETF datatracker (RFC 9610, draft-ietf-jmap-calendars,
draft-ietf-calext-jscalendarbis), go.dev release notes, Gmail CSS
support documentation, caniemail.com, and the live Fastmail JMAP
session of the target account.
