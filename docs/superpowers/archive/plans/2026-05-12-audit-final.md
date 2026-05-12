# Pass 41 — Audit Final (pre-soak)

## Goal

Comprehensive pre-soak audit per `docs/poplar/audit-plan.md` Phase
Final. Phases A–G ran one-per-pass over Passes 36–40 and returned
empty (or empty-after-remediation). This sweep re-applies them at
once plus the three Final-only lenses: test-infrastructure quality,
security + credentials, voice + doc rot.

Deliverable: this doc, ending with a findings table. An empty
table is the soak-entry trigger.

## Method

Four parallel agents covered the Final-only lenses + an invariant-
drift cross-check against `docs/poplar/invariants.md` and
`.claude/rules/*-invariants.md`. Phase A–G re-skim was elided —
those lenses ran 1–5 passes ago against the code they were
designed for; calling fresh agents over unchanged code degrades
into noise (audit-plan.md "Failure modes for the audit itself").

## Findings table

### P0 — blocker (must remediate before soak)

None.

### P1 — fix in Pass 41 (this pass)

Doc drift — repaired inline in this pass:

- `docs/poplar/invariants.md:75-78` — `mailcompose.AssembleMIME`
  signature `(d, identities, now)` was documented as `(d, now)`;
  package qualifier was `compose.AssembleMIME` instead of
  `mailcompose.AssembleMIME`. **Fixed.**
- `docs/poplar/invariants.md:301-302` — `compose.Model embeds
  catkin.Model directly` was wrong; the actual struct has a named
  `editor catkin.Model` field. **Fixed.**

### P1 — queue Pass 41.1

Security:

- **Wizard config temp-file race.** `internal/ui/wizard/section_confirm.go:118-136`,
  `cmd/poplar/root.go:262-278` (repair write), `cmd/poplar/config_discover_folders.go:80-97`:
  `os.CreateTemp` honors the process umask. On a loose-umask
  system, the file is world-readable between `CreateTemp` and the
  follow-up `os.Chmod(0o600)`. Config contents include plaintext
  `password` values. Fix: `os.OpenFile(...O_CREATE|O_EXCL, 0o600)`
  or `f.Chmod(0o600)` immediately on the file handle before any
  write — pattern already correct in `tokenstore_age.go`.

Test infrastructure (Audit G remediation pattern, per ADR-0233):

- **`internal/cache/cache_test.go:44` `fakeBackend.Connect`** returns
  `nil` unconditionally. Add `connectErr` injection seam.
- **`internal/cache/cache_test.go:49` `fakeBackend.QueryFolder`**
  returns `nil, 0, nil` unconditionally. Add `queryErr` seam.
- **`internal/cache/bodies_test.go:197` `fakeBackendWithBody.FetchBody`**
  returns `nil` error unconditionally. Add `bodyErr` seam.
- **`internal/mailimap/fake_test.go`** missing `selectErr`/`loginErr`
  on `fakeClient`; `finishConnect` error path during idle-reconnect
  redial is untested.
- **IMAP cmd-path `ErrAuth` → drainer conflict matrix.** SMTP path
  has end-to-end coverage (`smtp_test.go:103`); IMAP `dial` path
  surfaces `mail.ErrAuth` via `classifyErr` but no drainer test
  pulls it through to `OpConflict auth-failure`.

Voice — high-yield T15 renames:

- **`internal/cache/account.go:83` `cache.CacheEvent` → `cache.Event`.**
  Package-doubled type appearing across UI, drainer, and tests.
  Will require an invariants.md companion edit (the type appears
  by name in multiple binding facts).
- **`internal/ui/compose/model.go:28` `compose.CacheStore` → `compose.Store`.**

Voice — ADR rot:

- **ADRs 0233–0237 em-dash density.** 4–9 em-dashes per ADR
  (T33 allowance is "a handful per repo"). Mechanical pass:
  replace clause-joining `—` with `.` where the halves are
  independent clauses; keep parenthetical em-dashes.

Invariant drift (cross-checked, low-priority repairs queued):

- **`invariants.md:31` ADR refs `0178, 0191` are wrong** for the
  mail backends paragraph. ADR-0178 = claude-tidy; ADR-0191 =
  first-run wizard. Correct refs include 0193 (OAuth refresh),
  0220 (native OAuth wiring), 0227 (device-code fallback).
- **`invariants.md:24-25`** internal-package brace-list omits
  `ansix`, `cache`, `catkin`, `contacts`, `icalendar`, `mailcompose`,
  `search`, `strdist`. List was always partial; either annotate
  "(load-bearing subset)" or list all 20.
- **`.claude/rules/catkin-invariants.md`** claims muesli/reflow
  is "provenance-noted in catkin.go and uicore/overlay.go".
  `muesli/reflow` is not in `go.mod`; the named files do not
  contain that provenance. The reflow primitives are a clean-room
  reimplementation — update the invariant accordingly.

### P2 — fix within two passes

- **Secret-in-memory lifetime.** `mailimap.Backend.password` and
  `mailauth.Client.cachedTok` survive past disconnect with no
  explicit zeroing. Acceptable under the "no swap, OS memory
  isolation" threat model — add a one-line comment near each
  field stating the model explicitly so future readers don't
  treat the omission as accidental.
- **Cache drainer transient→`OpFailed` with `next_eligible_at`**
  end-to-end test gap. `TestQueuedSendFailure_AppendHeldOff`
  exercises the gate; nothing reads back the status + retry
  timestamp.
- **Voice line-level findings.** `internal/cache/account.go:97`,
  `internal/wizard/apply.go:44`, `internal/mail/types.go:96`,
  `internal/catkin/catkin.go:8`, `internal/catkin/popover.go:168`,
  `internal/term/probe.go:17` — minor T8/T28/T30/T34. Fold into
  Pass 41.1's simplify run.

### nit — fold inline next pass

- Three independent `checkGolden` helpers in `internal/ui/`,
  `uicore/`, `account/` all do byte-for-byte diff with no
  pre-check for blank output. A one-line `len(got) > 0` guard
  would catch a blank-golden accident with a clearer message.
- `contacts.NewClient` takes a naked `insecureTLS bool` param;
  call sites pass struct fields so the footgun-at-call-site
  doesn't trigger.
- Protonmail preset `InsecureTLS: true` for 127.0.0.1 Bridge
  loopback is intentional; add a one-line comment so a future
  reader doesn't read it as a mistake.

## Soak gate

The audit table is not empty: 1 P1 security finding + 5 test-infra
seams + 2 voice renames + 4 ADR-voice cleanups + 3 invariant drift
items. Beta soak does not open with Pass 41; Pass 41.1 (remediation)
must land first, then a re-skim confirms the table empty.

## Pass 41.1 plan summary

- **Security:** OpenFile-with-mode for the three temp-file write
  sites. Mechanical.
- **Test infra:** five new `*Err error` injection fields per Audit
  G pattern (ADR-0233); one new end-to-end IMAP cmd-path ErrAuth
  → drainer test.
- **Voice:** two T15 renames (`CacheEvent → Event`, `CacheStore →
  Store`); ADR-0233–0237 em-dash pass; six line-level voice fixes.
- **Doc drift:** correct ADR refs in invariants.md:31; expand /
  annotate the internal-package list; fix catkin reflow provenance
  claim.

Standard pass-end checklist applies. Expect 41.1 to fit in one ADR
+ ~10 tasks.
