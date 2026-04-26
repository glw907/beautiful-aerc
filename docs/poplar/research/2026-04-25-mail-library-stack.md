# Mail library stack: aerc fork vs. direct-on-libraries

**Pass 2.9 research deliverable.** Decides the question raised by
BACKLOG #10: keep the aerc fork (ADR-0002) or migrate to a
library-first stack. Outcome: ADR-0075 — adopt direct-on-libraries
(approach B), superseding ADR-0002, 0006, 0008, 0010, 0012.

## Premise correction

The starter prompt (and BACKLOG #10) framed the open question as
"Go JMAP landscape is very thin — options if confirmed: drop
JMAP, hybrid, or write a Go JMAP client." That premise is wrong.

`git.sr.ht/~rockorager/go-jmap` (v0.5.3, Feb-2025) covers JMAP
Core (RFC 8620), Mail (RFC 8621), MDN (RFC 9007), S/MIME
(RFC 9219), and Push/EventSource. Submodules under `core/` and
`mail/` cover blob, push, push/subscription, email, mailbox,
thread, identity, emailsubmission, mdn, vacationresponse, and
searchsnippet. **It is already in our `go.mod`** — aerc's own
JMAP worker is built on it. The "no JMAP client" concern that
motivated keeping the aerc fork doesn't apply.

The remaining question is whether to keep the ~10 kLOC of aerc
*worker glue* that wraps `rockorager/go-jmap` and `emersion/go-imap`
in aerc's Action→Message channel idiom, or call those libraries
directly from a synchronous `mail.Backend` implementation.

## Library inventory

### JMAP

| Library | Latest | Coverage | Adoption signal |
|---|---|---|---|
| `git.sr.ht/~rockorager/go-jmap` | v0.5.3 (2025-02) | Core + Mail + MDN + S/MIME + Push/EventSource | Powers aerc's JMAP worker. Already a poplar dep. Pre-1.0 but stable in practice. |
| `mikluko/jmap` | 2026-02 | Core only | Newer, narrower, no traction. |
| `foxcpp/go-jmap` | 2019 | draft Core | Abandoned. |
| `josephburnett/go-jmap` | 2023 | draft Core | Abandoned. |
| `rhyselsmore/go-jmap` | 2026-03 | core + contacts + mail | Low star count, unclear adoption. |
| emersion | — | none | No JMAP repo exists. |

### Emersion mail/DAV stack

| Library | Latest | Stars | Role |
|---|---|---|---|
| `emersion/go-imap` v1 | v1.2.1 (2022-05) | 2.3k | IMAP4rev1 client/server. Maintenance mode but feature-complete with IDLE built in. v2 has been "in development" for ~3 years with no stable release. |
| `emersion/go-message` | v0.18.2 (2025-01) | 444 | Streaming RFC 5322 / MIME / Content-Disposition. The de-facto Go MIME library. |
| `emersion/go-smtp` | v0.24.0 (2025-08) | 2.0k | ESMTP client/server. Needed Pass 9 (compose + send). |
| `emersion/go-sasl` | active | 103 | PLAIN, LOGIN (deprecated), ANONYMOUS, EXTERNAL, OAUTHBEARER. **Gap: no XOAUTH2** — Gmail's required mech. Aerc's `auth/xoauth2.go` (~80 LOC) fills it; we vendor that snippet under `internal/mailauth/`. |
| `emersion/go-webdav` | v0.7.0 (2025-10) | 473 | WebDAV + CalDAV + CardDAV. Covers post-1.0 contacts and any future calendar work. |
| `emersion/go-vcard` | active | 126 | vCard 3.0/4.0. Natural pair with go-webdav. |

`emersion/go-imap-sortthread` is the current source for IMAP SORT
and THREAD; we already depend on it (used by aerc's IMAP worker
and we'd keep it post-rewrite).

The emersion stack is the only Go mail library family with
end-to-end coverage: IMAP, SMTP, MIME, SASL, WebDAV/CardDAV/CalDAV,
vCard. Releases land every 3-6 months across the family. Maintained
by one developer (Simon Ser / emersion) with consistent style and
long-term track record (`go-imap` started in 2016). Used by
`maddy`, `hut`, aerc, and most other Go mail projects.

`rockorager/go-jmap` is maintained by Tim Culverhouse (vaxis
author, active in the aerc community). Used in production by aerc
itself. Single-maintainer risk is real but the library shape is
mechanical (JMAP wire types + JSON), low-churn, and we'd be one of
several consumers.

## Fork composition (10,011 LOC)

Poplar's `internal/mailworker/` (forked from aerc on 2026-04-09)
breaks down by what each subtree contributes versus what the
underlying libraries already provide.

| Subtree | LOC | What it is | Replaceable by |
|---|---|---|---|
| `worker/types/`, `worker/lib/`, `worker/middleware/`, `worker/handlers/`, `worker/` | ~2,000 | Aerc's async Action→Message dispatcher, foldermap middleware, gmail middleware | Nothing — vanishes when `mail.Backend` calls libraries synchronously. Foldermapper is replaced by poplar's own `mail.Classify`. |
| `worker/jmap/` + `worker/jmap/cache/` | ~2,740 | Wraps `rockorager/go-jmap` calls in the Action/Message idiom. Includes a gob-on-disk blob/state cache. | Direct calls to `rockorager/go-jmap`. Cache deferred to a later pass (offline-first is not a v1 goal). |
| `worker/imap/` + `extensions/` | ~3,360 | Wraps `emersion/go-imap` v1 in the Action/Message idiom. IDLE loop, observer, seqmap. Includes Gmail-specific X-GM-EXT (~300 LOC). | Direct calls to `emersion/go-imap` v1. IDLE loop reimplemented as ~150 LOC; xgmext vendored verbatim under `internal/mailimap/xgmext/`. |
| `models/` + `rfc822/` | ~780 | Aerc's `MessageInfo`/`Folder` wire types + body structure parser. | Mostly redundant: poplar already owns `mail.MessageInfo` / `mail.Folder`. Body structure parsing is in `emersion/go-message`. |
| `auth/` + `keepalive/` + `xdg/` + `log/` + `lib/` + `parse/` | ~730 | Cross-cutting: oauth bearer, TCP keepalive, XDG paths, structured logger, header parsing. | Most can be deleted — poplar has its own log and config. Vendor `auth/xoauth2.go` (~80 LOC) and `keepalive/` (~32 LOC) under `internal/mailauth/`. |

**Audit signals:**

- Zero cherry-picks from upstream since the fork (16 days). The
  fork-burden cost hasn't started accruing — Pass 3 hasn't begun.
- No notmuch / maildir / mbox traces — already pruned at fork
  time. The fork is already poplar-shaped.
- The fork's actual value-add over the libraries is: aerc's worker
  idiom, IMAP IDLE plumbing, XOAUTH2 helper, JMAP push glue, gob
  blob cache. The first of those is friction (we then bridge it
  back to sync); the rest are <500 LOC of vendorable snippets.

## Approaches considered

**A. Keep the fork as-is.** Pass 3 is unblocked today. Trades
~10 kLOC of aerc-shaped indirection for not having to write the
direct backends. Bets on aerc remaining a healthy upstream — it
is, but aerc is a TUI project (no API stability promise), so
cherry-pick churn scales with their internal refactoring.

**B. Direct-on-libraries rewrite.** Replace `internal/mailworker/`
+ `internal/mailjmap/` with synchronous `internal/mailimap/` (on
`emersion/go-imap` v1) and `internal/mailjmap/` (on
`rockorager/go-jmap`). Both implement `mail.Backend` directly —
no Action channels, no pump goroutine, no foldermapper indirection
(poplar's `mail.Classify` already covers that role). Vendor
`auth/xoauth2.go` and `keepalive/` as small utilities. Estimated
2-4 passes worth of work. End state: ~3 kLOC of poplar-owned
direct code instead of ~10 kLOC of aerc-shaped indirection.

**C. Hybrid — keep the fork's IMAP, rewrite JMAP direct.** Saves
~3 kLOC immediately by ripping out `worker/jmap/`. Defers the IMAP
rewrite. End state: two architectural styles in the same binary,
which is worse than either pure option. Postpones the same
question we already have evidence to answer.

## Recommendation

**Approach B.** The premise that motivated the fork (ADR-0002:
"Go JMAP landscape too thin to depend on libraries") is no longer
true given `rockorager/go-jmap`'s current coverage. Trading
~10 kLOC of foreign indirection for ~3 kLOC of direct code that
matches `mail.Backend`'s synchronous shape gives us:

- code we own, can read, can debug, can test;
- no async→sync pump goroutine (the JMAP adapter's reason to
  exist disappears);
- alignment with the strongest library momentum in Go mail —
  emersion's stack covers SMTP (Pass 9), CardDAV/vCard (post-1.0
  contacts), and CalDAV without us forking anything else;
- `rockorager/go-jmap` as the JMAP layer keeps us on the same
  foundation as aerc itself, just consuming it as a library
  instead of inheriting aerc's wrapper.

The execution is a Pass 3 rewrite. There is no hybrid carve-out
in v1 — we commit fully to direct-on-libraries and delete
`internal/mailworker/` as part of Pass 3.

## Consequences for downstream passes

- **Pass 3** changes shape. Was "wire prototype to live backend
  via existing JMAP adapter." Becomes "write
  `internal/mailimap/` + new synchronous `internal/mailjmap/` on
  vendor libraries; delete `internal/mailworker/`; wire prototype
  to live JMAP backend." Likely splits into Pass 3 (JMAP direct,
  Fastmail live) and Pass 8 (IMAP direct, Gmail live), since
  Gmail/IMAP isn't needed until Pass 8 anyway.
- **Pass 9** (compose + send) gains `emersion/go-smtp` as the
  obvious SMTP submission path. No additional library research
  needed there.
- **Post-1.0 contacts** (`project_contacts_sidebar_microhighlight`)
  gains `emersion/go-webdav` + `emersion/go-vcard` for free —
  same library family.
- ADR-0058 (Pass 2.5b-7 single-binary pivot) is unaffected: this
  pass changes how we implement the mail layer, not the binary
  shape.
