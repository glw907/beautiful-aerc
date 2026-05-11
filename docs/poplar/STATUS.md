# Poplar Status

**Current pass:** post-v0.9.0; pre-beta cadence continues.

**Beta soak deferred** (2026-05-11 decision). `v0.9.0` is tagged
as a milestone but does not gate the rules — pre-beta operational
rules in `CLAUDE.md` continue to apply until soak entry. Refactors,
schema changes, and feature work all land on master.
`docs/poplar/release-stance.md` describes the soak/post-1.0
phases for reference.

**Soak-entry bar.** The trigger for entering beta soak is not a
calendar or a version tag — it is **a directed Claude audit
unable to surface any bugs or major flaws**. The
2026-05-11 audit that prompted skipping soak surfaced #51, #52,
and #53 (mail-infra hazards) plus the catkin all-value straddle
(`catkin-all-value` project) and the 874-line `App.Update`
(`app-decomposition` project), among others. Pre-beta stays
active until a comparable audit comes back empty.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 16d | Scaffold through slog adoption (ADRs 0001–0197) | done |
| 17a | Sidebar folder hierarchy on a v2 tree component (ADR-0198) | done |
| 17b | `messagelist` on `bubbles/v2/list` (ADR-0199) | done |
| 17c | `bubbles/v2/help` audit + bubbles-deviation ADRs (0200, 0201) | done |
| 18 | Polish II — retire underlay dim, footer ellipsis, helppopover zero-arg View, KeyMap exports, sidebar render cache (ADR-0202) | done |
| 19 | pre-beta refactor (outbound) — drop MessageInfo.Date string, compose→mailcompose, tidy→tidytext + CLI strip (ADR-0203) | done |
| 19.1 | pre-beta refactor (mechanical) — #46 reconciled, #47 strdist consolidation (ADR-0204) | done |
| 20 | v0.9.0 prep — README, docs sweep, hero screenshot, tag (ADR-0205) | done |
| 21 | Beta-soak bug fix — restore compose body Focus() (ADR-0206); compose hero screenshot; Nerd Font in VHS tapes | done |
| 22 | Wizard signature step — catkin editor between identity and label, multi-line TOML render, sentinel round-trip | done |
| 23 | First-launch safety — #49 wizard preset, #29 name default, #51 MockBackend | queued |
| 24 | IMAP robustness — #53 IDLE/cmd redial, #52 outbox send/append gate | queued |
| 25 | Small-refactor sweep — #50 ansix Measurer, options collapse, backoff/humanize fold | queued |
| 26 | Catkin Elm conformance (all-value path) | queued |
| 27 | Compose `Editor` wrapper deletion (subsumes overengineering-cleanup item 1) | queued |
| 28 | `app.go` decomposition — split 874-line `App.Update` | queued |
| 29 | v2 declarative View fields — ProgressBar + ReportFocus + KeyboardEnhancements | queued |
| 30 | Mouse support (reader + attachments + scroll) | queued |
| 31 | Mouse support (sidebar + cross-pane) — optional split from 30 | queued |
| 32 | Native OAuth for Gmail / Outlook IMAP (#42, BYO client ID) | queued |
| v1.0.0 | Tag when the queued sequence lands | pending |
| post-1.0 | Neovim companion (#6), raw RFC822 (#21), beyond | future |

## Next steps

Pre-beta cadence is back in effect: all work lands on master,
schema changes welcomed, refactors first-class. Sequence below
runs straight through to `v1.0.0`. No master / 1.1 branch split.

**Pass 23 — First-launch safety.** Three first-launch / config
hazards converging on `internal/config/accounts.go`:
- **#49** wizard probe runs against unresolved preset (dies on
  "session URL is empty" for every hosted preset; first-run
  unusable). Fix: extract `config.ResolvePreset(*AccountConfig)`,
  call from both the decoder and `wizard.Apply`.
- **#29** config-template `name` defaults to email (drop the
  `name == ""` validator check; default `Name` to `Email`).
- **#51** `MockBackend` ships in production and silently swallows
  `Send`. Fix: `//go:build dev` tag on `mail.NewMockBackend`
  registration in `cmd/poplar/backend.go`; reject `provider = ""`
  and `provider = "mock"` in the production validator.

**Pass 24 — IMAP robustness.** Two related outbox/connection bugs:
- **#53** IMAP IDLE "reconnect" doesn't redial — `idleLoop` retries
  against the same `b.idle` pointer set once in `Connect`. When
  the TCP connection actually dies (laptop sleep/wake, NAT reap,
  server housekeeping), poplar shows `Reconnecting…` indefinitely
  with no recovery. The `cmd` connection has the same defect for
  every Move/Flag/Destroy action. Fix: introduce `mail.ErrConnection`
  sentinel; on connection-dead, drop `b.idle` / `b.cmd` and dial
  fresh before retry. Mirrors the same backend's SMTP drop-and-
  redial pattern (`smtp.go:141-153`).
- **#52** IMAP outbox `Append` can dispatch while sibling `Send`
  is failed (never-sent message lands in Sent). Gate
  `nextOutboxRow` with a `NOT EXISTS` subquery on the `draft_id`-
  linked sibling. No schema change.

**Pass 25 — Small-refactor sweep.** Net deletions; no user-visible
behavior change:
- **#50** ansix `Measurer` (drop the `spuaCellWidth` package global)
- Collapse triplicate `WithLogger` functional-options in
  `mailjmap` / `mailimap` / `cache` to plain `*slog.Logger` args
  (amend ADR-0197)
- Fold `internal/backoff/` + `internal/humanize/` into their
  primary callers; drop the defensive `<= 0` clamps

**Pass 26 — Catkin Elm conformance (all-value path).** Deepest
structural fix; everything after sits on top. Convert every
pointer-mutator on `catkin.Model` and `catkin.Buffer` into Msg
types handled in `Update`. Wrap `bubbles/v2/textarea` in a
value-typed adapter inside catkin so upstream contagion stops at
the package boundary. Migrate `compose.Model` and
`wizard/section_signature.go` to send config Msgs.

**Pass 27 — Compose `Editor` wrapper deletion.** Subsumes
`overengineering-cleanup` item 1; mechanical post-Pass 26. Delete
`mailcompose.Editor` + `CatkinEditor`; `compose.Model` holds
`catkin.Model` directly. Fold into Pass 26 if review-size allows.

**Pass 28 — `app.go` decomposition.** Split the 874-line
`App.Update` into per-screen controllers (`updateAccount`,
`updateContacts`, `updateCompose`, `updateModals`,
`updateOutbox`, `updateWizard`); peel `internal/ui/app.go` into
`app_update.go` / `app_view.go` / `app_chrome.go` so no single
file exceeds ~600 lines.

**Pass 29 — v2 declarative View fields.** Wire `v.ProgressBar`
(OSC 9;4 for sync, outbox drain, attachment downloads),
`v.ReportFocus` + `FocusMsg`/`BlurMsg` (pause JMAP push / IMAP
IDLE refresh on blur, kick refresh on focus, suppress bell), and
`v.KeyboardEnhancements` (Kitty keyboard protocol; sequenced
after Pass 26 so catkin's chord-disambiguation surface is
conformant; unlocks `IsRepeat` for j/k acceleration).

**Pass 30 — Mouse support (reader + attachments + scroll).** Wire
`v.MouseMode = MouseModeCellMotion` + `v.OnMouse` hit-test for
link clicks in the reader, attachment row clicks, wheel-scroll
in messagelist and reader. `[ui] mouse = "on" | "off"`, default
on.

**Pass 31 — Mouse support (sidebar + cross-pane).** Sidebar
folder selection on click, click-to-focus between panes,
contacts list/detail interaction. Splits from Pass 30 only if
planning shows >12 tasks.

**Pass 32 — Native OAuth (Gmail + Outlook IMAP, BYO client ID).**
The biggest feature still queued. BYO client-ID consent flow in
`internal/mailauth/`; refresh-token storage in keyring/age-file;
proactive refresh before access-token expiry. Wizard provider
picker (Pass 14) already routes `gmail`/`outlook` rows here.

Tag **`v1.0.0`** after Pass 32 (or earlier if scope feels
complete and OAuth slips to post-1.0).

Full project descriptions in `ROADMAP.md`.

## Notes for the 16-series (modernization)

ADR-0196 binds the convention; 16b–d apply it. Audit appendix in
the archived 16a plan has the full file:line list. Pass 16d
landed ADR-0197 (slog adoption).
