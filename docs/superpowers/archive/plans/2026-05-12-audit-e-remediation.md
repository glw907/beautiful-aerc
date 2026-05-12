# Pass 38.1 — Audit E remediation

Lands the three P1 findings from ADR-0225 plus F1's bundled
doc-hygiene fix.

## F6 wizard surface — decision (no formal brainstorm)

Inline radio in `oauthStageCredentials`, not a separate stage.

- `huh.NewSelect[string]("Consent method", "Loopback (recommended)",
  "Device code")` appended after Client ID + Client secret.
- Default loopback; user toggles for SSH/container/NAT cases.
- On loopback failure the existing retry screen gains `[d] switch
  to device code` next to `[r] retry` / `[s] cancel`.
- Wizard state grows `OAuthMode string` ("loopback" | "device-
  code"); threaded into `buildOAuthClient` so it picks the right
  `Authorize` method.

Rationale: separate stage adds friction for the loopback-default
case; ADR-0225 frames device-code as fallback, not first-class;
the existing form already collects two fields, a third radio
composes naturally with `huh`.

## Tasks

1. **F1** — Set `0003-external-editor-only.md` frontmatter
   `status: superseded by 0034`; append a Consequences-section
   link from 0003 to 0034. Doc-only.

2. **F2** — Movepicker mode toggle.
   - Add `mode` field (filter | nav) to `movepicker.Model`,
     default filter.
   - `Tab` toggles modes; in nav mode `j/k` navigate, in filter
     mode keystrokes feed the filter.
   - List's internal `KeyMap.CursorUp/Down` disabled so filter
     mode never steals `j/k`.
   - Footer hint updates per mode.
   - Update `docs/poplar/keybindings.md` movepicker row.
   - Update `.claude/rules/ui-invariants.md` movepicker bullet
     under Overlays.
   - Unit test: filter mode types `j` into the filter; nav mode
     `j` advances cursor.

3. **F4** — Outbox per-row size cap.
   - Add `MaxOutboxBytes int64` to `config.CacheConfig` with TOML
     key `max-outbox-bytes`, default 0 (unlimited), mirroring
     ADR-0122's `max-size`.
   - Thread through `cache.Config` → `Account.maxOutboxBytes`.
   - `cache.ErrOutboxRowTooLarge` sentinel.
   - `insertFolderOp` rejects with the sentinel when
     `maxOutboxBytes > 0 && len(payload) > maxOutboxBytes`;
     before INSERT.
   - `QueueOutbound` propagates the error (returns nil ids on
     failure of the first op).
   - Config writer round-trip in `writer.go`.
   - Unit tests: under-cap accepts, over-cap rejects, zero
     disables.

4. **F6 — `mailauth.AuthorizeDeviceCode`.**
   - RFC 8628 device-code flow: POST device-auth endpoint,
     display user_code + verification_uri in the TUI, poll token
     endpoint per `interval` until success / `authorization_
     pending` / `slow_down` / `access_denied` / `expired_token`.
   - Add `DeviceAuthURL string` to `mailauth.Config` and
     `config.OAuthEndpoints`; preset values for gmail + outlook.
   - `Client.AuthorizeDeviceCode(ctx context.Context,
     display func(userCode, verificationURI string)) error`.
   - Reuses existing keyring/age-file `TokenStore` — only the
     `Authorize` path differs.

5. **F6 — Wizard wiring.**
   - `wizard.Model` gains `OAuthMode string`.
   - `oauthSection.buildForm` appends the consent-method radio.
   - `runAuthorize` branches on mode; device-code path advances
     `stage` to a new `oauthStageDevice` that renders user_code
     + URL while polling.
   - `oauthStageFlow` failure screen on loopback adds `[d]`
     hint and key handler that resets to `oauthStageDevice`
     directly (skipping the credentials form re-fill).
   - Persist mode to `[account] oauth-mode` when device-code was
     used (loopback is the default; field omitted on round-trip
     when unset).

6. **F6 — Tests.**
   - `mailauth` httptest server emulating RFC 8628 happy path
     and `slow_down` interval bump.
   - `wizard` model test verifying mode propagates through
     `buildOAuthClient`.

7. **Consolidation.**
   - Write ADR-0226 (Outbox per-row cap, F4) and ADR-0227
     (Device-code OAuth fallback, F6 — supersedes ADR-0193's
     "no device-code" clause; update 0193 frontmatter +
     Consequences).
   - F1 + F2 don't warrant new ADRs (F1 is doc hygiene; F2 is a
     UX consistency fix already covered by ADR-0064's pattern,
     just extending it to a second consumer — note in
     ui-invariants update is enough).
   - Update `docs/poplar/invariants.md`: movepicker bullet under
     Overlays (Tab toggle), cache bullet (MaxOutboxBytes),
     mailauth bullet (device-code).
   - Update `decisions/INDEX.md` with rows for 0226 + 0227.
   - Update `STATUS.md`: mark 38.1 done; emit Pass 39 starter
     prompt (Audit F — sharp edges + insecure defaults).
   - Archive this plan to `docs/superpowers/archive/plans/`.
   - `make check`, commit, push, `make install`.

## Pass-size budget

7 tasks; under the 12-task ceiling. F6 is the heaviest single
piece but `mailauth.AuthorizeDeviceCode` is self-contained and
the wizard wiring follows the existing oauthSection shape. No
need to split to 38.2.

## Out of scope

- P2 findings (F3, F5, F7–F11) — BACKLOG / ROADMAP per ADR-0225.
- F12 — already addressed by ADR-0165.
- Gmail/Outlook live device-code verification — blocked on
  Pass 35.1 credentials.
