---
title: Audit E — specified vs. assumed
status: accepted
date: 2026-05-12
---

## Context

Pass 38 ran Audit E per `docs/poplar/audit-plan.md` §"Phase E":
walk every ADR in `docs/poplar/decisions/` (0001–0224) and tag
each one *chosen* (named alternative, real tradeoff interrogated)
or *defaulted* (the obvious-looking choice ossified without
interrogation). The premise: LLM defaults are right 80% and
silently wrong 20%; unquestioned defaults compound across passes.
Trigger: Audit D remediation (Pass 37.1, ADR-0224) shipped.

The walk and per-range findings live in
`docs/superpowers/archive/plans/2026-05-12-audit-e.md`.

## Decision

Aggregate over 212 active ADRs (12 superseded skipped):
121 chosen, 79 defaulted-and-still-right, 12 defaulted-and-wrong.
Twelve findings split 0 P0 / 3 P1 / 8 P2 / 1 already-addressed.

Pass 38.1 lands the three P1 items plus F1 as a free hygiene
bundle. P2 items go to BACKLOG / ROADMAP. F12 is recorded so a
future audit doesn't re-surface it.

**P1 — queue Pass 38.1:**

- **F2 — Movepicker arrow-key navigation (ADR-0091).** The
  sidebar-search pattern (ADR-0064, `Tab` cycles filter/nav)
  solves the same `j`/`k`-vs-filter conflict and preserves
  vim-first consistency. Movepicker's `↑`/`↓` carve-out is the
  finding. Fix: Tab toggles modes; `j`/`k` navigate in nav mode.
- **F4 — Outbox `payload` unbounded (ADR-0158).** No per-account
  or per-row cap on assembled MIME bytes. A stalled multi-MB
  Append against a disconnected backend with `MaxAttempts = 0`
  bloats the cache forever. Fix: `[cache] max-outbox-bytes`
  config knob (default unlimited, mirrors ADR-0122) + a per-row
  soft cap surfaced at queue time.
- **F6 — OAuth BYO-client only, no device-code fallback
  (ADR-0193).** Loopback PKCE forecloses SSH-tunnel, container,
  and NAT-bound users. The ADR rejected device-code citing
  Google CASA display-name verification, but CASA gates
  maintainer-distributed clients, not user-driven device-code.
  Fix: keep loopback PKCE default, add device-code as fallback
  selectable in the wizard when loopback fails or the user opts
  in. Scope is wizard + `mailauth` + an additional `Authorize`
  mode; if 38.1 oversizes, F6 splits to its own pass.

**P2 — noted, not queued:**

- **F1 — ADR-0003 not marked superseded.** Status still
  `accepted` despite being contradicted by ADR-0034 (inline
  compose). Doc hygiene. Bundled into 38.1 free of charge.
- **F3 — ADR-0108 XOAUTH2 token caching gap.** Legacy
  `password-cmd`-XOAUTH2 path runs the resolver on every dial.
  Native OAuth (ADR-0193/0220) caches via `mailauth.Token`; the
  legacy lane is sunsetting. 5-minute grace cache would
  eliminate the round-trip if anyone still uses it.
- **F5 — ADR-0184 hand-rolled date parser.** `compose.ParseSchedule`'s
  ~175-line English keyword + offset parser will grow gaps as
  users type things it doesn't grok. `araddon/dateparse` is one
  dep with the full vocabulary.
- **F7 — ADR-0188 FTS5 stored-content storage cost.** 33k+
  message bodies duplicated on disk (FTS5 stored-content vs.
  contentless). Reclassifying to contentless is schema v14 +
  `fts.go` rewrite. Storage-only; ROADMAP candidate.
- **F8 — ADR-0066 markdown body width hardcoded 72.** One
  `[ui] body-width` config field with default 72. Small.
- **F9 — ADR-0085 long-bare-URL threshold 30 cells.** Trips on
  short query-param URLs (e.g. `example.com/login?token=abc`).
  Revisit when BACKLOG #22 lands.
- **F10 — ADR-0135 zero-length attachment results not cached.**
  `messages.has_attachments BOOL` eliminates a per-viewer-open
  backend round-trip on every attachment-free message.
- **F11 — ADR-0147 catkin word-level undo coalescing.** Workable
  for prose; suboptimal for Ctrl+K link skeletons, chip edits,
  and tidy-replace blocks. No clear remediation candidate yet.

**Already addressed:**

- **F12 — ADR-0164 draft last-write-wins** was extended by
  ADR-0165 (`ErrDraftSuperseded` + banner). Logged so a future
  audit doesn't re-surface.

## Consequences

- Pass 38.1 lands F1 + F2 + F4 + F6 before Audit F (Pass 39).
- F6 has the largest surface area (wizard, `mailauth`, an
  `Authorize` mode); if 38.1's task list crosses 12, F6 splits
  into Pass 38.2 per the CLAUDE.md pass-size budget.
- The 79 defaulted-and-still-right ADRs are not findings, but
  the 22 ADRs in that bucket with no articulable alternative
  (listed in the plan doc) form a candidate set for Phase Final
  if the project decides to prune ADRs that should not have
  needed to be ADRs.
- The audit-plan §"Phase E" walk strategy (parallel dispatch by
  themed range with a per-range tally) returned 12 real findings
  across 212 ADRs — the rubric is calibrated. Empty audits are
  the soak-entry signal; this isn't one.
- Phase F (sharp edges + insecure defaults) trigger remains
  "Phase E returns empty"; with three P1s queued, Phase F gates
  on Pass 38.1 landing, then re-runs Phase E if any of the P1
  remediations reshape the ADR archive enough to invalidate the
  walk.
