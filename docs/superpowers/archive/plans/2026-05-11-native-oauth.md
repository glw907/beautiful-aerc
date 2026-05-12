# Pass 35 — Native OAuth final wiring — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the wizard's probe stage succeed for Gmail/Outlook OAuth accounts, retire the `oauth2l` recipe from the config template, and document native OAuth as the canonical XOAUTH2 path.

**Architecture:** Thread `*mailauth.Client` through `wizard.Probe` → `mailimap.Probe` + `mailimap.ProbeSMTP`. The wizard's probe stage builds the client from the just-collected OAuth state (refresh token already in the store from the consent step). Replace the template's oauth2l block with an `[account.oauth]` example. Live-verify against real Gmail + Outlook accounts.

**Tech Stack:** Go 1.26, `golang.org/x/oauth2`, `emersion/go-imap/v2`, `emersion/go-smtp`, existing `mailauth` package.

Spec: `docs/superpowers/specs/2026-05-11-native-oauth-design.md`.

---

## Task 1 — Thread access token through `mailimap.Probe`

**Files:**
- Modify: `internal/mailimap/probe.go`
- Modify: `internal/mailimap/probe_test.go`

Today `Probe` accepts `*mailauth.Client`, calls `Token(ctx)` as the first step on success, but then drops the result on the floor — `realProbeDial` falls back to `cfg.ResolvePassword()` for the AUTHENTICATE step. Capture the token and thread it to `probeDial`.

- [ ] **Step 1: Update `probeDialFn` to take an access-token override**

In `internal/mailimap/probe.go`, change the indirection:

```go
// probeDial is the test seam for the dial+TLS+auth phase.
var probeDial probeDialFn = realProbeDial

type probeDialFn func(cfg config.AccountConfig, accessToken string) (imapClient, []mail.ProbeStep, error)
```

- [ ] **Step 2: Update `Probe` to capture the token and pass it through**

Replace the `Probe` body so the OAuth step stores the token, then hands it to `probeDial`:

```go
func Probe(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
	var r mail.ProbeResult
	var accessToken string
	if oauthCli != nil {
		step := mail.ProbeStep{Label: "oauth-token"}
		tok, err := oauthCli.Token(ctx)
		if err != nil {
			step.Status = mail.ProbeFail
			step.Detail = err.Error()
			r.Steps = append(r.Steps, step)
			r.Err = err
			return r
		}
		accessToken = tok
		step.Status = mail.ProbeOK
		r.Steps = append(r.Steps, step)
	}

	cli, steps, err := probeDial(cfg, accessToken)
	r.Steps = append(r.Steps, steps...)
	if err != nil {
		r.Err = err
		return r
	}
	defer func() { _ = cli.Logout() }()

	// CAPABILITY (UIDPLUS asserted) — unchanged
	caps, err := cli.Capabilities()
	// ... rest of Probe body unchanged ...
}
```

Leave the CAPABILITY and STATUS INBOX sections alone — only the prelude changes.

- [ ] **Step 3: Update `realProbeDial` to honor the override**

In `realProbeDial`, replace the password-resolution block (the `cfg.ResolvePassword()` call and what follows down through the AUTHENTICATE step) with:

```go
	var pw string
	if accessToken != "" && cfg.Auth == "xoauth2" {
		pw = accessToken
	} else {
		var err error
		pw, err = cfg.ResolvePassword()
		if err != nil {
			_ = cli.Logout().Wait()
			steps = append(steps, mail.ProbeStep{
				Label: "AUTHENTICATE", Status: mail.ProbeFail, Detail: err.Error(),
			})
			return nil, steps, fmt.Errorf("password: %v", err)
		}
	}

	if err := authenticate(cli, cfg, pw); err != nil {
		_ = cli.Logout().Wait()
		steps = append(steps, mail.ProbeStep{
			Label: "AUTHENTICATE", Status: mail.ProbeFail, Detail: err.Error(),
		})
		return nil, steps, fmt.Errorf("authenticate: %v", err)
	}
```

The function signature becomes `func realProbeDial(cfg config.AccountConfig, accessToken string) (imapClient, []mail.ProbeStep, error)`.

- [ ] **Step 4: Update probe_test.go fakes for the new signature**

In `internal/mailimap/probe_test.go`, every `probeDial = func(...)` test stub gains an `accessToken string` parameter. Add a new test:

```go
func TestProbeUsesOAuthAccessToken(t *testing.T) {
	prev := probeDial
	t.Cleanup(func() { probeDial = prev })

	var gotToken string
	probeDial = func(cfg config.AccountConfig, accessToken string) (imapClient, []mail.ProbeStep, error) {
		gotToken = accessToken
		return &fakeIMAP{caps: map[string]bool{"UIDPLUS": true}}, nil, nil
	}

	store := &memTokenStore{tok: "rt"}
	cli := mailauth.NewClient(mailauth.Config{
		ClientID: "id", AuthURL: "https://example/auth", TokenURL: tokenServerStub(t),
	}, store, "slug", mailauth.BackendKeyring)

	r := Probe(context.Background(), config.AccountConfig{Auth: "xoauth2", Host: "h"}, cli)
	if !r.OK() {
		t.Fatalf("probe failed: %+v", r)
	}
	if gotToken == "" {
		t.Fatal("expected access token threaded into probeDial")
	}
}
```

(`memTokenStore` + `tokenServerStub` follow the pattern already in `internal/mailauth/oauth_test.go`. If those helpers aren't exported, inline minimal equivalents here — copy, don't export.)

- [ ] **Step 5: Run tests**

Run: `go test ./internal/mailimap/... -run Probe`

Expected: PASS, including the new `TestProbeUsesOAuthAccessToken`.

- [ ] **Step 6: Commit**

```bash
git add internal/mailimap/probe.go internal/mailimap/probe_test.go
git commit -m "mailimap: thread OAuth access token through Probe"
```

---

## Task 2 — Add `*mailauth.Client` parameter to `mailimap.ProbeSMTP`

**Files:**
- Modify: `internal/mailimap/smtp.go`
- Modify: `internal/mailimap/smtp_test.go` (if probe tests live there) or `internal/mailimap/probe_test.go`
- Modify: `cmd/poplar/config_cmd.go`

Today `ProbeSMTP` builds the Backend with `New(cfg, nil)` — `b.oauth` is always nil — so `smtpAuth`'s xoauth2 branch falls through to `cfg.SMTP.ResolvePassword()`, which is unset for OAuth accounts.

- [ ] **Step 1: Change `ProbeSMTP` signature and routing**

In `internal/mailimap/smtp.go`, replace `ProbeSMTP`:

```go
// ProbeSMTP dials, authenticates, and closes. `poplar config check` and
// the wizard's probe stage call it after IMAP succeeds. oauthCli is non-nil
// for accounts whose SMTP.Auth is xoauth2; passing nil for non-OAuth
// accounts keeps the password-based path.
func ProbeSMTP(cfg config.AccountConfig, oauthCli *mailauth.Client) error {
	var b *Backend
	if oauthCli != nil {
		b = NewWithOAuth(cfg, oauthCli, nil)
	} else {
		b = New(cfg, nil)
	}
	cli, err := smtpDial(b)
	if err != nil {
		return err
	}
	return cli.Close()
}
```

- [ ] **Step 2: Update `cmd/poplar/config_cmd.go` to thread the client**

Find the `ProbeSMTP` call in `newConfigCheckCmd` (currently around line 134). Replace:

```go
				if a.Backend == "imap" {
					if err := mailimap.ProbeSMTP(a); err != nil {
```

with a block that builds the OAuth client when `a.OAuth != nil`:

```go
				if a.Backend == "imap" {
					var oauthCli *mailauth.Client
					if a.OAuth != nil {
						c, err := buildOAuthClient(a)
						if err != nil {
							fmt.Fprintf(cmd.OutOrStdout(), "%-20s oauth error: %v\n", a.Name, err)
							anyFail = true
							continue
						}
						oauthCli = c
					}
					if err := mailimap.ProbeSMTP(a, oauthCli); err != nil {
```

Add `"github.com/glw907/poplar/internal/mailauth"` to the import block if not already present.

- [ ] **Step 3: Update existing ProbeSMTP test stubs**

Search for `ProbeSMTP(` usage in tests:

Run: `grep -rn "ProbeSMTP(" internal/ cmd/ --include="*.go"`

Update every non-production call site to pass `nil` as the second argument. Most tests will be one-line edits.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mailimap/... ./cmd/poplar/...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/smtp.go cmd/poplar/config_cmd.go internal/mailimap/*_test.go
git commit -m "mailimap: thread OAuth client through ProbeSMTP"
```

---

## Task 3 — Add `*mailauth.Client` parameter to `wizard.Probe`

**Files:**
- Modify: `internal/wizard/probe.go`
- Modify: `internal/wizard/probe_test.go`

The wizard's `Probe` indirection currently passes `nil` to `mailimap.Probe` and calls `mailimap.ProbeSMTP(cfg)`. Plumb the client through both.

- [ ] **Step 1: Update indirections and the public `Probe` signature**

In `internal/wizard/probe.go`, replace the file's contents below the imports:

```go
// Indirection so tests can substitute fakes without dialing real servers.
var (
	imapProbeFn = func(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
		return mailimap.Probe(ctx, cfg, oauthCli)
	}
	jmapProbeFn = mailjmap.Probe
	smtpProbeFn = func(cfg config.AccountConfig, oauthCli *mailauth.Client) error {
		return mailimap.ProbeSMTP(cfg, oauthCli)
	}
)

// Probe routes to the right backend based on cfg.Backend and returns a
// step-by-step transcript. oauthCli is non-nil for accounts using xoauth2;
// nil otherwise. SMTP is appended for IMAP accounts only; JMAP submission
// rides the JMAP session itself.
func Probe(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
	switch cfg.Backend {
	case "imap":
		r := imapProbeFn(ctx, cfg, oauthCli)
		if !r.OK() {
			return r
		}
		smtpErr := smtpProbeFn(cfg, oauthCli)
		step := mail.ProbeStep{Label: "SMTP submission", Status: mail.ProbeOK}
		if smtpErr != nil {
			step.Status = mail.ProbeFail
			step.Detail = smtpErr.Error()
			r.Err = fmt.Errorf("smtp: %w", smtpErr)
		}
		r.Steps = append(r.Steps, step)
		return r
	case "jmap":
		return jmapProbeFn(ctx, cfg)
	}
	return mail.ProbeResult{Err: fmt.Errorf("unknown backend %q", cfg.Backend)}
}
```

Add `"github.com/glw907/poplar/internal/mailauth"` to the imports.

- [ ] **Step 2: Update probe_test.go fakes**

In `internal/wizard/probe_test.go`, change `withFakeProbes` and every call site so the IMAP/SMTP fakes take the new `*mailauth.Client` parameter. Pattern:

```go
func withFakeProbes(t *testing.T,
	imap func(context.Context, config.AccountConfig, *mailauth.Client) mail.ProbeResult,
	jmap func(context.Context, config.AccountConfig) mail.ProbeResult,
	smtp func(config.AccountConfig, *mailauth.Client) error,
) {
	t.Helper()
	prevI, prevJ, prevS := imapProbeFn, jmapProbeFn, smtpProbeFn
	imapProbeFn, jmapProbeFn, smtpProbeFn = imap, jmap, smtp
	t.Cleanup(func() { imapProbeFn, jmapProbeFn, smtpProbeFn = prevI, prevJ, prevS })
}
```

Each existing test's fake gets a `_ *mailauth.Client` parameter. Existing call sites `Probe(ctx, cfg)` become `Probe(ctx, cfg, nil)`. Add a new test:

```go
func TestProbeThreadsOAuthClientIntoIMAPAndSMTP(t *testing.T) {
	var gotIMAP, gotSMTP bool
	withFakeProbes(t,
		func(_ context.Context, _ config.AccountConfig, c *mailauth.Client) mail.ProbeResult {
			gotIMAP = c != nil
			return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "imap", Status: mail.ProbeOK}}}
		},
		func(context.Context, config.AccountConfig) mail.ProbeResult { return mail.ProbeResult{} },
		func(_ config.AccountConfig, c *mailauth.Client) error { gotSMTP = c != nil; return nil },
	)

	sentinel := &mailauth.Client{}
	Probe(context.Background(), config.AccountConfig{Backend: "imap"}, sentinel)
	if !gotIMAP || !gotSMTP {
		t.Fatalf("oauth client not threaded: imap=%v smtp=%v", gotIMAP, gotSMTP)
	}
}
```

Add `"github.com/glw907/poplar/internal/mailauth"` to the test imports.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/wizard/...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/wizard/probe.go internal/wizard/probe_test.go
git commit -m "wizard: thread OAuth client through Probe"
```

---

## Task 4 — Build the OAuth client in the wizard probe screen

**Files:**
- Modify: `internal/ui/wizard/probe_screen.go`

The probe stage already runs after the OAuth section has stored a refresh token in the keyring or age file. Construct a `*mailauth.Client` from the wizard state and pass it to `wizard.Probe`. `Token(ctx)` will read the just-stored refresh token from the store and exchange it for an access token without another browser hop.

- [ ] **Step 1: Add a helper that builds the OAuth client when needed**

In `internal/ui/wizard/probe_screen.go`, add (above `runProbe`):

```go
func (p *probeScreen) oauthClient() *mailauth.Client {
	state := p.parent.State
	if !state.OAuthDone {
		return nil
	}
	preset := config.Providers[state.Preset]
	if preset.OAuth == nil {
		return nil
	}
	slug := cache.Slugify(state.Email)
	store, backend, err := mailauth.OpenStore(slug, oauthFallbackDir())
	if err != nil {
		return nil
	}
	cfg := mailauth.Config{
		ClientID:     state.OAuthCID,
		ClientSecret: state.OAuthSecret,
		AuthURL:      preset.OAuth.AuthURL,
		TokenURL:     preset.OAuth.TokenURL,
		Scopes:       preset.OAuth.Scopes,
	}
	return mailauth.NewClient(cfg, store, slug, backend)
}
```

Add these imports to the file:

```go
"github.com/glw907/poplar/internal/cache"
"github.com/glw907/poplar/internal/mailauth"
```

`config` is already imported. `oauthFallbackDir` lives in `section_oauth.go` (same package), so no new helper needed.

- [ ] **Step 2: Thread the client into `runProbe`**

Replace the `runProbe` body:

```go
func (p *probeScreen) runProbe() tea.Cmd {
	cfg := p.cfg
	oauthCli := p.oauthClient()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return ProbeDoneMsg{Result: wizdomain.Probe(ctx, cfg, oauthCli)}
	}
}
```

The client is built once at `runProbe` call time (not inside the goroutine) so retries on the same screen reuse the same store handle.

- [ ] **Step 3: Build**

Run: `go build ./...`

Expected: success.

- [ ] **Step 4: Run all tests**

Run: `make test`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/wizard/probe_screen.go
git commit -m "wizard: build OAuth client for probe stage from wizard state"
```

---

## Task 5 — Replace the `oauth2l` block in the config template

**Files:**
- Modify: `internal/config/template.go`

- [ ] **Step 1: Rewrite the OAuth-providers section**

In `internal/config/template.go`, replace the block currently spanning roughly lines 81–95 (the "# OAuth providers" comment block) with:

```go
# OAuth providers
#
#   gmail and outlook authenticate via OAuth 2.0 (XOAUTH2). Poplar
#   handles the consent flow natively — when you run ` + "`poplar`" + ` for
#   the first time, the setup wizard opens a browser, you grant
#   access, and a refresh token is stored in your system keyring
#   (or, as a fallback, an age-encrypted file under
#   ` + "`$XDG_CONFIG_HOME/poplar/oauth/`" + `).
#
#   Bring your own OAuth app: register a desktop/installed client
#   with Google or Microsoft, then set the client ID and secret
#   under [account.oauth]:
#
#       [[account]]
#       name = "personal-gmail"
#       email = "you@gmail.com"
#       provider = "gmail"
#
#         [account.oauth]
#         client-id     = "1234567890-abc.apps.googleusercontent.com"
#         client-secret = "GOCSPX-..."
#         oauth-store   = "keyring"  # or "age-file"
#
#   To rerun consent (e.g. after revoking access), run
#   ` + "`poplar --reauth=<account-name>`" + `.
```

Leave the surrounding "# App passwords" and "# ProtonMail" sections alone.

- [ ] **Step 2: Verify the template still parses**

Run: `go test ./internal/config/... -run Template`

Expected: PASS. (If no test exercises the rendered template, also run `go vet ./internal/config/...` to catch raw-string mistakes.)

- [ ] **Step 3: Commit**

```bash
git add internal/config/template.go
git commit -m "config: replace oauth2l template recipe with native OAuth block"
```

---

## Task 6 — Live verification: Gmail

**Files:**
- New: `docs/poplar/captures/2026-05-11-pass35-gmail-wizard-120x40.txt`
- New: `docs/poplar/captures/2026-05-11-pass35-gmail-wizard-80x24.txt`
- New: `docs/poplar/captures/2026-05-11-pass35-gmail-reauth.txt`

Prereqs: a real Google account with a desktop OAuth client registered in the Google Cloud console (Client ID + Client Secret in hand). Throwaway test inbox preferred.

- [ ] **Step 1: Move the live config aside**

```bash
mv ~/.config/poplar/config.toml ~/.config/poplar/config.toml.pass35-bak 2>/dev/null || true
rm -rf ~/.config/poplar/tokens ~/.config/poplar/oauth 2>/dev/null || true
```

This forces first-run wizard behavior and clears any stale tokens.

- [ ] **Step 2: Build and install**

Run: `make install`

Expected: success; `~/.local/bin/poplar` updated.

- [ ] **Step 3: Run the wizard in tmux at 120×40, capture each stage**

Follow `.claude/docs/tmux-testing.md`. Launch poplar in a 120×40 pane, walk through preset → email → OAuth credentials → consent → probe → identity → signature → label. Capture frames at each stage and concatenate into `docs/poplar/captures/2026-05-11-pass35-gmail-wizard-120x40.txt` with `=== <stage> ===` headers.

The probe stage must show `✓ oauth-token`, `✓ Connecting`, `✓ TLS handshake`, `✓ AUTHENTICATE xoauth2`, `✓ CAPABILITY (UIDPLUS)`, `✓ STATUS INBOX <N> messages`, `✓ SMTP submission`. Any failure step is a regression — stop and report.

- [ ] **Step 4: Repeat at 80×24**

Same as Step 3 but in an 80×24 pane. Capture to `docs/poplar/captures/2026-05-11-pass35-gmail-wizard-80x24.txt`. Verify the probe-step transcript is still legible at Spartan width.

- [ ] **Step 5: Verify the inbox loads after wizard exit**

Stay in poplar (or relaunch with the just-written config). The inbox should render with real Gmail messages. If not, capture a screenshot and stop.

- [ ] **Step 6: Verify `--reauth` works**

Quit poplar. Run:

```bash
POPLAR_LOG=debug poplar --reauth=<gmail-account-name> 2>&1 | tee /tmp/pass35-reauth.log
```

The browser opens, consent completes, the CLI prints `reauth <name>: refresh token stored`. Excerpt the relevant log lines (token refresh, store write) into `docs/poplar/captures/2026-05-11-pass35-gmail-reauth.txt`.

- [ ] **Step 7: Restore prior config (if any)**

```bash
mv ~/.config/poplar/config.toml.pass35-bak ~/.config/poplar/config.toml 2>/dev/null || true
```

- [ ] **Step 8: Commit captures**

```bash
git add docs/poplar/captures/2026-05-11-pass35-gmail-*.txt
git commit -m "captures: Pass 35 Gmail OAuth live verification"
```

---

## Task 7 — Live verification: Outlook

**Files:**
- New: `docs/poplar/captures/2026-05-11-pass35-outlook-wizard-120x40.txt`
- New: `docs/poplar/captures/2026-05-11-pass35-outlook-wizard-80x24.txt`
- New: `docs/poplar/captures/2026-05-11-pass35-outlook-reauth.txt`

Prereqs: a real Microsoft/Outlook account with an Azure-registered desktop OAuth client (Client ID + Client Secret). Throwaway test inbox preferred.

- [ ] **Step 1: Move the live config aside**

```bash
mv ~/.config/poplar/config.toml ~/.config/poplar/config.toml.pass35-bak 2>/dev/null || true
rm -rf ~/.config/poplar/tokens ~/.config/poplar/oauth 2>/dev/null || true
```

- [ ] **Step 2: Run the wizard in tmux at 120×40**

Same procedure as Gmail Task 6 Step 3, but choose preset `outlook`. Capture to `docs/poplar/captures/2026-05-11-pass35-outlook-wizard-120x40.txt`.

Expected steps in the probe stage are identical to Gmail: `oauth-token`, `Connecting`, `TLS handshake`, `AUTHENTICATE xoauth2`, `CAPABILITY (UIDPLUS)`, `STATUS INBOX`, `SMTP submission`. The host shown is `outlook.office365.com:993`.

- [ ] **Step 3: Repeat at 80×24**

Capture to `docs/poplar/captures/2026-05-11-pass35-outlook-wizard-80x24.txt`.

- [ ] **Step 4: Verify inbox loads**

Same as Gmail Task 6 Step 5.

- [ ] **Step 5: Verify `--reauth`**

```bash
POPLAR_LOG=debug poplar --reauth=<outlook-account-name> 2>&1 | tee /tmp/pass35-reauth-outlook.log
```

Excerpt the token-refresh and store-write log lines into `docs/poplar/captures/2026-05-11-pass35-outlook-reauth.txt`.

- [ ] **Step 6: Force a refresh-token rotation check**

In poplar, leave the account idle for 65 minutes (longer than Outlook's access-token lifetime). When activity resumes (folder switch or IDLE event), `mailauth.Token` should refresh transparently and the IMAP connection stays alive. If a fresh refresh token was returned by the token endpoint, `mailauth.Client.Token` writes it back to the store. Tail the log:

```bash
tail -F ~/.local/state/poplar/poplar.log
```

Confirm an `oauth refresh: rotated` (or similar) line appears. Append the relevant excerpt to the outlook-reauth capture.

- [ ] **Step 7: Restore prior config**

```bash
mv ~/.config/poplar/config.toml.pass35-bak ~/.config/poplar/config.toml 2>/dev/null || true
```

- [ ] **Step 8: Commit captures**

```bash
git add docs/poplar/captures/2026-05-11-pass35-outlook-*.txt
git commit -m "captures: Pass 35 Outlook OAuth live verification"
```

---

## Pass-end consolidation

Run the `poplar-pass` skill's ending ritual after Task 7 lands:

1. `/simplify` over the diff (Tasks 1–5).
2. Write ADR-0220 — Native OAuth is the canonical XOAUTH2 path. Body covers: BYO client ID via `[account.oauth]`; loopback redirect ephemeral (`127.0.0.1:0`); `mailauth.SetOpenBrowser` separate from App `URLOpener`; `oauth2l` deprecated in template; refresh-token rotation handled in `mailauth.Client.Token`.
3. Edit `docs/poplar/invariants.md` in place: under the "OAuth" subsection (or "Architecture → Repo & libraries" if no dedicated subsection exists), state that `mailauth.Client` is constructed in `openBackend`, `runReauth`, `section_oauth.runAuthorize`, and `probe_screen.oauthClient`; that `mailimap.Probe` and `mailimap.ProbeSMTP` both take an optional `*mailauth.Client`; and that `oauth2l` shell-out is no longer supported in the template (existing user `password-cmd` configs continue to work via the existing path).
4. Add the ADR-0220 row to `docs/poplar/decisions/INDEX.md`.
5. Update `docs/poplar/STATUS.md`: mark Pass 35 done; replace the starter prompt with Pass 36 (Audit C — feature surface) per the existing pass table.
6. `make check` green.
7. `git add -A && git commit -m "Pass 35: native OAuth final wiring" && git push && make install`.

## Verification summary

- `make check` green after every code task (1–5).
- `internal/mailimap/probe_test.go` includes a test that asserts the access token is threaded into `probeDial`.
- `internal/wizard/probe_test.go` includes a test that asserts the `*mailauth.Client` flows into both IMAP and SMTP fake probes.
- Live Gmail capture shows `✓ oauth-token` and `✓ AUTHENTICATE xoauth2` in the wizard probe stage.
- Live Outlook capture shows the same.
- `--reauth` rewrites the stored refresh token for both providers.
- The config template no longer mentions `oauth2l`.
