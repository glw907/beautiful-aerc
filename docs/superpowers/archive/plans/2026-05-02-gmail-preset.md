# Gmail Preset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `gmail` provider preset adapting the generic IMAP backend to Gmail's quirks (X-GM-EXT-1 assertion, Destroy via `[Gmail]/Trash`, XOAUTH2 access tokens via `password-cmd` with no caching).

**Architecture:** Extend `config.Provider` with `GmailQuirks bool`; copy that flag onto `AccountConfig` during preset resolution. In `mailimap`, add `capSet.XGM`, assert at Connect when the flag is set, branch `Destroy` to `Select(trash)` first when the flag is set, and skip the `b.password` cache for `cfg.Auth == "xoauth2"`. Strip the unused `OAuthClient*`/`OAuthRefreshToken` fields per pre-beta clean-code stance — Pass 9.6 (first-run wizard) re-adds them with a real consumer.

**Tech Stack:** Go 1.26, `emersion/go-imap` v2, `BurntSushi/toml`. Existing `internal/mailauth/xoauth2.go` SASL client unchanged.

**Spec:** `docs/superpowers/specs/2026-05-02-gmail-preset-design.md`.

---

## Task 1: Strip dead OAuth fields

The `OAuthClientID`, `OAuthClientSecret`, `OAuthRefreshToken` fields on `AccountConfig` are decoded from TOML but never consumed by any backend. Pre-beta stance (CLAUDE.md): strip dead fields; the wizard pass re-adds with consumers.

**Files:**
- Modify: `internal/config/account.go:62-67`
- Modify: `internal/config/accounts.go:32-34, 114-125, 166-168`
- Modify: `internal/config/accounts_test.go` (any test referencing OAuth fields)

- [ ] **Step 1: Find every reference to the doomed fields**

```bash
grep -rn "OAuthClient\|OAuthRefresh\|oauth-client-id\|oauth-client-secret\|oauth-refresh-token" \
    /home/glw907/Projects/poplar/ \
    | grep -v "/archive/"
```

Expected: matches in `internal/config/account.go`, `internal/config/accounts.go`, possibly `internal/config/accounts_test.go`. The archive specs also mention them but are immutable — do not touch.

- [ ] **Step 2: Remove the fields from `AccountConfig`**

In `internal/config/account.go`, delete the block:

```go
	// XOAUTH2 inputs. All env-var-substituted via $VAR.
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRefreshToken string
```

The XOAUTH2 path uses `Password`/`PasswordCmd` directly as the access token (see `mailimap/auth.go:213-217`); no struct fields needed.

- [ ] **Step 3: Remove the fields from the TOML decode struct**

In `internal/config/accounts.go`, delete the three lines from `accountEntry`:

```go
	OAuthClientID     string            `toml:"oauth-client-id"`
	OAuthClientSecret string            `toml:"oauth-client-secret"`
	OAuthRefreshToken string            `toml:"oauth-refresh-token"`
```

- [ ] **Step 4: Remove the `resolveEnv` plumbing**

In `internal/config/accounts.go`, delete the three blocks following the password-cmd validation:

```go
	clientID, err := resolveEnv(e.OAuthClientID)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) oauth-client-id: %w", e.Name, e.Provider, err)
	}
	clientSecret, err := resolveEnv(e.OAuthClientSecret)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) oauth-client-secret: %w", e.Name, e.Provider, err)
	}
	refresh, err := resolveEnv(e.OAuthRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("account %q (provider = %q) oauth-refresh-token: %w", e.Name, e.Provider, err)
	}
```

And drop these three lines from the `&AccountConfig{...}` literal:

```go
		OAuthClientID:     clientID,
		OAuthClientSecret: clientSecret,
		OAuthRefreshToken: refresh,
```

- [ ] **Step 5: Delete tests covering the removed fields**

```bash
grep -n "OAuthClient\|OAuthRefresh\|oauth-client\|oauth-refresh" \
    /home/glw907/Projects/poplar/internal/config/accounts_test.go
```

For each match, remove the line if it's an assertion, or remove the whole test case if the case exists solely to validate OAuth-field round-tripping. Do not introduce stub assertions on `Password` to "preserve coverage" — Pass 9.6 reintroduces with real cases.

- [ ] **Step 6: Build + test**

```bash
cd /home/glw907/Projects/poplar && go build ./... && go test ./internal/config/...
```

Expected: builds clean; all `internal/config` tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/config/
git commit -m "$(cat <<'EOF'
Pass 8.1: strip unused OAuth account fields

OAuthClientID/Secret/RefreshToken were decoded from TOML but
never consumed. Pre-beta stance: delete dead fields; the wizard
pass (9.6) re-adds them with a real consumer (internal token
endpoint exchange).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add `Provider.GmailQuirks` and `gmail` preset

**Files:**
- Modify: `internal/config/providers.go`
- Modify: `internal/config/account.go` (add `GmailQuirks bool`)
- Modify: `internal/config/accounts.go` (copy quirks from preset)
- Modify: `internal/config/providers_test.go`

- [ ] **Step 1: Write the failing preset test**

Append to `internal/config/providers_test.go` (create the file if absent — match the pattern of other `_test.go` files in this package):

```go
func TestLookupProvider_Gmail(t *testing.T) {
	p, ok := LookupProvider("gmail")
	if !ok {
		t.Fatal("gmail preset missing")
	}
	if p.Backend != "imap" {
		t.Errorf("Backend = %q, want %q", p.Backend, "imap")
	}
	if p.Host != "imap.gmail.com" {
		t.Errorf("Host = %q, want %q", p.Host, "imap.gmail.com")
	}
	if p.Port != 993 {
		t.Errorf("Port = %d, want 993", p.Port)
	}
	if p.AuthHint != "xoauth2" {
		t.Errorf("AuthHint = %q, want %q", p.AuthHint, "xoauth2")
	}
	if !p.GmailQuirks {
		t.Errorf("GmailQuirks = false, want true")
	}
}
```

If `providers_test.go` does not exist, create with header:

```go
// SPDX-License-Identifier: MIT

package config

import "testing"
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/config/ -run TestLookupProvider_Gmail
```

Expected: FAIL — either "gmail preset missing" or compile error on `p.GmailQuirks`.

- [ ] **Step 3: Add `GmailQuirks` to `Provider`**

In `internal/config/providers.go`, after the `InsecureTLS` field:

```go
	InsecureTLS bool   // true only for self-signed-cert presets (self-hosted)
	GmailQuirks bool   // X-GM-EXT-1 + Trash-precondition-on-EXPUNGE
	URL         string // JMAP presets only
```

- [ ] **Step 4: Add the `gmail` preset entry**

In the `Providers` map (`internal/config/providers.go`), add — keep the alphabetical-ish ordering consistent with existing entries (insert after `gmx`, before `protonmail`):

```go
	"gmail": {
		Name:        "gmail",
		Backend:     "imap",
		Host:        "imap.gmail.com",
		Port:        993,
		AuthHint:    "xoauth2",
		GmailQuirks: true,
		HelpURL:     "https://support.google.com/mail/answer/7126229",
	},
```

- [ ] **Step 5: Add `GmailQuirks` to `AccountConfig`**

In `internal/config/account.go`, after `InsecureTLS`:

```go
	// InsecureTLS skips TLS certificate verification. Intended for
	// self-hosted IMAP servers with self-signed certs and local
	// development (e.g., Dovecot in Docker). Never set for hosted
	// providers.
	InsecureTLS bool

	// GmailQuirks enables Gmail-specific IMAP behavior in mailimap:
	// X-GM-EXT-1 assertion at Connect, and Destroy routed via
	// SELECT [Gmail]/Trash before EXPUNGE so EXPUNGE truly deletes.
	// Set automatically by the gmail preset; never set by hand.
	GmailQuirks bool
```

- [ ] **Step 6: Copy `GmailQuirks` from preset → account**

In `internal/config/accounts.go`, inside the `if preset, ok := LookupProvider(...)` block (right after `insecureTLS = preset.InsecureTLS`), add a local var:

```go
	gmailQuirks := false
	if preset, ok := LookupProvider(e.Provider); ok {
		// ...existing assignments...
		if !insecureTLS {
			insecureTLS = preset.InsecureTLS
		}
		gmailQuirks = preset.GmailQuirks
		if source == "" {
			source = preset.URL
		}
	}
```

Then add to the `&AccountConfig{...}` literal (next to `InsecureTLS: insecureTLS`):

```go
		GmailQuirks: gmailQuirks,
```

Note: `gmailQuirks` is declared at function scope before the preset lookup so it's in scope at struct-literal time. Match the existing style.

- [ ] **Step 7: Run the preset test — should pass**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/config/ -run TestLookupProvider_Gmail
```

Expected: PASS.

- [ ] **Step 8: Add the preset → account flow test**

Append to `internal/config/accounts_test.go`:

```go
func TestParseAccounts_GmailPresetCopiesQuirks(t *testing.T) {
	const cfg = `
[[account]]
name        = "g"
provider    = "gmail"
email       = "me@example.com"
password    = "tok"
`
	accts, err := ParseAccountsFromBytes([]byte(cfg))
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	if len(accts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accts))
	}
	a := accts[0]
	if a.Backend != "imap" {
		t.Errorf("Backend = %q, want imap", a.Backend)
	}
	if a.Host != "imap.gmail.com" {
		t.Errorf("Host = %q", a.Host)
	}
	if !a.GmailQuirks {
		t.Errorf("GmailQuirks = false, want true")
	}
}
```

- [ ] **Step 9: Run all config tests**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/config/
git commit -m "$(cat <<'EOF'
Pass 8.1: add gmail provider preset with GmailQuirks flag

Provider gains GmailQuirks bool. The gmail preset sets it; the
flag is copied onto AccountConfig during preset resolution and
will gate Gmail-specific IMAP behavior in the next commit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Update config template + golden

The current template promises an internal OAuth flow. Reality after this pass: Gmail/Outlook auth via `password-cmd` returning a fresh access token. Update the prose so the template-write-on-first-run does not mislead.

**Files:**
- Modify: `internal/config/template.go:86-92`
- Regenerate: `internal/config/template.golden`

- [ ] **Step 1: Replace the OAuth-providers block**

In `internal/config/template.go`, locate the `# OAuth providers` block (around line 86):

```
# OAuth providers
#
#   gmail and outlook authenticate via OAuth, not a password.
#   poplar runs the OAuth flow on first connect and caches the
#   refresh token via your `+ "`password-cmd`" + `. See:
#
#       https://github.com/glw907/poplar/blob/master/docs/oauth.md
```

Replace with:

```
# OAuth providers
#
#   gmail and outlook authenticate with a short-lived access
#   token (XOAUTH2), not a password. Until poplar's first-run
#   wizard ships, set ` + "`password-cmd`" + ` to a command that prints a
#   fresh access token to stdout. Examples:
#
#       oauth2l fetch --type=oauth2 --output_format=bare \
#           --credentials=$HOME/.config/poplar/gmail-client.json \
#           https://mail.google.com/
#
#       op read op://Personal/Gmail-XOAUTH2/access-token
#
#   The token expires every hour; password-cmd is re-run on each
#   reconnect, so wire it to a refresher that returns a current
#   token (most CLIs above handle the refresh transparently).
```

- [ ] **Step 2: Regenerate the golden**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/config/ -run TestTemplate_Golden -update
```

If the test does not support `-update`, regenerate manually:

```bash
cd /home/glw907/Projects/poplar && go run ./cmd/poplar config init --force --config /tmp/poplar-template.toml \
  && cp /tmp/poplar-template.toml internal/config/template.golden
```

(Check `template_test.go` for the actual update mechanism before doing either.)

- [ ] **Step 3: Verify the golden test passes**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/config/ -run Template
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/config/template.go internal/config/template.golden
git commit -m "$(cat <<'EOF'
Pass 8.1: rewrite template OAuth note to match reality

XOAUTH2 access tokens come from password-cmd; an in-app OAuth
flow lands with the first-run wizard (Pass 9.6).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `capSet.XGM` + Connect-time assertion

**Files:**
- Modify: `internal/mailimap/imap.go:43-48, 110-123`
- Modify: `internal/mailimap/imap_test.go` (add cases)

- [ ] **Step 1: Write the failing test**

Append to `internal/mailimap/imap_test.go`:

```go
func TestFinishConnect_GmailQuirks_RequiresXGM(t *testing.T) {
	cfg := config.AccountConfig{Name: "g", Backend: "imap", GmailQuirks: true}
	b := New(cfg)
	fc := newFakeClient()
	fc.caps = map[string]bool{"UIDPLUS": true} // X-GM-EXT-1 absent
	b.cmd = fc
	b.idle = newFakeClient()

	err := b.finishConnect(context.Background())
	if err == nil {
		t.Fatal("finishConnect with GmailQuirks and no X-GM-EXT-1: want error, got nil")
	}
	if !strings.Contains(err.Error(), "X-GM-EXT-1") {
		t.Errorf("error %q does not mention X-GM-EXT-1", err)
	}
}

func TestFinishConnect_GmailQuirks_AcceptsXGM(t *testing.T) {
	cfg := config.AccountConfig{Name: "g", Backend: "imap", GmailQuirks: true}
	b := New(cfg)
	fc := newFakeClient()
	fc.caps = map[string]bool{"UIDPLUS": true, "X-GM-EXT-1": true}
	b.cmd = fc
	b.idle = newFakeClient()

	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("finishConnect: %v", err)
	}
	// Tear down the idle goroutine started by finishConnect.
	_ = b.Disconnect()
}
```

Adjust imports at the top of the file if `context` or `strings` aren't already imported.

- [ ] **Step 2: Run to verify it fails**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/ -run TestFinishConnect_GmailQuirks
```

Expected: FAIL — either compile error on `GmailQuirks`/`XGM` or the assertion succeeds when it shouldn't.

- [ ] **Step 3: Add `XGM` to `capSet`**

In `internal/mailimap/imap.go`, replace:

```go
type capSet struct {
	UIDPLUS    bool
	MOVE       bool
	IDLE       bool
	SpecialUse bool
}
```

with:

```go
type capSet struct {
	UIDPLUS    bool
	MOVE       bool
	IDLE       bool
	SpecialUse bool
	XGM        bool // X-GM-EXT-1 (Gmail extensions)
}
```

- [ ] **Step 4: Populate `XGM` and assert**

In `finishConnect`, replace:

```go
	cs := capSet{
		UIDPLUS:    caps["UIDPLUS"],
		MOVE:       caps["MOVE"],
		IDLE:       caps["IDLE"],
		SpecialUse: caps["SPECIAL-USE"],
	}
	if !cs.UIDPLUS {
		return errors.New("server does not advertise UIDPLUS — required for safe deletion")
	}
```

with:

```go
	cs := capSet{
		UIDPLUS:    caps["UIDPLUS"],
		MOVE:       caps["MOVE"],
		IDLE:       caps["IDLE"],
		SpecialUse: caps["SPECIAL-USE"],
		XGM:        caps["X-GM-EXT-1"],
	}
	if !cs.UIDPLUS {
		return errors.New("server does not advertise UIDPLUS — required for safe deletion")
	}
	if b.cfg.GmailQuirks && !cs.XGM {
		return errors.New("gmail account but server does not advertise X-GM-EXT-1")
	}
```

- [ ] **Step 5: Run the new tests**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/ -run TestFinishConnect_GmailQuirks
```

Expected: PASS.

- [ ] **Step 6: Run the full mailimap test suite**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/...
```

Expected: PASS — no other tests rely on the absence of `X-GM-EXT-1`.

- [ ] **Step 7: Commit**

```bash
git add internal/mailimap/
git commit -m "$(cat <<'EOF'
Pass 8.1: assert X-GM-EXT-1 at Connect for Gmail accounts

capSet gains XGM; finishConnect rejects GmailQuirks accounts on
servers that do not advertise X-GM-EXT-1. The bit is also
populated for non-Gmail accounts (it costs nothing) so Pass 8.6
can use it for label-aware attachment listings if needed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Gmail-quirks `Destroy` routes via `[Gmail]/Trash`

**Files:**
- Modify: `internal/mailimap/actions.go:96-115`
- Modify: `internal/mailimap/actions_test.go` (add cases)
- Modify: `internal/mailimap/README.md` (document the contract)

- [ ] **Step 1: Write the failing test**

Append to `internal/mailimap/actions_test.go`. The fakeClient has the `selected`, `storeCalls`, `expungeCalls` fields needed; the existing `[]listEntry` shape may need a mock Trash entry — peek at neighboring tests for the pattern:

```go
func TestDestroy_GmailQuirks_SelectsTrashFirst(t *testing.T) {
	cfg := config.AccountConfig{Name: "g", Backend: "imap", GmailQuirks: true}
	b := New(cfg)
	fc := newFakeClient()
	// Provide a Trash folder so resolveTrashFolder finds it.
	fc.folders = []listEntry{
		{name: "INBOX", attrs: nil},
		{name: "[Gmail]/Trash", attrs: []string{"\\Trash"}},
	}
	b.cmd = fc
	b.caps = capSet{UIDPLUS: true, XGM: true, SpecialUse: true}

	uids := []mail.UID{"10", "11"}
	if err := b.Destroy(uids); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if fc.selected != "[Gmail]/Trash" {
		t.Errorf("selected = %q, want %q", fc.selected, "[Gmail]/Trash")
	}
	if len(fc.storeCalls) != 1 {
		t.Fatalf("storeCalls = %d, want 1", len(fc.storeCalls))
	}
	if len(fc.expungeCalls) != 1 {
		t.Fatalf("expungeCalls = %d, want 1", len(fc.expungeCalls))
	}
}

func TestDestroy_NonQuirks_DoesNotSelect(t *testing.T) {
	cfg := config.AccountConfig{Name: "f", Backend: "imap"}
	b := New(cfg)
	fc := newFakeClient()
	b.cmd = fc
	b.caps = capSet{UIDPLUS: true}

	if err := b.Destroy([]mail.UID{"7"}); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if fc.selected != "" {
		t.Errorf("selected = %q, want empty (no Select expected)", fc.selected)
	}
	if len(fc.storeCalls) != 1 || len(fc.expungeCalls) != 1 {
		t.Errorf("expected one Store + one UIDExpunge, got store=%d expunge=%d",
			len(fc.storeCalls), len(fc.expungeCalls))
	}
}
```

(Verify `listEntry` field names — `name` / `attrs` may differ. Open `internal/mailimap/folders.go` and match.)

- [ ] **Step 2: Run to verify it fails**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/ -run TestDestroy_GmailQuirks_SelectsTrashFirst
```

Expected: FAIL — `fc.selected` is empty (Destroy doesn't Select today).

- [ ] **Step 3: Branch `Destroy` on GmailQuirks**

In `internal/mailimap/actions.go`, replace the entire `Destroy` function:

```go
// Destroy satisfies mail.Backend. Permanently deletes via STORE \Deleted
// then UID EXPUNGE. Per ADR-0092: empty input is a no-op, the
// operation is irreversible, missing UIDs are treated as success
// (the server silently ignores them).
//
// On Gmail (b.caps.GmailQuirks), EXPUNGE outside [Gmail]/Trash only
// removes labels — it does not delete. Destroy on a Gmail backend
// therefore selects [Gmail]/Trash before STORE+EXPUNGE. The caller
// must pass UIDs that already live in Trash; both real callers
// (manual Empty Trash per ADR-0094, retention sweep per ADR-0093)
// satisfy this because they only trigger inside Disposal folders.
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	gmail := b.cfg.GmailQuirks
	b.mu.Unlock()

	if gmail {
		trash, err := b.resolveTrashFolder()
		if err != nil {
			return fmt.Errorf("destroy: %w", err)
		}
		if _, err := cmd.Select(trash, false); err != nil {
			return fmt.Errorf("select trash: %w", err)
		}
	}

	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("store deleted: %w", err)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("uid expunge: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run the destroy tests**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/ -run TestDestroy
```

Expected: PASS for both new tests; existing Destroy tests still PASS.

- [ ] **Step 5: Run the full mailimap suite**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/...
```

Expected: PASS.

- [ ] **Step 6: Document the Gmail Destroy contract in README**

In `internal/mailimap/README.md`, add a short section after the existing capabilities note (or create one if absent):

```markdown
### Gmail (`GmailQuirks = true`)

The `gmail` provider preset sets `GmailQuirks` on the
`AccountConfig`. The IMAP backend then:

- Asserts `X-GM-EXT-1` at Connect; refuses to start without it.
- Routes `Destroy(uids)` through `SELECT [Gmail]/Trash` before
  `STORE \Deleted` + `UID EXPUNGE`. Gmail's IMAP server only
  permanently deletes when the SELECTed mailbox is `[Gmail]/Trash`;
  EXPUNGE elsewhere just removes the matching label. Callers must
  pass UIDs that already live in Trash — both real callers
  (manual Empty Trash, retention sweep) operate from inside
  Disposal folders, so this is satisfied.

XOAUTH2 access tokens are short-lived (~1h). Poplar does not
refresh them internally yet; wire `password-cmd` to a refresher
(`oauth2l`, `op`, etc.). Internal token-endpoint exchange lands
with the first-run wizard.
```

- [ ] **Step 7: Commit**

```bash
git add internal/mailimap/
git commit -m "$(cat <<'EOF'
Pass 8.1: Gmail Destroy routes via SELECT [Gmail]/Trash

Gmail's EXPUNGE only deletes when the selected mailbox is
[Gmail]/Trash; elsewhere it just removes a label. Destroy on a
GmailQuirks backend selects Trash first, then STORE+EXPUNGE.
Callers (manual Empty, retention sweep) already operate from
Trash, so the UID contract is preserved.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: XOAUTH2 dial bypasses the password cache

For `cfg.Auth == "xoauth2"`, the access token returned by `password-cmd` is short-lived. Caching it on `b.password` means any reconnect after token expiry uses a stale token. Solution: skip cache reads + writes for XOAUTH2 accounts so each dial gets a fresh value.

**Files:**
- Modify: `internal/mailimap/auth.go:180-195` (`resolvedPassword`)
- Modify: `internal/mailimap/password_test.go` (add case)

- [ ] **Step 1: Write the failing test**

Append to `internal/mailimap/password_test.go`:

```go
func TestResolvedPassword_XOAUTH2_BypassesCache(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "n")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Each invocation increments the counter file and prints the new value.
	cmd := fmt.Sprintf(
		`n=$(cat %q); n=$((n+1)); printf %%s "$n" > %q; printf %%s "$n"`,
		counter, counter)

	b := New(config.AccountConfig{
		Name:        "g",
		Auth:        "xoauth2",
		PasswordCmd: cmd,
	})

	first, err := b.resolvedPassword()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := b.resolvedPassword()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first == second {
		t.Errorf("xoauth2 should not cache: first=%q second=%q (want different)", first, second)
	}
}

func TestResolvedPassword_NonXOAUTH2_Caches(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "n")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := fmt.Sprintf(
		`n=$(cat %q); n=$((n+1)); printf %%s "$n" > %q; printf %%s "$n"`,
		counter, counter)

	b := New(config.AccountConfig{
		Name:        "f",
		Auth:        "plain",
		PasswordCmd: cmd,
	})

	first, _ := b.resolvedPassword()
	second, _ := b.resolvedPassword()
	if first != second {
		t.Errorf("plain auth should cache: first=%q second=%q (want equal)", first, second)
	}
}
```

Adjust imports if `os`, `fmt`, `filepath` are not already imported in this test file.

- [ ] **Step 2: Run to verify it fails**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/ -run TestResolvedPassword_XOAUTH2_BypassesCache
```

Expected: FAIL — `first == second` (cached).

- [ ] **Step 3: Add the XOAUTH2 short-circuit**

In `internal/mailimap/auth.go`, replace the `resolvedPassword` body:

```go
// resolvedPassword returns the cached password for b, resolving it on
// the first call. The cached value is stored under b.mu so reconnects
// within the session reuse the same credential without re-running the cmd.
//
// XOAUTH2 access tokens are short-lived (~1h on Gmail). For
// cfg.Auth == "xoauth2", the cache is bypassed: every dial re-runs
// password-cmd to fetch a current token. Internal refresh against
// the provider's token endpoint will land with the first-run wizard
// (Pass 9.6).
func (b *Backend) resolvedPassword() (string, error) {
	if b.cfg.Auth == "xoauth2" {
		return resolvePassword(&b.cfg)
	}
	b.mu.Lock()
	cached := b.password
	b.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	pw, err := resolvePassword(&b.cfg)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	b.password = pw
	b.mu.Unlock()
	return pw, nil
}
```

- [ ] **Step 4: Run the new tests**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/ -run TestResolvedPassword
```

Expected: PASS.

- [ ] **Step 5: Run the full mailimap suite**

```bash
cd /home/glw907/Projects/poplar && go test ./internal/mailimap/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/mailimap/
git commit -m "$(cat <<'EOF'
Pass 8.1: bypass password cache for XOAUTH2 accounts

Gmail access tokens last ~1h; caching the first one across
reconnects breaks the second hour of any session. XOAUTH2
accounts now re-run password-cmd on every dial. Internal
token-endpoint refresh waits for the first-run wizard.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Pass-end consolidation

This is the standard `poplar-pass` ritual. Run the steps in order; each is its own commit.

- [ ] **Step 1: `/simplify`**

Run the `simplify` skill against the diff since `master`'s prior pass-end commit. Apply genuine wins; ignore stylistic noise. Commit any non-trivial simplifications separately.

- [ ] **Step 2: Update `docs/poplar/invariants.md`**

The "Backends in v1" bullet needs updating. Locate the line listing IMAP presets:

```
or one of the presets `yahoo`, `icloud`, `zoho`, `outlook`, `mailbox-org`, `posteo`, `runbox`, `gmx`, `protonmail`; `gmail` lands with X-GM-EXT support in Pass 8.1)
```

Replace the parenthetical with text reflecting Gmail-as-shipped:

```
or one of the presets `yahoo`, `icloud`, `zoho`, `outlook`, `mailbox-org`, `posteo`, `runbox`, `gmx`, `protonmail`, `gmail`)
```

Then under the "IMAP backend invariants" paragraph, add a sentence at the end:

```
Gmail accounts (`GmailQuirks = true`) additionally assert
`X-GM-EXT-1` at Connect and route `Destroy` through `SELECT
[Gmail]/Trash` before `STORE \Deleted` + `UID EXPUNGE` so EXPUNGE
truly deletes; XOAUTH2 access tokens come from `password-cmd`
with no internal refresh until Pass 9.6.
```

Update the **Decision index** table — the `JMAP + IMAP only, minimal account config` row gains 0106; add a new row for the new ADRs:

```
| Gmail preset, X-GM-EXT-1 assertion, Destroy routing, XOAUTH2 via password-cmd | 0106, 0107, 0108 |
```

Verify the file is still ≤ 300 lines (`wc -l docs/poplar/invariants.md`).

- [ ] **Step 3: Write ADR-0106 — Gmail preset and X-GM-EXT-1 assertion**

Create `docs/poplar/decisions/0106-gmail-preset-and-xgm-assertion.md`:

```markdown
---
title: Gmail preset and X-GM-EXT-1 assertion
status: accepted
date: 2026-05-02
---

## Context

ADR-0098 / ADR-0101 set the provider-registry shape and named
Gmail as a Pass 8.1 preset. The IMAP backend (ADR-0099/0100) is
generic; Gmail's IMAP server has well-known eccentricities
(EXPUNGE-only-deletes-in-Trash, label-as-folder semantics) that
must be gated to Gmail accounts only.

## Decision

`config.Provider` gains `GmailQuirks bool`. The `gmail` preset is:

```
"gmail": {
    Name: "gmail", Backend: "imap",
    Host: "imap.gmail.com", Port: 993,
    AuthHint: "xoauth2", GmailQuirks: true,
}
```

The flag is copied onto `AccountConfig` during preset resolution
(mirroring `InsecureTLS`). The IMAP backend's `capSet` gains
`XGM bool`, populated from `caps["X-GM-EXT-1"]`. When
`b.cfg.GmailQuirks && !cs.XGM`, `finishConnect` returns an error
— a Gmail account on a server without X-GM-EXT-1 is a
misconfiguration, not a fallback case.

## Consequences

- Other Gmail-specific paths (Destroy routing in ADR-0107) gate on
  the same flag with the same name in both places.
- Non-Gmail backends still populate `cs.XGM` (from `caps[...]`),
  but it is never read outside Gmail-quirks code paths today.
- Future presets that need quirks add another bool rather than
  reusing GmailQuirks. We have the budget; pre-beta cleanups can
  unify if a pattern emerges.
```

- [ ] **Step 4: Write ADR-0107 — Gmail Destroy routing**

Create `docs/poplar/decisions/0107-gmail-destroy-routing.md`:

```markdown
---
title: Gmail Destroy routes via SELECT [Gmail]/Trash
status: accepted
date: 2026-05-02
---

## Context

ADR-0100 specified the IMAP `Destroy` mapping (STORE \Deleted +
UID EXPUNGE) and explicitly deferred Gmail's quirk: EXPUNGE only
truly deletes when the SELECTed mailbox is `[Gmail]/Trash`;
elsewhere it removes the matching label. Pass 8.1 implements that
quirk.

## Decision

`mailimap.Backend.Destroy(uids)` branches on `b.cfg.GmailQuirks`.
Generic path is unchanged. Gmail path:

1. Resolve Trash via `resolveTrashFolder()`.
2. `cmd.Select(trash, false)`.
3. `cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"})`.
4. `cmd.UIDExpunge(uids)`.

The caller contract: UIDs must reference messages already in
`[Gmail]/Trash`. Both real callers — manual Empty Trash
(ADR-0094) and the per-session retention sweep (ADR-0093) —
satisfy this because they only trigger inside Disposal folders.
A caller that violates the contract gets a NO/BAD response from
the server (UID not in Trash), which surfaces as a clear error
rather than silent data loss.

No selection-restore step. Every other backend method
(`OpenFolder`, `QueryFolder`, …) issues its own `Select` before
acting, so leaving the cmd connection on Trash after `Destroy`
costs nothing.

## Consequences

- Gmail's "delete inside Trash" semantic now matches the JMAP
  `Email/set { destroy }` semantic from `mailjmap`.
- Future "permanent delete from arbitrary folder" UIs would need
  to MOVE-then-Destroy on Gmail; not in scope for v1.
- `internal/mailimap/README.md` documents the contract.
```

- [ ] **Step 5: Write ADR-0108 — XOAUTH2 access tokens via password-cmd**

Create `docs/poplar/decisions/0108-xoauth2-access-tokens-via-password-cmd.md`:

```markdown
---
title: XOAUTH2 access tokens via password-cmd
status: accepted
date: 2026-05-02
---

## Context

Gmail / Outlook XOAUTH2 access tokens expire (~1h on Gmail). The
IMAP backend caches `password-cmd` output on `b.password` so
reconnects within a session don't re-run the user's secret-
manager command. That cache breaks XOAUTH2: hour two of any
session sends a stale token.

`AccountConfig` previously carried `OAuthClientID`,
`OAuthClientSecret`, `OAuthRefreshToken`, decoded from TOML but
never read. The intent was an internal token-endpoint refresh —
never wired up because no UI captures the values.

## Decision

For `cfg.Auth == "xoauth2"`, `Backend.resolvedPassword()` skips
the cache: every dial re-runs `password-cmd`. Users wire any
refresher (`oauth2l`, `op`, a custom script) into `password-cmd`
and get a fresh token on every connection.

The unused `OAuth*` fields are deleted from `AccountConfig`,
the TOML decode struct, and the template. Pre-beta stance
(CLAUDE.md): strip dead fields; the consumer re-adds when ready.
The first-run wizard (Pass 9.6) reintroduces them — alongside
the HTTP exchange against Google's token endpoint — when there
is a UI to populate them.

## Consequences

- Reconnects on slow `password-cmd` block on the refresher each
  time. Acceptable pre-beta; documented in the template.
- The XOAUTH2 SASL mechanism (`internal/mailauth/xoauth2.go`) is
  unchanged.
- Internal refresh, when it lands, will live in `internal/mailauth/`
  as a `TokenSource` consumed by `dialCommand`/`dialIdle` —
  parallel to (not replacing) the `password-cmd` path.
```

- [ ] **Step 6: Update `docs/poplar/STATUS.md`**

Mark Pass 8.1 done in the table; replace the starter prompt with the next pass (8.2 — bubbletea cleanup II). The entries currently in the pass table for 8.2 give the goal; format the prompt to match the existing style at the bottom of the file.

Verify ≤ 60 lines (`wc -l docs/poplar/STATUS.md`).

- [ ] **Step 7: Archive plan + spec**

```bash
cd /home/glw907/Projects/poplar
git mv docs/superpowers/plans/2026-05-02-gmail-preset.md \
       docs/superpowers/archive/plans/2026-05-02-gmail-preset.md
git mv docs/superpowers/specs/2026-05-02-gmail-preset-design.md \
       docs/superpowers/archive/specs/2026-05-02-gmail-preset-design.md
```

- [ ] **Step 8: `make check`**

```bash
cd /home/glw907/Projects/poplar && make check
```

Expected: PASS (vet + tests across the module).

- [ ] **Step 9: Commit + push + install**

```bash
cd /home/glw907/Projects/poplar
git add -A
git commit -m "$(cat <<'EOF'
Pass 8.1 consolidation: ADR-0106/0107/0108, invariants, archive

Gmail preset shipped. ADRs codify the X-GM-EXT-1 assertion,
Destroy routing via SELECT [Gmail]/Trash, and the XOAUTH2-via-
password-cmd decision (with the OAuth* field strip). Invariants
updated, plan + spec archived. Next pass: 8.2 bubbletea cleanup.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git push
make install
```

Expected: clean push, `~/.local/bin/poplar` rebuilt.

---

## Self-review

- **Spec coverage:** every spec section maps to a task — Task 1 (strip OAuth fields), Task 2 (preset + GmailQuirks plumbing), Task 3 (template), Task 4 (XGM cap), Task 5 (Destroy routing), Task 6 (XOAUTH2 cache bypass), Task 7 (ADRs + invariants + archive).
- **Placeholder scan:** clean. No TBD/TODO; every code change shows the code; every test has an assertion.
- **Type consistency:** `GmailQuirks bool` — same name on `Provider`, `AccountConfig`, copied as a single bool. `capSet.XGM` — same name in the test, the struct, and the read-site. `cfg.Auth == "xoauth2"` — same string literal in `auth.go` and `resolvedPassword`.
