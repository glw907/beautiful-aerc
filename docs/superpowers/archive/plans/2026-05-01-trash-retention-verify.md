# Pass 6.7 — trash retention verification + reference-app research

**Date:** 2026-05-01
**Pass:** 6.7
**Goal.** Verify Pass 6.6 (retention sweep + manual empty) end-to-end
in a real terminal AND research the pattern against reference apps so
the design is either ratified or revised before it ossifies.

This is a low-risk pass: half verification, half research, no new
production code. Bugs found during verification are fixed in this
pass; design recommendations from research log as a follow-on pass —
do not expand scope here.

## Settled (do not re-brainstorm)

- ADR-0092 (`Backend.Destroy`), ADR-0093 (per-session retention
  sweep), ADR-0094 (`ConfirmModal` + manual empty) are the current
  design.
- Research may *recommend* revision, but does not rewrite invariants.
  Any revision is logged as a Pass 6.9 starter prompt and
  implemented in a follow-on pass.

## Still open — answer in the research doc

1. Retention default — 0 (opt-in) vs 30 (opt-out)?
2. Sweep trigger — first-visit / every visit / timer / on-quit?
3. Does manual empty need a "type EMPTY" affordance for very large
   folders, in addition to y/n confirm?

## Approach

Verify first. Verification may surface bugs that reframe the research
(e.g. if the sweep visibly stalls the UI, that informs the
trigger-frequency question). Bugs are fixed inline; otherwise the two
halves stay independent.

## Task 1 — Tmux verification

Mirrors the archived Pass 6.6 plan, Task 12 (which was deferred
because the pass shipped without live verification).

Run against the mock backend via a temporary `XDG_CONFIG_HOME` so the
real Fastmail account in `~/.config/poplar/accounts.toml` is not
touched.

Capture each case to a pane dump under
`docs/poplar/research/captures/2026-05-01-retention/`.

- [ ] Step 1: `make install`.
- [ ] Step 2: Set up a mock-only config:
  ```
  XDG=/tmp/poplar-verify
  mkdir -p "$XDG/poplar"
  cat > "$XDG/poplar/accounts.toml" <<'TOML'
  [[account]]
  name = "Mock"
  backend = "mock"

  [ui]
  TOML
  ```
- [ ] Step 3: Verify cases (capture each pane dump):
  - `T` jumps to Trash, no error banner.
  - `E` while on Trash opens confirm modal centered, dimmed underlay.
  - `n` closes the modal without destroying anything.
  - `E` again, then `y` — toast `Emptied Trash (5)` with no
    `[u undo]` hint, message list empties.
  - `E` while on Inbox is inert.
  - `trash_retention_days = 0` (default): no sweep on Trash visit.
  - `trash_retention_days = 30`: Trash visit triggers a sweep (mock
    fixtures have no expired messages so nothing visibly changes;
    confirm by reading `MockBackend.Destroyed()` if exposed, or by
    inspecting destroy traffic via debug log if available, or by
    seeding an old-`SentAt` message into the mock).
- [ ] Step 4: If anything fails, fix in the relevant `internal/ui/`
  or `internal/mail/` file, `make install`, re-verify.
- [ ] Step 5: Write a one-paragraph verification summary into the
  research doc's preamble (each case → pass/fail + capture path).

## Task 2 — Reference-app research

Survey how comparable mail clients handle Trash/Spam retention,
manual empty, permanent-delete bypass, and confirmation copy.
Reference-app research already lives at
`docs/poplar/research/2026-04-26-reference-apps.md`; this is a
focused supplement on the retention/empty pattern.

Targets:

- **mutt / NeoMutt** — `$trash` mailbox, `delete-message` semantics,
  `purge-message`, `mbox_close_unsave`, retention-via-cron norms.
- **aerc** — `:delete` vs trash, retention config, recent
  retention-related tickets/issues.
- **himalaya** — delete commands, trash workflow, IMAP/JMAP parity.
- **meli** — trash handling, expunge semantics.
- **One representative GUI client** — Thunderbird's "Empty Trash on
  Exit" + per-folder "delete after N days"; or Apple Mail "Erase
  deleted messages: After one month".

For each: cite the source (manpage section, config option, or
upstream doc URL). Where the project doesn't expose retention
(common case for TUI clients), say so explicitly — that's a finding.

Write to `docs/poplar/research/2026-05-01-trash-retention-norms.md`
with the structure:

1. Verification preamble (from Task 1).
2. Per-app survey (one section each, ~10 lines max).
3. Pattern synthesis — what most clients do, what the outliers do.
4. Comparison to ADR-0092/0093/0094, point by point.
5. Answers to the three open questions, each with a recommendation
   (`ratify` / `revise — see Pass 6.9`) and 1–2 sentence rationale.
6. Pass-6.9 starter prompt (if any revision is recommended).

## Task 3 — Pass-end consolidation

Standard `poplar-pass` ritual:

- `/simplify` — research doc only; expect no diff to flag.
- ADRs — only if research recommends revision *and* the revision is
  trivial enough to ratify in this pass; otherwise none.
- `docs/poplar/invariants.md` — only if an ADR is written.
- `docs/poplar/STATUS.md` — mark 6.7 done, write Pass 6.9 starter
  prompt (if recommended) or roll forward to Pass 7 (popover
  narrow-terminal polish).
- Archive this plan to `docs/superpowers/archive/plans/`.
- `make check`.
- Commit, push, install.

## Out of scope

- Implementing any retention-default change. If research says the
  default should flip, log Pass 6.9 and stop.
- Re-running the bubbletea conventions audit — no `internal/ui/`
  code lands in this pass unless Task 1 surfaces a bug.
- Touching the real Fastmail account during verification.
