# Email-client feature matrix

Cross-client inventory used to spot gaps in poplar's v1.0 surface.
Rows are features; columns are clients. The point is not to match
every cell — poplar is opinionated — but to surface anything that a
modern user expects and we'd be embarrassed to ship without.

**Legend.** `✓` full. `~` partial, via plugin, or via an external
tool the client steers (e.g. `$EDITOR`, `aspell`, `khard`,
`notmuch`). `—` absent. `⏳` planned, with the pass that lands it.
`?` uncertain — verify before relying on the cell.

**Columns.** mutt, aerc, alpine (= modern alpine / re-alpine),
**TB** (Thunderbird), **Apple** (Apple Mail), **Gmail** (web),
**Fast** (Fastmail web), **poplar**.

Mobile clients (K-9, Apple Mail iOS, Gmail mobile) are out of scope
— poplar is a desktop TUI; mobile UX patterns don't transfer.

## Core — v1.0 table stakes

These are the features a user will check for in the first ten
minutes. Any `—` in the poplar column here is a v1.0 risk.

| Feature                            | mutt | aerc | alp | TB | Apple | Gmail | Fast | poplar |
|------------------------------------|:----:|:----:|:---:|:--:|:-----:|:-----:|:----:|:------:|
| **Reading**                        |      |      |     |    |       |       |      |        |
| Folder list with classification    |  ~   |  ✓   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Message list                       |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Threaded / conversation view       |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| HTML email rendering               |  ~   |  ~   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Plain-text rendering               |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Attachments: view & save           |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Image-loading off by default       |  ✓   |  ✓   |  ✓  | ✓  |   ~   |   ~   |  ✓   |   ✓    |
| Search (basic, current folder)     |  ✓   |  ✓   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ~    |
| Search (full account / full-text)  |  ~   |  ~   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.8 |
| **Compose**                        |      |      |     |    |       |       |      |        |
| New / reply / reply-all / forward  |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9h  |
| Quote depth on reply               |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Drafts (saved & resumable)         |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Drafts cross-device sync           |  —   |  —   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Signatures (per account)           |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.4 |
| Spellcheck                         |  ~   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Address autocomplete (from history)|  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.1 |
| Attach files when composing        |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.5 |
| Plain-text compose                 |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ~   |  ✓   |   ✓    |
| HTML compose (any path)            |  —   |  ~   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| **Sending & accounts**             |      |      |     |    |       |       |      |        |
| IMAP                               |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| JMAP                               |  —   |  ?   |  —  | ~  |   —   |   —   |  ✓   |   ✓    |
| SMTP submission                    |  ~   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| OAuth / XOAUTH2 (Gmail, MS365)     |  ~   |  ~   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ~    |
| App passwords                      |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Multiple accounts                  |  ✓   |  ✓   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| **Triage**                         |      |      |     |    |       |       |      |        |
| Mark read / unread                 |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Flag / star                        |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Archive                            |  ~   |  ✓   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Move to folder                     |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Trash & permanent delete           |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Multi-select / batch ops           |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| Undo last triage                   |  —   |  ~   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| **Sync & offline**                 |      |      |     |    |       |       |      |        |
| Local cache / offline reading      |  ~   |  ~   |  —  | ✓  |   ✓   |   —   |  ~   |   ✓    |
| Outbox (queue sends offline)       |  ~   |  ~   |  —  | ✓  |   ✓   |   ~   |  ~   |   ⏳9g  |
| Push (IMAP IDLE / JMAP push)       |  ✓   |  ✓   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ✓    |
| **First-run UX**                   |      |      |     |    |       |       |      |        |
| Account-setup wizard               |  —   |  —   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.6 |
| Helpful errors on bad config       |  ~   |  ~   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.6 |

## Nice-to-have — post-v1.0 candidates

These are real features users ask for, but not all are right for
poplar's audience (a vim-first TUI for coders) or its phase. The
matrix tells us where the field lands; it does not commit poplar
to any cell.

| Feature                            | mutt | aerc | alp | TB | Apple | Gmail | Fast | poplar |
|------------------------------------|:----:|:----:|:---:|:--:|:-----:|:-----:|:----:|:------:|
| **Productivity**                   |      |      |     |    |       |       |      |        |
| Snooze                             |  —   |  —   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| Schedule send                      |  —   |  —   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.2 |
| Undo send (delay-based)            |  —   |  —   |  —  | ~  |   ✓   |   ✓   |  ✓   |   ⏳9.2 |
| Templates / canned responses       |  ✓   |  ✓   |  ~  | ~  |   ~   |   ✓   |  ✓   |   —    |
| Multiple identities (per account)  |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.4 |
| Per-identity signatures            |  ✓   |  ✓   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.4 |
| Saved / smart searches             |  ✓   |  ~   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| Tags / keywords / labels           |  ~   |  ✓   |  —  | ✓  |   ✓   |   ✓   |  ~   |   —    |
| Conversation mute                  |  —   |  —   |  —  | ~  |   —   |   ✓   |  ✓   |   —    |
| Block sender                       |  ~   |  —   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| **Server-managed**                 |      |      |     |    |       |       |      |        |
| Vacation / auto-responder          |  —   |  —   |  —  | ~  |   —   |   ✓   |  ✓   |   —    |
| Sieve filter editor                |  —   |  ~   |  —  | ~  |   —   |   ✓   |  ✓   |   —    |
| Server-side spam controls          |  —   |  —   |  —  | ~  |   —   |   ✓   |  ✓   |   —    |
| **Standards / interop**            |      |      |     |    |       |       |      |        |
| PGP / OpenPGP encrypt + sign       |  ✓   |  ✓   |  ~  | ✓  |   ~   |   —   |  —   |   —    |
| S/MIME                             |  ✓   |  —   |  ~  | ✓  |   ✓   |   ✓   |  —   |   —    |
| Calendar invites (.ics) display    |  —   |  ~   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.7 |
| Calendar invites: respond inline   |  —   |  —   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| List-Unsubscribe one-click (RFC8058)|  ~  |  ~   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   ⏳9.3 |
| Read receipts (MDN, send + reply)  |  ✓   |  —   |  ✓  | ✓  |   ✓   |   —   |  —   |   —    |
| **Contacts / identity**            |      |      |     |    |       |       |      |        |
| Built-in address book (editable)   |  ~   |  ~   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| CardDAV sync (read for autocomplete)|  ~   |  ~   |  —  | ✓  |   ✓   |   —   |  ✓   |   ⏳9.1 |
| LDAP autocomplete                  |  ✓   |  ~   |  ✓  | ✓  |   ✓   |   ~   |  —   |   —    |
| **Power-user**                     |      |      |     |    |       |       |      |        |
| Scripting / hooks                  |  ✓   |  ✓   |  ~  | ~  |   —   |   —   |  —   |   —    |
| User-customizable keybindings      |  ✓   |  ✓   |  ✓  | ~  |   —   |   ~   |  ~   |   —    |
| Themes (multiple)                  |  ✓   |  ✓   |  ~  | ~  |   —   |   ~   |  ~   |   ✓    |
| Mailing-list helpers               |  ✓   |  ✓   |  ✓  | ~  |   ~   |   ~   |  ~   |   —    |
| Inline images (sixel/kitty/iTerm)  |  —   |  ✓   |  —  | n/a|  n/a  |  n/a  | n/a  |   —    |
| **Storage / migration**            |      |      |     |    |       |       |      |        |
| Maildir / mbox local store         |  ✓   |  ✓   |  ✓  | ~  |   —   |   —   |  —   |   —    |
| notmuch integration                |  ✓   |  ✓   |  —  | —  |   —   |   —   |  —   |   —    |
| Import from other clients          |  —   |  —   |  —  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| Backup / export                    |  ~   |  ~   |  ~  | ✓  |   ~   |   ✓   |  ✓   |   —    |
| **Other**                          |      |      |     |    |       |       |      |        |
| POP3                               |  ✓   |  —   |  ✓  | ✓  |   ✓   |   ✓   |  —   |   —    |
| OS notifications                   |  ~   |  ✓   |  ~  | ✓  |   ✓   |   ✓   |  ✓   |   —    |
| Internationalization (i18n)        |  ✓   |  ~   |  ✓  | ✓  |   ✓   |   ✓   |  ✓   |   ~    |
| Accessibility (screen reader)      |  ~   |  ~   |  ~  | ✓  |   ✓   |   ✓   |  ~   |   —    |

## Pre-beta scheduling

Every `⏳` cell above maps to a pass in `STATUS.md`. Issue numbers
in BACKLOG.md carry the implementation notes (prior-art references,
scope, file list, task budget). Order:

| Pass  | Feature                                              | Issue |
|-------|------------------------------------------------------|-------|
| 9g    | Cache outbox Send/Append dispatch                    | —     |
| 9h    | ComposeTab UI + `c` wiring + tidy seam               | —     |
| 9h.5  | Drafts persistence                                   | #33   |
| 9.1   | Address autocomplete from CardDAV                    | #34   |
| 9.4   | Email signatures + multiple identities               | #32   |
| 9i    | Claude Tidy implementation                           | —     |
| 9.5   | Attachments-richer compose UI                        | #24   |
| 9.2   | Outbox delivery controls — undo + schedule send      | #35   |
| 9.3   | List-Unsubscribe one-click (RFC 8058)                | #36   |
| 9.7   | Calendar invite (.ics) viewer                        | #37   |
| 9.8   | Full-account / cross-folder search                   | #38   |
| 9.6   | First-run wizard + OAuth refresh + config template   | #27, #29 |
| 10    | Polish II                                            | #14   |
| 11    | v0.9.0 prep — feature freeze                         | —     |

## Items deliberately *not* scheduled for pre-beta

These cells are `—` in the poplar column and stay that way through
v1.0. The rationale matters in case the question comes up again.

- **Tags / keywords / labels.** JMAP keywords and IMAP keywords map
  to the same concept; Gmail labels are the closest mainstream
  analog. Real UI surface (assign, filter by, display). Post-1.0
  unless a user need forces it forward.
- **Snooze.** Fastmail implements it as a JMAP keyword + scheduled
  mailbox move — feasible without a client-side scheduler. Wait
  for a user request before designing.
- **Vacation / Sieve filter editor / conversation mute / block
  sender / templates.** All real, all post-1.0 candidates.
- **PGP / S/MIME.** Standards-heavy, niche-but-loud audience.
  Post-1.0 unless a contributor drives it.
- **Editable CardDAV address book.** Read-only is in 9.1; the full
  contacts-sidebar UI is the post-1.0 contacts initiative
  (micro-highlight design already wireframed).
- **Read receipts (MDN).** Privacy-hostile; mainstream clients are
  trending away. Skip.
- **POP3, maildir / mbox, notmuch.** Out of scope for poplar v1
  (ADR-0009 / ADR-0075 froze the backend list to JMAP + IMAP).
- **OS notifications, accessibility, full i18n.** Real gaps but
  large enough each to warrant their own initiative; queue for
  post-1.0 unless one becomes a v1.0 blocker on user feedback.

## How to use this matrix

When a new feature request lands, ask three questions:

1. **Does its absence make poplar feel like a toy** to a user
   coming from TB / Apple / Gmail / Fastmail? If yes, it's a v1.0
   candidate regardless of how the TUI peers handle it.
2. **Is the surface in poplar's voice** (terminal, vim-first,
   keyboard-driven, opinionated)? Schedule send and snooze fit.
   Stationery and rich-text toolbars don't.
3. **Does the fix ride existing infrastructure** (cache outbox,
   compose foundation, JMAP/IMAP keywords, vendored go-webdav)?
   Cheap features that ride existing rails are the highest-leverage
   v1.0 picks.

Update this doc when a feature lands or a new gap surfaces. Cells
should match reality, not aspiration.

## How to use this matrix

When considering a v1.0 candidate, ask three questions:

1. **Does its absence make poplar feel like a toy** to a user
   coming from TB / Apple / Gmail / Fastmail? If yes, it's a v1.0
   candidate regardless of how the TUI peers handle it.
2. **Is the surface in poplar's voice** (terminal, vim-first,
   keyboard-driven, opinionated)? Schedule send and snooze fit.
   Stationery and rich-text toolbars don't.
3. **Does the fix ride existing infrastructure** (cache outbox,
   compose foundation, JMAP/IMAP keywords)? Cheap features that
   ride existing rails are the highest-leverage v1.0 picks.

Update this doc when a feature lands or a new gap surfaces. Cells
should match reality, not aspiration.
