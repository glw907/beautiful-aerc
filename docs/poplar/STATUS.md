# Poplar Status

**Current pass:** Pass 39 ran Audit F (sharp edges + insecure
defaults) across all 21 packages in four parallel batches. Tally
**0 P0, 8 P1, 14 P2**; dominant pattern is `_ =` error suppression
on write-back / terminal-state paths, not bad config defaults.
ADR-0228 records the finding set. Pass 39.1 lands the eight P1
items before Audit G.

Pass 35.1 still pending Gmail/Outlook creds.

**Beta soak deferred.** Pre-beta rules apply.

## Passes

| Pass | Goal | Status |
|------|------|--------|
| 1 – 34 | Scaffold through cross-pane mouse | done |
| 35 | Native OAuth final wiring (ADR-0220) | done |
| 35.1 | Live Gmail + Outlook OAuth verification | pending creds |
| 36 / 36.1 | Audit C + remediation (ADR-0221/0222) | done |
| 37 / 37.1 | Audit D + remediation (ADR-0223/0224) | done |
| 38 / 38.1 | Audit E + remediation (ADRs 0225/0226/0227) | done |
| 39 | Audit F (ADR-0228) | done |
| 39.1 | **Audit F remediation — 8 P1 fixes** | next |
| 40 | Audit G — test assertion meaningfulness | gate |
| 41 | Audit Final — comprehensive pre-soak | gate |
| Beta soak | Enter when Audit Final returns empty | conditional |
| v1.0.0 | Tag after soak settles | conditional |

### Next starter prompt (Pass 39.1)

> **Goal.** Land the eight P1 items from Audit F (ADR-0228)
> before Audit G runs.
>
> **Scope (per item, all surface-level):**
>
> - **F-F-1** `mailauth/loopback.go:39` — `ReadHeaderTimeout: 10s`,
>   `WriteTimeout: 5s` on the loopback `http.Server`.
> - **F-F-2** `mailimap/smtp.go:34` — thread `ctx` through the
>   `smtpDial` seam and `smtpClientLocked`; update the test fake.
> - **F-F-3** `mailauth/devicecode.go:115` — floor `ExpiresIn` to
>   300 after `RequestDeviceAuth` when the server returns ≤ 0.
> - **F-batch2-1** `cache/reads.go:266`, `cache/attachments.go:34,
>   120` — `slog.Warn` at each `_ = storeErr`; propagate the
>   metadata-cache error at `attachments.go:34`.
> - **F-batch2-2** `cache/drainer.go:138–174` — log every
>   `finalizeSuccess` / `finishOp` failure via the drainer logger.
> - **F-batch3-1** `ui/account/cmds.go:67` — surface `SyncFolder`
>   errors (match `loadFoldersCmd`'s `ErrorMsg` pattern).
> - **F-batch4-1** `cmd/poplar/backend.go:55`, `reauth.go:34` —
>   pass `acct.OAuthStore` to `mailauth.OpenStore`; configured
>   store wins over the keyring probe.
> - **F-batch4-2** `cmd/poplar/config_discover_folders.go:76` —
>   `os.Chmod(tmpPath, 0o600)` before `Rename`, mirroring
>   `writeConfigAtomic` in `root.go:278`.
>
> **Settled.** ADR-0228 records disposition. P2 items go to
> BACKLOG separately; don't bundle them.
>
> **Open — brainstorm.** None; mechanical fixes.
>
> **Approach.** Plan at
> `docs/superpowers/plans/2026-05-13-audit-f-remediation.md`,
> implement, write remediation ADR-0229, standard pass-end
> checklist.
