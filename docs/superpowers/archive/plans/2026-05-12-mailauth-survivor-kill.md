# Pass 40.5b — mailauth survivor kill + seven-package floor recalibration

## Goal

Push `internal/mailauth` efficacy from 78.50% to ≥ 98% by killing
the 23 surviving mutants surfaced under the calibrated
`--timeout-coefficient 10 --workers 1` invocation, then re-measure
the seven other curated packages' floors under the same flags.

## The 23 survivors

Captured 2026-05-12 via `gremlins unleash -t dev
--timeout-coefficient 10 --workers 1 --output-statuses l
./internal/mailauth` (Killed: 84, Lived: 23, Not covered: 1; mutator
coverage 99.07%).

| Site | Mutator | Strategy |
|------|---------|----------|
| devicecode.go:80:23 | `len(Scopes) > 0` boundary | Test empty-Scopes path: device POST omits `scope=` |
| devicecode.go:101:13 ×2 | `expires <= 0` boundary + negation | Test ExpiresIn=0 + negative; both default to 300 |
| devicecode.go:125:14 | `interval <= 0` boundary | Test Interval=0; defaultDevicePollInterval honored |
| devicecode.go:184:22 | `ClientSecret != ""` negation | Test empty ClientSecret omits `client_secret=` |
| devicecode.go:200:19 ×2 | `ExpiresIn > 0` boundary + negation | Test ExpiresIn=0 → zero Expiry; positive → non-zero |
| devicecode.go:201:60 | `ExpiresIn * time.Second` arithmetic | Pin exact Expiry value (ExpiresIn=3600 → +1h) |
| loopback.go:42:25 | `10 * time.Second` arithmetic | Assert srv.ReadHeaderTimeout == 10s |
| loopback.go:43:24 | `5 * time.Second` arithmetic | Assert srv.WriteTimeout == 5s |
| oauth.go:151:23 ×2 | `len(Scopes) > 0` boundary + negation | Test buildAuthURL with empty Scopes: no scope param |
| oauth.go:177:33 | rand.Read err check | Test generatePKCEVerifier returns non-empty |
| oauth.go:190:33 | rand.Read err check | Test generateState returns non-empty |
| oauth.go:213:48 | `time.Until > 5*time.Minute` boundary | **Equivalent.** Boundary only differs at exact 5-min equality, untestable without a clock seam; the 5-min buffer is conservative, not load-bearing (analogous to ADR-0235 mock.go:117). |
| oauth.go:243:22 | `RefreshToken != ""` negation | Server returns empty RefreshToken; assert store unchanged via call-counting store |
| oauth.go:243:48 | `RefreshToken != refresh` negation | Server returns same RefreshToken; assert no Set call |
| oauth.go:256:22 | `sc == 400` negation | 500 status + invalid_grant body → raw err, not ErrAuth |
| tokenstore_keyring.go:24:9 | Set `err == nil` negation | keyring.MockInitWithError → Set returns non-nil |
| tokenstore_keyring.go:49:9 | Delete `err == nil` negation | MockInitWithError → Delete returns non-nil |
| tokenstore_keyring.go:80:36 | OpenStore probe Set err check | MockInitWithError → OpenStore returns AgeFileStore |
| tokenstore_keyring.go:81:36 ×2 | probe Get err + value check | Test both branches via mock |

22 killable, 1 documented equivalent. Total mutants = Killed 84 + Lived 23 + NotCovered 1 = 108. Killing 22 of 23 lifts Killed to 106, Lived to 1, efficacy to 106/107 = 99.07%. Mutator coverage stays 107/108 since the one NotCovered mutant remains.

## Tasks

1. Write the killer tests, file by file:
   - `devicecode_test.go` — 9 new assertions across new + existing tests.
   - `loopback_test.go` — assert srv timeouts.
   - `oauth_test.go` — empty-scope auth URL, rand-helper smoke, refresh-token Set semantics, 500+invalid_grant.
   - `tokenstore_test.go` — `keyring.MockInit` / `MockInitWithError` paths for Set/Delete/OpenStore probe.
2. Re-run gremlins on mailauth; confirm only the documented
   equivalent mutant survives.
3. Run gremlins on the seven other curated packages
   (`internal/{content,filter,mail,cache,tidytext,mailcompose,config}`)
   with the calibrated flags; update `scripts/check-deep.sh`
   floors to `observed − 5pp` per package.
4. `make check` green.
5. Standard pass-end consolidation: ADR-0237 captures the survivor
   policy + new floor table; invariants.md + INDEX.md updated;
   plan archived; commit/push/install.

## Out of scope

- New tests beyond what's needed to kill survivors.
- Rewriting `oauth.go:213` to introduce a clock seam — the boundary
  mutant is equivalent under any non-clock-controlled test, and
  the 5-minute buffer is intentionally fuzzy.
- The single NotCovered mutant — that's a separate ratchet (would
  need to identify which line gremlins flagged; pursue when
  efficacy work resumes).
