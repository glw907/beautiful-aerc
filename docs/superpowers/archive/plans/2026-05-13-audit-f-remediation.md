# Pass 39.1 — Audit F remediation

Land the 8 P1 items from ADR-0228. All surface-level edits.

## Items

1. **F-F-1** — `mailauth/loopback.go`: set
   `ReadHeaderTimeout: 10*time.Second` and `WriteTimeout:
   5*time.Second` on the consent `http.Server`.

2. **F-F-2** — `mailimap/smtp.go`: thread `ctx` through the
   `smtpDial` seam (`func(ctx context.Context, b *Backend)`)
   and `Backend.smtpClientLocked(ctx)`. `Send` carries
   `context.Background()` (no caller ctx yet); future work can
   plumb a real ctx. `ProbeSMTP` passes its own ctx (it has
   none today — wrap with `context.Background()` at the call
   site, since the probe is short and synchronous). Update the
   test fake to the two-arg signature.

3. **F-F-3** — `mailauth/devicecode.go`: in `RequestDeviceAuth`
   (or its caller) floor `da.ExpiresIn` to 300 when the server
   returns ≤ 0, before the value reaches `PollDeviceCode`.

4. **F-batch2-1** — `cache/reads.go:266` + `cache/attachments.go:
   34, 120`: replace `_ = storeErr` with `a.log.Warn(...)` and
   for the metadata path at `attachments.go:34` propagate the
   error to the caller (the row hasn't been written, so a
   subsequent `FetchAttachment` would surface "unknown row").
   Add `log/slog` to the package via `a.log` — already present
   on `*Account`.

5. **F-batch2-2** — `cache/drainer.go:executeOne`: capture the
   results of every `finalizeSuccess` and `finishOp` call and
   log via `a.log.Error("finalize", "op", row.ID, "err", err)`
   on non-nil.

6. **F-batch3-1** — `ui/account/cmds.go:67`: when `c.SyncFolder`
   errors, return a `uicore.ErrorMsg{Op: "sync folder", Err:
   err}`. Mirror `loadFoldersCmd`. Compose: `SyncFolder` is in
   a sub-closure; promote the error out and short-circuit the
   subsequent QueryFolder when sync failed (matches sibling
   behavior where sync error fails the load).

   *On reflection:* ADR-0228 says "sync errors don't fail the
   load" was the prior comment, but the sibling `loadFoldersCmd`
   does fail. Match the sibling — fail with an ErrorMsg, don't
   silently proceed.

7. **F-batch4-1** — `cmd/poplar/backend.go:buildOAuthClient`:
   change the `mailauth.OpenStore(acct.Name, tokenDir)` call
   site to honor `acct.OAuthStore`. New signature: extend
   `OpenStore` to take an optional preferred backend
   (`OpenStore(slug, fallbackDir string, preferred Backend)
   (TokenStore, Backend, error)`), where empty string falls
   back to the existing keyring-probe path; "keyring" and
   "age-file" skip the probe and use the requested store.

8. **F-batch4-2** — `cmd/poplar/config_discover_folders.go:
   writeAtomically`: insert `os.Chmod(tmpPath, 0o600)` after
   `tmp.Close()` and before `os.Rename`. Mirror
   `writeConfigAtomic` in `root.go`.

## Pass-end

- `/simplify`, run modern-go + voice check.
- ADR-0229 (remediation; supersedes nothing).
- No invariant updates expected — these are bug-fixes, not
  binding-fact changes. (Possibly mention `OpenStore` signature
  in invariants if the API surface is enshrined; it isn't —
  skip.)
- Archive this plan.
- `make check`, commit, push, install.
