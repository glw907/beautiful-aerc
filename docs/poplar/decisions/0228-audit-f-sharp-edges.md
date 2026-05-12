---
title: Audit F — sharp edges and insecure defaults
status: accepted
date: 2026-05-12
---

## Context

Pass 39 ran Audit F per `docs/poplar/audit-plan.md` §"Phase F":
walk every package in `internal/` plus `cmd/poplar/` against the
Trail-of-Bits sharp-edges / insecure-defaults lens — config
defaults whose easy path is the wrong path, boolean parameters
opaque at the call site, magic optional defaults, ignored
returns that suppress errors, and permissive failure paths that
return `nil` after logging a warning. The bias the lens targets:
LLMs prefer "looks helpful," producing convenience methods that
obscure failure modes and defaults tuned for the example case.

Trigger: Audit E remediation (Pass 38.1, ADRs 0226/0227) shipped.

Walk dispatched in four parallel batches by package cluster:
mail stack; storage & content; UI layer; config / CLI / utilities.
Per-batch findings live in
`docs/superpowers/archive/plans/2026-05-12-audit-f.md`.

## Decision

Aggregate over the four batches: **0 P0, 9 P1, 13 P2** (22 total
findings).

| Batch | P0 | P1 | P2 |
|-------|----|----|----|
| 1 — mail stack | 0 | 3 | 4 |
| 2 — storage & content | 0 | 2 | 1 |
| 3 — UI layer | 0 | 2 | 5 |
| 4 — config / CLI | 0 | 2 | 3 |
| **total** | **0** | **9** | **13** |

On re-triage, batch-3's F-batch3-2 (compose pointer receivers)
demotes from P1 to P2 — structural-only, no current bug. Final
P1 set is the 8 items below.

Pass 39.1 lands the eight P1 items. P2s land in BACKLOG; the
disk-leak shapes (F-batch4-4, F-batch3-5) go to ROADMAP if
appetite warrants.

**P1 — queue Pass 39.1:**

- **F-F-1 — OAuth consent server lacks HTTP timeouts.**
  `internal/mailauth/loopback.go:39` constructs `http.Server`
  with no `ReadHeaderTimeout`/`WriteTimeout`. A stalled local
  connection hangs the consent goroutine until
  `consentTimeout`. Fix: set `ReadHeaderTimeout: 10s`,
  `WriteTimeout: 5s`.
- **F-F-2 — `smtpDial` discards caller context.**
  `internal/mailimap/smtp.go:34` hardcodes
  `context.Background()` in the production closure. SMTP dial
  cannot be cancelled mid-drain. Fix: thread `ctx` through
  `smtpClientLocked` and the `smtpDial` seam.
- **F-F-3 — Device-code poll deadline computed from
  potentially-zero `ExpiresIn`.**
  `internal/mailauth/devicecode.go:115` — when the server
  omits `expires_in`, the deadline is `time.Now()` and the
  first iteration returns `ErrConsentTimeout` before polling.
  Fix: `if da.ExpiresIn <= 0 { da.ExpiresIn = 300 }` after
  `RequestDeviceAuth`.
- **F-batch2-1 — Cache write-back errors silently dropped on
  read paths.** `cache/reads.go:266`, `cache/attachments.go:120`,
  `cache/attachments.go:34` discard `storeBody` /
  `storeAttachments` errors with `_ = storeErr`. Body and
  attachment writes that fail are invisible: server re-fetch
  every open, no error-banner signal. Fix: `slog.Warn` at each
  site; for the metadata-cache path at `attachments.go:34`
  also propagate so callers don't subsequently see "unknown
  row (call Attachments first)."
- **F-batch2-2 — Drainer terminal-state errors silently
  dropped.** `cache/drainer.go:138–174` uses `_ =` on every
  `finishOp` and `finalizeSuccess`. A DB write failure leaves
  the op stuck in `OpExecuting`, blocking sibling-op pickup
  with no log entry. Fix: capture and log via the drainer's
  logger.
- **F-batch3-1 — `SyncFolder` error silently swallowed in
  `queryFolderCmd`.** `account/cmds.go:67` discards the
  per-folder sync error while `loadFoldersCmd` surfaces
  `SyncFolders` errors as `ErrorMsg`. Stale-folder failures
  are invisible. Fix: at minimum `slog.Warn`; better, emit
  an `ErrorMsg` consistent with the sibling path.
- **F-batch4-1 — `buildOAuthClient` ignores `acct.OAuthStore`.**
  `cmd/poplar/backend.go:55` and `cmd/poplar/reauth.go:34`
  call `mailauth.OpenStore` without the configured store hint.
  A user who chose `oauth-store = "age-file"` silently gets
  whichever store the keyring probe favors. Worse: keyring
  probe drift between setup and runtime hides the token in
  the wrong place. Fix: pass `acct.OAuthStore` to
  `OpenStore`; skip probe when explicit.
- **F-batch4-2 — `writeAtomically` in
  `config_discover_folders.go` skips `chmod 0600` before
  `Rename`.** `cmd/poplar/config_discover_folders.go:76`
  diverges from `writeConfigAtomic` in `root.go:278`, which
  explicitly chmods. config.toml can carry embedded
  `password` / `password-cmd`; the sister function's chmod
  signals defense-in-depth intent. Fix: mirror the
  `os.Chmod(tmpPath, 0o600)` call before `Rename`.

Total: 8 P1, all queued.

**P2 — BACKLOG / noted:**

- **F-F-4** Rotated refresh-token store error dropped
  (`oauth.go:244`). Subsequent process start fails to auth
  with no diagnostic.
- **F-F-5** `io.ReadAll` error dropped on device-code HTTP
  body (`devicecode.go:159`). Truncated body produces
  misleading "unexpected end of JSON input."
- **F-F-6** `lru.New` error dropped in `mailjmap.NewWithClient`
  (size compile-time known; `panic(err)` more honest).
- **F-F-7** `Flag(uids, flag, set bool)` boolean parameter.
  Named `type FlagOp bool` with `Set`/`Clear` constants reads
  better at every call site.
- **F-batch2-3** `contacts.Client.HomeSet` falls back to base
  URL on auth error, hiding 401/403 as a downstream
  "list books" error.
- **F-batch3-3** `wizard/section_theme.go:66` uses
  `lipgloss.JoinHorizontal` with potentially-SPUA content
  (huh form rows). ADR-0084 ban applies; cosmetic
  misalignment on Nerd-Font terminals.
- **F-batch3-4** `walkBody` `mr.Close()` not deferred
  (`account/cmds.go:229`). Current paths all reach the close;
  future `return` inside the loop would leak. `defer` is the
  small fix.
- **F-batch3-5** `tidytext.CallAPI` hardcodes `MaxTokens:
  4096` (`tidytext/api.go:47`). Long bodies silently truncate;
  `stop_reason == "max_tokens"` not differentiated from a
  complete response. ROADMAP candidate if Tidy goes beyond
  short-reply scope.
- **F-batch3-6** `maxBodyWidth = 72` constant
  (`content/render.go:11`). Already noted in Audit E F8;
  `[ui] body-width` knob proposed there.
- **F-batch3-7** `SetBackfill(done, total int, paused, warn
  bool)` two adjacent booleans (`status_bar.go:79`). Named
  types or a small `BackfillState` struct.
- **F-batch4-3** Protonmail preset forces `InsecureTLS = true`
  with no runtime warning in `config check` output.
- **F-batch4-4** `[cache] max-size = 0` (unlimited) undocumented
  in the first-run template. Disk-fill surprise. Template
  comment is the small fix; ROADMAP if a sensible default cap
  is decided.
- **F-batch4-5** `password-cmd` inherits full parent
  environment. Standard pattern (pass/op CLI/secret-tool); a
  comment near `resolvePasswordCmd` documenting the inheritance
  semantics is the fix.
- **F-batch3-2** `compose.Model.View()` and `Draft()` pointer
  receivers. No current mutation in View, but the pointer-
  receiver API across `compose.Model` (`SetSize`, `SetTidy`,
  `refreshSuggest`, `handleTidyKey`, `applyTidyResult`) lets
  callers mutate state outside the Update loop. Structural
  finding, no current bug. Demoted from P1 on re-triage. Worth
  a follow-up pass to convert to value receivers if the
  current pointer-API isn't load-bearing.

## Consequences

- Pass 39.1 lands the eight P1 items before Audit G (Pass 40).
  Largest item is F-batch2-2 (drainer terminal-state logging,
  touches the conflict matrix); F-F-2 (smtpDial context) is
  the second-largest because it threads `ctx` through the
  `smtpClient` seam used in tests. Other seven are
  surface-level edits.
- The cluster of "errors dropped with `_ =`" findings
  (F-batch2-1, F-batch2-2, F-batch3-1, F-F-4, F-F-5) confirms
  the Phase F lens caught a real pattern: silent error
  suppression is the project's dominant sharp-edge shape, not
  insecure config defaults. ADR-0073's "errors must surface"
  has a discipline gap on the drainer + cache write-back
  paths.
- No P0 findings → soak gate not invalidated. Audit G
  proceeds after 39.1.
- Audit-plan §"Phase F" walk strategy (per-package parallel
  dispatch in four batches by cluster) returned 22 findings
  across 21 packages — the rubric is calibrated. The cluster
  shape (errors dropped, not bad defaults) tells future
  audits where to grep first.
- Phase G (test assertion meaningfulness) trigger is "Phase F
  returns empty"; with eight P1s queued, Phase G gates on Pass
  39.1 landing.
