# Pass 40.4 — `internal/mail` mutation-efficacy lift

## Goal

Lift `internal/mail` mutation efficacy past 80% by killing the
surviving mutants and covering the untested ones surfaced by
`make check-deep`. Raise the `scripts/check-deep.sh` floor to
observed minus 5pp at pass-end.

## Baseline (2026-05-12)

`gremlins unleash -t dev --timeout-coefficient 10 --workers 1
./internal/mail` reports:

- Killed: 9, Lived: 4, Not covered: 20 → efficacy 69.23%,
  mutator coverage 39.39%.

### Survivors (LIVED)

| Site                              | Mutator              | Notes                                                |
|-----------------------------------|----------------------|------------------------------------------------------|
| `mock.go:113:12` QueryFolder      | CONDITIONALS_BOUNDARY| `offset >= total` → `offset > total`                 |
| `mock.go:117:9` QueryFolder       | CONDITIONALS_BOUNDARY| `end > total` → `end >= total` — **equivalent mutant** |
| `mock.go:120:28` `end-offset` cap | INVERT_NEGATIVES     | Mutates capacity expression                          |
| `mock.go:120:28` `end-offset` cap | ARITHMETIC_BASE      | `end-offset` → `end+offset` (capacity grows)         |

`mock.go:117` is the equivalent mutant: with `end == total`,
both `>` and `>=` produce `end = total`. Cannot be killed by any
test. Document and accept.

### Not covered (representative)

- `backend.go:46` — `if err == nil { return false }` in
  `IsConnectionDead`. No test exists.
- `classify.go:41` — `if cf.Canonical != ""` in `ConfigKey`. No test.
- `types.go:114, 117` — `ClassifyDisposition` branch arms. No test.
- `probe.go:18` — `if ip == nil` in `IsSelfHosted`. No test.
- `probe.go:28` — `iota + 1` on `ProbeStatus` constants. **Equivalent
  mutant** (constants remain distinct under mutation).
- `mock.go:148, 156, 197, 199, 204, 232` — `ARITHMETIC_BASE` on `+`
  in `mockBodies` string concatenation. Compile-fail under
  mutation → effectively not-viable; will report as such once the
  literals are referenced by a test.

## Approach

Add tests directly against the public API. Pre-beta rules — no
shims, no new code in `internal/mail/` beyond what a test needs.

### Tasks

1. `backend_test.go` — new file: `TestIsConnectionDead` covering
   `nil` (false), `io.EOF` (true), `io.ErrUnexpectedEOF` (true),
   `net.ErrClosed` (true), `*net.OpError` (true), a timeout
   `net.Error` (true), a wrapped `*url.Error` (true), and a
   plain `errors.New` (false). Kills `backend.go:46` plus
   covers the EOF/timeout/url.Error branches.
2. `classify_test.go` — extend with `TestConfigKey` covering
   canonical (e.g. `Inbox`) and custom (e.g. `Lists/golang`)
   paths. Kills `classify.go:41`.
3. `types_test.go` — extend with `TestClassifyDisposition`
   covering: valid raw disposition (`"attachment"` → Attachment),
   valid raw inline (`"inline"` → Inline), invalid raw + cid
   present → Inline, invalid raw + no cid → Attachment, whitespace
   cid → Attachment. Kills `types.go:114, 117`.
4. `probe_test.go` — extend with `TestIsSelfHosted` covering
   `.local`, RFC 1918 IPv4 (`192.168.1.1`), IPv6 ULA
   (`fd00::1`), loopback (`127.0.0.1`, `::1`), public IP
   (`8.8.8.8` → false), non-IP non-`.local` (`example.com` →
   false). Kills `probe.go:18`.
5. `mock_test.go` — extend `TestMockBackend_QueryFolder`:
   - Add `{"at end", total, 5, 0}` and assert `uids == nil`
     (distinguishes the early-return path). Kills
     `mock.go:113`.
   - In the "clamps end" case, also assert
     `cap(uids) == end-offset`. Kills `mock.go:120`
     ARITHMETIC_BASE (and the spurious INVERT_NEGATIVES).
6. `mock_test.go` — new `TestMockBackend_FetchBody_AllSeeds`:
   iterate UIDs `1`..`8` (the keys of `mockBodies`), assert each
   `FetchBody(uid)` returns non-empty bytes and contains a
   stable substring (e.g. UID `1` contains `"Q2 launch"`).
   Brings the `mockBodies` ARITHMETIC mutants into coverage.
7. `scripts/check-deep.sh` — raise `internal/mail` floor from
   `64` to the new observed efficacy minus 5pp. Update the
   header comment block accordingly.

### Equivalent mutants

Two mutants are mathematically equivalent and cannot be killed
without rewriting the code in a way that worsens readability:

- `mock.go:117:9` `end > total` ↔ `end >= total` (clamping is
  idempotent at the boundary).
- `probe.go:28:29` `iota + 1` arithmetic on the `ProbeStatus`
  enum (constants stay distinct under any base shift).

Both are documented in this plan; no test attempts them. Pass-end
ADR records the policy.

## Verification

- `make check` green.
- `gremlins unleash -t dev --timeout-coefficient 10 --workers 1
  ./internal/mail` reports efficacy ≥ 80%.
- `scripts/check-deep.sh` passes against the raised floor.

## Pass-end

Standard checklist (skill `poplar-pass` §"Ending a pass"). ADR
records:
- The new efficacy number and the raised floor.
- The two equivalent-mutant exemptions and the policy of
  documenting rather than chasing them.
