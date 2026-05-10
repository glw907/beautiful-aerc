# Pass 14 — First-run wizard implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first-run wizard (#27), the malformed-config error path (#29), and the supporting probe infrastructure. OAuth refresh and the Gmail/Outlook flow defer to Pass 14.1.

**Architecture:** Section-pluggable wizard. `internal/wizard/` is the UI-free domain (Sections, Strategies, Probe dispatcher); `internal/ui/wizard/` is the bubbletea + huh UI. New `CredentialStrategy` enum on `config.Provider` routes per-provider. `mailimap.Probe` and `mailjmap.Probe` produce step-by-step transcripts the probe screen renders live.

**Tech Stack:** Go 1.26, bubbletea, charm.land/huh/v2 (new dep), lipgloss, charmbracelet/bubbles. Existing: `internal/config`, `internal/theme`, `internal/mailimap`, `internal/mailjmap`, `internal/uicore`.

**Spec:** `docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md` — read before starting.

**Skills to invoke:**
- `go-conventions` before any Go file edit.
- `elm-conventions` before any `internal/ui/` file edit.
- `bubbletea-conventions` (load `docs/poplar/bubbletea-conventions.md`) before UI work.
- `simplify` after each task before commit.

---

## File structure

| Path | Status | Responsibility |
|---|---|---|
| `internal/wizard/model.go` | new | `Model`, `Step`, value types |
| `internal/wizard/section.go` | new | `Section` interface |
| `internal/wizard/strategy.go` | new | `CredentialStrategy` enum, `Strategy` interface, non-OAuth impls |
| `internal/wizard/apply.go` | new | `Apply(model) (config.AccountConfig, error)` |
| `internal/wizard/probe.go` | new | `Probe(ctx, cfg) ProbeResult` dispatcher; `ProbeStep` value type |
| `internal/wizard/probe_test.go` | new | dispatcher routing tests |
| `internal/wizard/apply_test.go` | new | conversion tests |
| `internal/wizard/strategy_test.go` | new | strategy selection tests |
| `internal/mailimap/probe.go` | new | `Probe(ctx, cfg) ProbeResult` step-by-step transcript |
| `internal/mailimap/probe_test.go` | new | probe transcript tests |
| `internal/mailjmap/probe.go` | new | `Probe(ctx, cfg) ProbeResult` step-by-step transcript |
| `internal/mailjmap/probe_test.go` | new | probe transcript tests |
| `internal/mail/probe.go` | new | shared `ProbeResult`, `ProbeStep`, `ProbeStatus` types |
| `internal/config/providers.go` | modify | add `CredentialStrategy`, `HelpURL` to `Provider` |
| `internal/config/accounts.go` | modify | drop empty-name check (#29 fix); make `name` default to `email` |
| `internal/config/errors.go` | new | `ConfigError` typed wrapper |
| `internal/config/template.go` | modify | rewrite OAuth section, drop wizard-doesn't-exist comments |
| `internal/config/template.golden` | modify | mirror template.go changes |
| `internal/config/writer.go` | modify | `Render(accts, ui, cache) []byte` |
| `internal/ui/wizard/model.go` | new | wizard `tea.Model` |
| `internal/ui/wizard/styles.go` | new | per-subpackage `Styles` from `theme.CompiledTheme` |
| `internal/ui/wizard/theme_adapter.go` | new | `huh.Theme` adapter from `theme.CompiledTheme` |
| `internal/ui/wizard/logo.go` | new | typographic wordmark; `art/poplar-logo.ans` embedded but unused |
| `internal/ui/wizard/sections.go` | new | section registry |
| `internal/ui/wizard/section_account.go` | new | account section impl |
| `internal/ui/wizard/section_theme.go` | new | theme section impl with live preview |
| `internal/ui/wizard/probe_screen.go` | new | custom `tea.Model` for probe transcript |
| `internal/ui/wizard/msgs.go` | new | cross-boundary `tea.Msg` types |
| `internal/ui/wizard/model_test.go` | new | Update tests with synthesized Msgs |
| `cmd/poplar/config_init.go` | new | `config init --interactive` cobra subcommand |
| `cmd/poplar/root.go` | modify | first-run auto-launch wizard, `--repair=<name>`, `--no-wizard` |
| `art/poplar-logo.ans` | new | cbonsai output (committed for future logo swap) |
| `docs/poplar/decisions/0189-firstrun-wizard.md` | new | ADR |
| `docs/poplar/invariants.md` | modify | wizard, sections, probe, ConfigError invariants |
| `docs/poplar/decisions/INDEX.md` | modify | ADR-0189 row |

---

## Pre-task setup

- [ ] **Step 0.1: Read the spec**

Read `docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md` end-to-end. Read `docs/poplar/invariants.md` (relevant sections: Architecture > Repo & libraries, Architecture > Elm architecture & idiomatic bubbletea, Config & theming, Mail model). Read `docs/poplar/bubbletea-conventions.md`.

- [ ] **Step 0.2: Verify clean working tree**

```bash
cd /home/glw907/Projects/poplar
git status
```

Expected: `nothing to commit, working tree clean`. If dirty, stash or commit first.

- [ ] **Step 0.3: Add huh dependency**

```bash
go get charm.land/huh/v2@latest
```

Verify in `go.mod`:

```bash
grep huh go.mod
```

Expected: a `charm.land/huh/v2 v2.x.y` line.

- [ ] **Step 0.4: Commit dep addition**

```bash
git add go.mod go.sum
git commit -m "Pass 14: add charm.land/huh/v2 dependency"
```

---

## Task 1: Probe value types in `internal/mail`

**Files:**
- Create: `internal/mail/probe.go`
- Create: `internal/mail/probe_test.go`

Shared types live in `internal/mail` so both `mailimap` and `mailjmap` import them without cycle.

- [ ] **Step 1.1: Write failing test**

`internal/mail/probe_test.go`:

```go
package mail

import "testing"

func TestProbeResultOK(t *testing.T) {
	r := ProbeResult{
		Steps: []ProbeStep{
			{Label: "connect", Status: ProbeOK},
			{Label: "auth", Status: ProbeOK},
		},
	}
	if !r.OK() {
		t.Fatalf("OK() = false, want true")
	}
}

func TestProbeResultFailedStep(t *testing.T) {
	r := ProbeResult{
		Steps: []ProbeStep{
			{Label: "connect", Status: ProbeOK},
			{Label: "auth", Status: ProbeFail, Detail: "bad creds"},
		},
		Err: errAuthFailed,
	}
	if r.OK() {
		t.Fatalf("OK() = true, want false")
	}
	if r.Err == nil {
		t.Fatalf("Err = nil, want non-nil")
	}
}

var errAuthFailed = stringErr("auth failed")

type stringErr string

func (s stringErr) Error() string { return string(s) }
```

- [ ] **Step 1.2: Run test, expect compile failure**

```bash
cd /home/glw907/Projects/poplar
go test ./internal/mail/... -run TestProbeResult -v
```

Expected: build failure — undefined `ProbeResult`, `ProbeStep`, `ProbeOK`, `ProbeFail`.

- [ ] **Step 1.3: Implement**

`internal/mail/probe.go`:

```go
package mail

// ProbeStatus is the outcome of one probe step.
type ProbeStatus int

const (
	ProbePending ProbeStatus = iota
	ProbeOK
	ProbeFail
	ProbeSkip
)

// ProbeStep is one entry in a probe transcript.
type ProbeStep struct {
	Label  string // human-readable, e.g. "TLS handshake"
	Status ProbeStatus
	Detail string // optional, e.g. "1,247 messages" or "bad credentials"
}

// ProbeResult is the wizard's view of a connect-test against a backend.
// Every probe step is recorded so the UI can render the transcript live.
type ProbeResult struct {
	Steps []ProbeStep
	Err   error // non-nil if any step's Status == ProbeFail
}

// OK reports whether all steps succeeded.
func (r ProbeResult) OK() bool {
	if r.Err != nil {
		return false
	}
	for _, s := range r.Steps {
		if s.Status == ProbeFail {
			return false
		}
	}
	return true
}
```

- [ ] **Step 1.4: Run tests, expect pass**

```bash
go test ./internal/mail/... -run TestProbeResult -v
```

Expected: PASS.

- [ ] **Step 1.5: Commit**

```bash
git add internal/mail/probe.go internal/mail/probe_test.go
git commit -m "Pass 14 task 1: shared mail.ProbeResult types"
```

---

## Task 2: `mailimap.Probe` (step-by-step transcript)

**Files:**
- Create: `internal/mailimap/probe.go`
- Create: `internal/mailimap/probe_test.go`

Wrap the existing `Backend.Connect` flow into per-step output. The existing single-shot `Connect` stays — `Probe` is a new entry point that records each step.

- [ ] **Step 2.1: Read existing connect path**

```bash
sed -n '1,80p' internal/mailimap/imap.go
```

Find where `Connect` does: dial, TLS handshake, AUTHENTICATE, CAPABILITY, optional STATUS INBOX. The probe will hook each.

- [ ] **Step 2.2: Write failing test (probe transcript shape)**

`internal/mailimap/probe_test.go`:

```go
package mailimap

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestProbeFastmailIMAP(t *testing.T) {
	// Use the package-local fake (see fake_test.go) by routing through
	// the same dial-injection used elsewhere in this package's tests.
	cfg := config.AccountConfig{
		Name:     "test",
		Provider: "fastmail",
		Email:    "user@fastmail.com",
		Host:     "imap.fastmail.com",
		Port:     993,
	}
	r := Probe(context.Background(), cfg)
	if len(r.Steps) == 0 {
		t.Fatalf("Probe returned no steps")
	}
	wantLabels := []string{"Connecting", "TLS handshake", "AUTHENTICATE", "CAPABILITY (UIDPLUS)", "STATUS INBOX"}
	if len(r.Steps) != len(wantLabels) {
		t.Fatalf("len(Steps) = %d, want %d", len(r.Steps), len(wantLabels))
	}
	for i, want := range wantLabels {
		if r.Steps[i].Label != want {
			t.Errorf("Steps[%d].Label = %q, want %q", i, r.Steps[i].Label, want)
		}
		if r.Steps[i].Status != mail.ProbeOK {
			t.Errorf("Steps[%d].Status = %v, want ProbeOK", i, r.Steps[i].Status)
		}
	}
	if r.Err != nil {
		t.Errorf("Err = %v, want nil", r.Err)
	}
}
```

(See existing `fake_test.go` in `internal/mailimap/` for how the package fakes the dial. Mirror that pattern.)

- [ ] **Step 2.3: Run test, expect compile failure**

```bash
go test ./internal/mailimap/ -run TestProbeFastmailIMAP -v
```

Expected: undefined `Probe`.

- [ ] **Step 2.4: Implement `Probe`**

`internal/mailimap/probe.go`:

```go
package mailimap

import (
	"context"
	"fmt"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// Probe runs a connect-test against an IMAP account, recording each
// step. Mirrors the logic in Backend.Connect but produces a transcript
// so the wizard probe screen can render progress live. SMTP probing
// is separate (ProbeSMTP); call both for full validation.
func Probe(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
	r := mail.ProbeResult{}
	addStep := func(label string) func(err error, detail string) {
		idx := len(r.Steps)
		r.Steps = append(r.Steps, mail.ProbeStep{Label: label, Status: mail.ProbePending})
		return func(err error, detail string) {
			if err != nil {
				r.Steps[idx].Status = mail.ProbeFail
				r.Steps[idx].Detail = err.Error()
				if r.Err == nil {
					r.Err = fmt.Errorf("%s: %w", label, err)
				}
				return
			}
			r.Steps[idx].Status = mail.ProbeOK
			r.Steps[idx].Detail = detail
		}
	}

	// Step 1: connecting (TCP)
	finishConnect := addStep("Connecting")
	conn, err := dialRawTCP(cfg)
	finishConnect(err, fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err != nil {
		return r
	}
	defer conn.Close()

	// Step 2: TLS handshake (or STARTTLS)
	finishTLS := addStep("TLS handshake")
	cli, err := newIMAPClient(conn, cfg)
	finishTLS(err, "")
	if err != nil {
		return r
	}
	defer cli.Close()

	// Step 3: AUTHENTICATE
	finishAuth := addStep("AUTHENTICATE")
	err = authenticateIMAP(cli, cfg)
	finishAuth(err, cfg.Auth)
	if err != nil {
		return r
	}

	// Step 4: CAPABILITY (UIDPLUS asserted)
	finishCap := addStep("CAPABILITY (UIDPLUS)")
	caps, err := cli.Capability()
	if err == nil && !caps.Has("UIDPLUS") {
		err = fmt.Errorf("server lacks UIDPLUS")
	}
	finishCap(err, "")
	if err != nil {
		return r
	}

	// Step 5: STATUS INBOX
	finishStatus := addStep("STATUS INBOX")
	status, err := cli.Status("INBOX", []string{"MESSAGES"})
	detail := ""
	if err == nil {
		detail = fmt.Sprintf("%d messages", status.Messages)
	}
	finishStatus(err, detail)
	return r
}
```

(Names like `dialRawTCP`, `newIMAPClient`, `authenticateIMAP` are
already in this package — reuse them. If signatures differ, adjust the
calls; the structure stays the same: one function call per step,
wrapped in `addStep` / `finish`.)

- [ ] **Step 2.5: Run test, expect pass against fake**

```bash
go test ./internal/mailimap/ -run TestProbeFastmailIMAP -v
```

Expected: PASS. If the fake doesn't cover one of the steps, extend the fake the same way the existing IMAP tests do.

- [ ] **Step 2.6: Add a failure-path test**

Add to `probe_test.go`:

```go
func TestProbeAuthFailure(t *testing.T) {
	// Configure the package-local fake to return AUTHENTICATIONFAILED
	// at the AUTHENTICATE step. Pattern mirrors existing auth_test.go.
	// (Adjust to whatever fake-injection helper this package already
	// has; see fake_test.go.)
	prevDial := imapDial
	defer func() { imapDial = prevDial }()
	imapDial = fakeDialAuthFails

	cfg := config.AccountConfig{
		Name: "t", Provider: "imap", Email: "u@x", Host: "x", Port: 993,
	}
	r := Probe(context.Background(), cfg)
	if r.OK() {
		t.Fatalf("OK() = true on auth failure, want false")
	}
	// First two steps should succeed; AUTHENTICATE should fail.
	if r.Steps[0].Status != mail.ProbeOK || r.Steps[1].Status != mail.ProbeOK {
		t.Fatalf("expected connect+TLS OK, got %+v", r.Steps[:2])
	}
	if r.Steps[2].Status != mail.ProbeFail {
		t.Fatalf("Steps[2].Status = %v, want ProbeFail", r.Steps[2].Status)
	}
	if r.Err == nil {
		t.Fatalf("Err = nil, want non-nil")
	}
}
```

(`fakeDialAuthFails` is the test helper you'll write or extend. Mirror existing fake helpers in `auth_test.go` / `fake_test.go`.)

- [ ] **Step 2.7: Run all mailimap tests**

```bash
go test ./internal/mailimap/ -v
```

Expected: all pass, including new probe tests.

- [ ] **Step 2.8: Commit**

```bash
git add internal/mailimap/probe.go internal/mailimap/probe_test.go
git commit -m "Pass 14 task 2a: mailimap.Probe step-by-step transcript"
```

---

## Task 3: `mailjmap.Probe`

**Files:**
- Create: `internal/mailjmap/probe.go`
- Create: `internal/mailjmap/probe_test.go`

Mirrors task 2 but with JMAP-specific step labels: "Resolving session URL", "TLS handshake", "Bearer authentication", "Session/get", "Account/get".

- [ ] **Step 3.1: Read existing JMAP connect path**

```bash
grep -n "func.*Connect\|Session/get\|Account/get" internal/mailjmap/jmap.go
```

Identify where each step occurs in the existing Connect flow.

- [ ] **Step 3.2: Write failing test**

`internal/mailjmap/probe_test.go` — analogous to mailimap's, but expects labels `["Resolving session URL", "TLS handshake", "Bearer authentication", "Session/get", "Account/get"]`. The Account/get step's Detail should be e.g. `"3 mailboxes"`.

```go
package mailjmap

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestProbeFastmailJMAP(t *testing.T) {
	cfg := config.AccountConfig{
		Name: "fm", Provider: "fastmail",
		Email: "user@fastmail.com",
		// Token comes from PasswordCmd in real usage; the package fake
		// short-circuits this. See fake_test.go.
	}
	r := Probe(context.Background(), cfg)
	wantLabels := []string{
		"Resolving session URL",
		"TLS handshake",
		"Bearer authentication",
		"Session/get",
		"Account/get",
	}
	if len(r.Steps) != len(wantLabels) {
		t.Fatalf("len(Steps) = %d, want %d", len(r.Steps), len(wantLabels))
	}
	for i, w := range wantLabels {
		if r.Steps[i].Label != w {
			t.Errorf("Steps[%d].Label = %q, want %q", i, r.Steps[i].Label, w)
		}
		if r.Steps[i].Status != mail.ProbeOK {
			t.Errorf("Steps[%d].Status = %v, want ProbeOK", i, r.Steps[i].Status)
		}
	}
}
```

- [ ] **Step 3.3: Run, expect compile failure**

```bash
go test ./internal/mailjmap/ -run TestProbeFastmailJMAP -v
```

- [ ] **Step 3.4: Implement**

`internal/mailjmap/probe.go`:

```go
package mailjmap

import (
	"context"
	"fmt"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// Probe runs a connect-test against a JMAP account, recording each
// step so the wizard probe screen can render progress live. Mirrors
// Backend.Connect's discovery + auth + Session/get + Account/get flow
// without leaving an open backend.
func Probe(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
	r := mail.ProbeResult{}
	addStep := func(label string) func(err error, detail string) {
		idx := len(r.Steps)
		r.Steps = append(r.Steps, mail.ProbeStep{Label: label, Status: mail.ProbePending})
		return func(err error, detail string) {
			if err != nil {
				r.Steps[idx].Status = mail.ProbeFail
				r.Steps[idx].Detail = err.Error()
				if r.Err == nil {
					r.Err = fmt.Errorf("%s: %w", label, err)
				}
				return
			}
			r.Steps[idx].Status = mail.ProbeOK
			r.Steps[idx].Detail = detail
		}
	}

	// 1. Resolve session URL (well-known or user-supplied)
	finishURL := addStep("Resolving session URL")
	url, err := resolveSessionURL(cfg)
	finishURL(err, url)
	if err != nil {
		return r
	}

	// 2. TLS handshake
	finishTLS := addStep("TLS handshake")
	cli, err := newJMAPClient(ctx, cfg, url)
	finishTLS(err, "")
	if err != nil {
		return r
	}

	// 3. Bearer authentication (token already in cfg via PasswordCmd)
	finishAuth := addStep("Bearer authentication")
	err = cli.AuthenticateBearer(ctx)
	finishAuth(err, "")
	if err != nil {
		return r
	}

	// 4. Session/get
	finishSession := addStep("Session/get")
	session, err := cli.SessionGet(ctx)
	finishSession(err, "")
	if err != nil {
		return r
	}

	// 5. Account/get
	finishAccount := addStep("Account/get")
	mailboxes, err := cli.AccountGet(ctx, session.PrimaryAccount("mail"))
	detail := ""
	if err == nil {
		detail = fmt.Sprintf("%d mailboxes", len(mailboxes))
	}
	finishAccount(err, detail)
	return r
}
```

Names like `resolveSessionURL`, `newJMAPClient`, `cli.AuthenticateBearer`, `cli.SessionGet`, `cli.AccountGet` — adapt to whatever the existing `jmap.go` exposes. The structure (one step per RPC call) stays the same.

- [ ] **Step 3.5: Run, expect pass**

```bash
go test ./internal/mailjmap/ -run TestProbeFastmailJMAP -v
```

- [ ] **Step 3.6: Add JMAP failure-path test (auth failure)**

Pattern mirrors task 2.6.

- [ ] **Step 3.7: Run all jmap tests**

```bash
go test ./internal/mailjmap/ -v
```

- [ ] **Step 3.8: Commit**

```bash
git add internal/mailjmap/probe.go internal/mailjmap/probe_test.go
git commit -m "Pass 14 task 2b: mailjmap.Probe step-by-step transcript"
```

---

## Task 4: `Provider.CredentialStrategy` + `HelpURL`

**Files:**
- Modify: `internal/config/providers.go`
- Modify: `internal/config/providers_test.go`

- [ ] **Step 4.1: Write failing test for new fields**

Add to `internal/config/providers_test.go`:

```go
func TestProviderCredentialStrategies(t *testing.T) {
	cases := []struct {
		name     string
		want     CredentialStrategy
		helpHas  string
	}{
		{"fastmail", StrategyAPIToken, "fastmail.com"},
		{"gmail", StrategyOAuth, "google.com"},
		{"outlook", StrategyOAuth, "microsoft.com"},
		{"icloud", StrategyAppPassword, "appleid.apple.com"},
		{"yahoo", StrategyAppPassword, "login.yahoo.com"},
		{"zoho", StrategyAppPassword, "zoho.com"},
		{"mailbox-org", StrategyAppPassword, "mailbox.org"},
		{"protonmail", StrategyAppPassword, "proton.me"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ok := Providers[c.name]
			if !ok {
				t.Fatalf("provider %q missing from registry", c.name)
			}
			if p.CredentialStrategy != c.want {
				t.Errorf("CredentialStrategy = %v, want %v", p.CredentialStrategy, c.want)
			}
			if !strings.Contains(p.HelpURL, c.helpHas) {
				t.Errorf("HelpURL = %q, want substring %q", p.HelpURL, c.helpHas)
			}
		})
	}
}
```

- [ ] **Step 4.2: Run, expect compile failure**

```bash
go test ./internal/config/ -run TestProviderCredentialStrategies -v
```

- [ ] **Step 4.3: Add fields and wire them up**

In `internal/config/providers.go`, add at the top of the file (after the existing Provider struct):

```go
// CredentialStrategy is the per-provider credential surface the wizard
// renders. The ImapJ/Jmap distinction matters for self-hosted: IMAP
// asks host+port, JMAP asks session URL.
type CredentialStrategy int

const (
	StrategyUnknown CredentialStrategy = iota
	StrategyAppPassword
	StrategyAPIToken
	StrategyOAuth
	StrategyPlainIMAP
	StrategyPlainJMAP
)
```

Add `CredentialStrategy` and `HelpURL` to the `Provider` struct:

```go
type Provider struct {
	Backend     string
	Host        string
	Port        int
	StartTLS    bool
	InsecureTLS bool
	GmailQuirks bool
	URL         string

	SMTPHost        string
	SMTPPort        int
	SMTPStartTLS    bool
	SMTPInsecureTLS bool

	CredentialStrategy CredentialStrategy
	HelpURL            string // page where user generates credentials
}
```

Then update each preset to set `CredentialStrategy` and `HelpURL`:

```go
"fastmail": {
	Backend: "jmap",
	URL:     "https://api.fastmail.com/jmap/session",
	SMTPHost: "smtp.fastmail.com", SMTPPort: 465,
	CredentialStrategy: StrategyAPIToken,
	HelpURL:            "https://app.fastmail.com/settings/security/tokens",
},
"gmail": {
	Backend: "imap",
	Host: "imap.gmail.com", Port: 993,
	GmailQuirks: true,
	SMTPHost: "smtp.gmail.com", SMTPPort: 465,
	CredentialStrategy: StrategyOAuth,
	HelpURL:            "https://accounts.google.com/o/oauth2/v2/auth",
},
"outlook": {
	Backend: "imap",
	Host: "outlook.office365.com", Port: 993,
	SMTPHost: "smtp.office365.com", SMTPPort: 587, SMTPStartTLS: true,
	CredentialStrategy: StrategyOAuth,
	HelpURL:            "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
},
"yahoo": {
	Backend: "imap",
	Host: "imap.mail.yahoo.com", Port: 993,
	SMTPHost: "smtp.mail.yahoo.com", SMTPPort: 465,
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://login.yahoo.com/account/security",
},
"icloud": {
	Backend: "imap",
	Host: "imap.mail.me.com", Port: 993,
	SMTPHost: "smtp.mail.me.com", SMTPPort: 587, SMTPStartTLS: true,
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://appleid.apple.com/account/manage",
},
"zoho": {
	Backend: "imap",
	Host: "imap.zoho.com", Port: 993,
	SMTPHost: "smtp.zoho.com", SMTPPort: 465,
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://accounts.zoho.com/home#security/app_password",
},
"mailbox-org": {
	Backend: "imap",
	Host: "imap.mailbox.org", Port: 993,
	SMTPHost: "smtp.mailbox.org", SMTPPort: 465,
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://login.mailbox.org/security/app-passwords",
},
"posteo": {
	Backend: "imap",
	Host: "posteo.de", Port: 993,
	SMTPHost: "posteo.de", SMTPPort: 465,
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://posteo.de/en/help",
},
"runbox": {
	Backend: "imap",
	Host: "mail.runbox.com", Port: 993,
	// (existing SMTP fields here)
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://help.runbox.com/account-passwords/",
},
"gmx": {
	// existing fields
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://www.gmx.com/mail/security/",
},
"protonmail": {
	// existing fields (Bridge, InsecureTLS=true)
	CredentialStrategy: StrategyAppPassword,
	HelpURL:            "https://proton.me/mail/bridge",
},
"imap": {
	Backend:            "imap",
	CredentialStrategy: StrategyPlainIMAP,
},
"jmap": {
	Backend:            "jmap",
	CredentialStrategy: StrategyPlainJMAP,
},
```

(Update fields you didn't enumerate above to match — read the existing
file first so you don't drop any fields. The new `CredentialStrategy`
and `HelpURL` fields are additive.)

- [ ] **Step 4.4: Run, expect pass**

```bash
go test ./internal/config/ -run TestProviderCredentialStrategies -v
```

- [ ] **Step 4.5: Run full config tests to verify no regressions**

```bash
go test ./internal/config/ -v
```

- [ ] **Step 4.6: Commit**

```bash
git add internal/config/providers.go internal/config/providers_test.go
git commit -m "Pass 14 task 4: CredentialStrategy + HelpURL on Provider"
```

---

## Task 5: `config.ConfigError` + accounts.go #29 fix

**Files:**
- Create: `internal/config/errors.go`
- Modify: `internal/config/accounts.go` — drop empty-name check; default `name` to `email`
- Modify: `internal/config/accounts_test.go` — adjust expectations
- Create: `internal/config/errors_test.go`

- [ ] **Step 5.1: Write failing test for ConfigError**

`internal/config/errors_test.go`:

```go
package config

import (
	"errors"
	"strings"
	"testing"
)

func TestConfigErrorFormat(t *testing.T) {
	e := &ConfigError{
		Path:    "/home/user/.config/poplar/config.toml",
		Line:    42,
		Account: "fastmail",
		Field:   "email",
		Message: "field is required",
		Suggest: "add `email = \"you@yourdomain.com\"` under the [[account]] block",
	}
	s := e.Error()
	for _, want := range []string{
		"config.toml:42",
		"account \"fastmail\"",
		"field \"email\"",
		"field is required",
		"add `email",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Error() = %q, missing %q", s, want)
		}
	}
}

func TestConfigErrorIs(t *testing.T) {
	e := &ConfigError{Field: "email", Message: "required"}
	if !errors.Is(e, ErrConfigInvalid) {
		t.Errorf("errors.Is(e, ErrConfigInvalid) = false, want true")
	}
}
```

- [ ] **Step 5.2: Run, expect compile failure**

```bash
go test ./internal/config/ -run TestConfigError -v
```

- [ ] **Step 5.3: Implement**

`internal/config/errors.go`:

```go
package config

import (
	"errors"
	"fmt"
	"strings"
)

// ErrConfigInvalid is the sentinel for any structured config error.
// Callers test errors.Is(err, ErrConfigInvalid) to branch on the
// invalid-config family. Specific instances are *ConfigError.
var ErrConfigInvalid = errors.New("invalid config")

// ConfigError describes one validation failure with enough context to
// guide the user (or the wizard's --repair flag) to a fix.
type ConfigError struct {
	Path    string // resolved config-file path
	Line    int    // 1-based; 0 if unknown
	Account string // account name when applicable
	Field   string // offending field name
	Message string // short human description
	Suggest string // optional fix hint
}

func (e *ConfigError) Error() string {
	var b strings.Builder
	if e.Path != "" {
		fmt.Fprintf(&b, "%s", e.Path)
		if e.Line > 0 {
			fmt.Fprintf(&b, ":%d", e.Line)
		}
		b.WriteString(": ")
	}
	if e.Account != "" {
		fmt.Fprintf(&b, "account %q: ", e.Account)
	}
	if e.Field != "" {
		fmt.Fprintf(&b, "field %q: ", e.Field)
	}
	b.WriteString(e.Message)
	if e.Suggest != "" {
		b.WriteString("\n  fix: ")
		b.WriteString(e.Suggest)
	}
	return b.String()
}

// Is supports errors.Is(err, ErrConfigInvalid).
func (e *ConfigError) Is(target error) bool {
	return target == ErrConfigInvalid
}
```

- [ ] **Step 5.4: Apply #29 fix**

In `internal/config/accounts.go`, find the empty-name check around line 74. The current behavior is "return error if name empty". New behavior: default `Name` to `Email` if blank, only error if both are blank.

```go
// In the validator (after Email is parsed):
if e.Name == "" {
	if e.Email == "" {
		return nil, &ConfigError{
			Account: fmt.Sprintf("[%d]", idx),
			Field:   "name",
			Message: "name is required when email is also blank",
			Suggest: `set "name" or "email" on the [[account]] block`,
		}
	}
	e.Name = e.Email
}
```

(Locate by reading `internal/config/accounts.go` first; the exact
insertion point is wherever the current `name is required` check
lives.)

- [ ] **Step 5.5: Update existing accounts_test.go expectations**

The existing test that asserts an empty-name failure now must assert the default-to-email behavior. Update it:

```go
func TestAccountNameDefaultsToEmail(t *testing.T) {
	tomlSrc := `
[[account]]
provider = "fastmail"
email    = "you@example.com"
password = "x"
`
	got, err := ParseAccounts([]byte(tomlSrc))
	if err != nil {
		t.Fatalf("ParseAccounts: %v", err)
	}
	if got[0].Name != "you@example.com" {
		t.Errorf("Name = %q, want %q (defaulted from email)", got[0].Name, "you@example.com")
	}
}
```

If there's an existing `TestAccountEmptyName` (or similar) that asserts the OLD failure behavior, delete it — the case it covered is no longer an error.

- [ ] **Step 5.6: Run config tests**

```bash
go test ./internal/config/ -v
```

Expected: PASS for all, including the new ConfigError tests and the updated default-to-email test.

- [ ] **Step 5.7: Commit**

```bash
git add internal/config/errors.go internal/config/errors_test.go internal/config/accounts.go internal/config/accounts_test.go
git commit -m "Pass 14 task 5: ConfigError + name defaults to email (#29 fix)"
```

---

## Task 6: `internal/wizard/` skeleton + Strategy + Apply

**Files:**
- Create: `internal/wizard/model.go`
- Create: `internal/wizard/section.go`
- Create: `internal/wizard/strategy.go`
- Create: `internal/wizard/strategy_test.go`
- Create: `internal/wizard/apply.go`
- Create: `internal/wizard/apply_test.go`

This is the UI-free domain layer. No bubbletea imports.

- [ ] **Step 6.1: Write failing test for Strategy selection**

`internal/wizard/strategy_test.go`:

```go
package wizard

import (
	"testing"

	"github.com/glw907/poplar/internal/config"
)

func TestSelectStrategy(t *testing.T) {
	cases := []struct {
		preset string
		want   config.CredentialStrategy
	}{
		{"fastmail", config.StrategyAPIToken},
		{"gmail", config.StrategyOAuth},
		{"yahoo", config.StrategyAppPassword},
		{"imap", config.StrategyPlainIMAP},
		{"jmap", config.StrategyPlainJMAP},
	}
	for _, c := range cases {
		s, err := SelectStrategy(c.preset)
		if err != nil {
			t.Fatalf("SelectStrategy(%q): %v", c.preset, err)
		}
		if s.Kind() != c.want {
			t.Errorf("SelectStrategy(%q).Kind() = %v, want %v", c.preset, s.Kind(), c.want)
		}
	}
}

func TestSelectStrategyUnknown(t *testing.T) {
	_, err := SelectStrategy("totally-not-a-provider")
	if err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}
```

- [ ] **Step 6.2: Write failing test for Apply**

`internal/wizard/apply_test.go`:

```go
package wizard

import (
	"testing"

	"github.com/glw907/poplar/internal/config"
)

func TestApplyFastmailJMAP(t *testing.T) {
	m := Model{
		Provider:     "fastmail",
		Email:        "user@fastmail.com",
		Token:        "fmip-secret-token",
		IdentityName: "User",
		AccountLabel: "Fastmail",
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.Provider != "fastmail" {
		t.Errorf("Provider = %q, want fastmail", cfg.Provider)
	}
	if cfg.Email != "user@fastmail.com" {
		t.Errorf("Email = %q", cfg.Email)
	}
	if cfg.Name != "Fastmail" {
		t.Errorf("Name = %q, want Fastmail", cfg.Name)
	}
	if cfg.Password != "fmip-secret-token" {
		t.Errorf("Password = %q (api token routes through Password field)", cfg.Password)
	}
}

func TestApplyPlainIMAP(t *testing.T) {
	m := Model{
		Provider:    "imap",
		Email:       "user@example.com",
		Host:        "mail.example.com",
		Port:        993,
		Username:    "user@example.com",
		Password:    "p",
		InsecureTLS: false,
	}
	cfg, err := Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if cfg.Host != "mail.example.com" || cfg.Port != 993 {
		t.Errorf("host/port = %s:%d", cfg.Host, cfg.Port)
	}
}
```

- [ ] **Step 6.3: Run, expect compile failure**

```bash
go test ./internal/wizard/... -v
```

- [ ] **Step 6.4: Implement model + section + strategy + apply**

`internal/wizard/model.go`:

```go
package wizard

import "github.com/glw907/poplar/internal/mail"

// Step is the wizard's current position. Bumped by Update on advance.
type Step int

const (
	StepWelcome Step = iota
	StepProvider
	StepEmail
	StepCredentials
	StepProbe
	StepIdentity
	StepLabel
	StepTheme
	StepConfirm
	StepDone
)

// Model is the wizard's domain state. UI state (focused field, scroll
// position) lives in the bubbletea sub-models; this is the data the
// wizard ultimately writes.
type Model struct {
	Step Step

	// Provider section
	Provider string // preset key, e.g. "fastmail" or "imap"

	// Identity section
	Email        string
	IdentityName string
	AccountLabel string

	// Credentials, depending on strategy. Fields are populated only by
	// the strategy that's active; others stay zero-valued.
	Password    string // app-password strategy
	Token       string // api-token strategy (Fastmail) — held separately for clarity, written into AccountConfig.Password by Apply
	Host        string // plain-IMAP
	Port        int    // plain-IMAP
	Username    string // plain-IMAP / plain-JMAP
	InsecureTLS bool   // plain-IMAP / plain-JMAP
	SessionURL  string // plain-JMAP

	// Theme section
	Theme string // empty == use default

	// Probe outcome (set by ProbeStep)
	Probe mail.ProbeResult
}
```

`internal/wizard/section.go`:

```go
package wizard

// SectionOpts describes how a Section should run.
type SectionOpts struct {
	FirstRun bool // true on first-run (auto-launched); false on poplar config init --interactive
	StartAt  string // optional — jump back into a specific step within this section (e.g. "credentials" after probe failure)
}

// Section is a self-contained group of wizard steps that produces some
// configuration delta. The wizard composes ordered Sections at
// runtime; new configurable surfaces (contacts, signatures, tidy)
// land as new Sections without restructuring the orchestrator.
type Section interface {
	Name() string         // stable identifier, used by --section= and --repair=
	Required() bool       // first-run blocks until required sections succeed
	Hide() bool           // skip silently when the underlying feature isn't ready
}
```

`internal/wizard/strategy.go`:

```go
package wizard

import (
	"fmt"

	"github.com/glw907/poplar/internal/config"
)

// Strategy is the credential surface for one provider preset. The UI
// layer dispatches on Kind to render the right huh group; the domain
// layer never imports bubbletea.
type Strategy interface {
	Kind() config.CredentialStrategy
}

type appPasswordStrategy struct{}

func (appPasswordStrategy) Kind() config.CredentialStrategy { return config.StrategyAppPassword }

type apiTokenStrategy struct{}

func (apiTokenStrategy) Kind() config.CredentialStrategy { return config.StrategyAPIToken }

type oauthStrategy struct{}

func (oauthStrategy) Kind() config.CredentialStrategy { return config.StrategyOAuth }

type plainIMAPStrategy struct{}

func (plainIMAPStrategy) Kind() config.CredentialStrategy { return config.StrategyPlainIMAP }

type plainJMAPStrategy struct{}

func (plainJMAPStrategy) Kind() config.CredentialStrategy { return config.StrategyPlainJMAP }

// SelectStrategy looks up the preset and returns its strategy.
func SelectStrategy(preset string) (Strategy, error) {
	p, ok := config.Providers[preset]
	if !ok {
		return nil, fmt.Errorf("unknown provider preset %q", preset)
	}
	switch p.CredentialStrategy {
	case config.StrategyAppPassword:
		return appPasswordStrategy{}, nil
	case config.StrategyAPIToken:
		return apiTokenStrategy{}, nil
	case config.StrategyOAuth:
		return oauthStrategy{}, nil
	case config.StrategyPlainIMAP:
		return plainIMAPStrategy{}, nil
	case config.StrategyPlainJMAP:
		return plainJMAPStrategy{}, nil
	}
	return nil, fmt.Errorf("provider %q has no credential strategy", preset)
}
```

`internal/wizard/apply.go`:

```go
package wizard

import (
	"fmt"

	"github.com/glw907/poplar/internal/config"
)

// Apply converts wizard state into a config.AccountConfig ready for
// validation. It does not write to disk; the orchestrator does that
// after Confirm.
func Apply(m Model) (config.AccountConfig, error) {
	preset, ok := config.Providers[m.Provider]
	if !ok {
		return config.AccountConfig{}, fmt.Errorf("unknown provider %q", m.Provider)
	}
	cfg := config.AccountConfig{
		Name:     m.AccountLabel,
		Provider: m.Provider,
		Email:    m.Email,
	}
	if cfg.Name == "" {
		cfg.Name = m.Email
	}

	switch preset.CredentialStrategy {
	case config.StrategyAppPassword:
		cfg.Password = m.Password
	case config.StrategyAPIToken:
		cfg.Password = m.Token
	case config.StrategyOAuth:
		// Pass 14: write a placeholder password-cmd. Pass 14.1
		// replaces this with the real OAuth wiring.
		cfg.PasswordCmd = "# Pass 14.1 will configure OAuth here"
	case config.StrategyPlainIMAP:
		cfg.Host = m.Host
		cfg.Port = m.Port
		cfg.Username = m.Username
		cfg.Password = m.Password
		cfg.InsecureTLS = m.InsecureTLS
	case config.StrategyPlainJMAP:
		cfg.URL = m.SessionURL
		cfg.Username = m.Username
		cfg.Password = m.Token // bearer or password — same field, server distinguishes
		cfg.InsecureTLS = m.InsecureTLS
	default:
		return config.AccountConfig{}, fmt.Errorf("provider %q has no credential strategy", m.Provider)
	}

	// Identity: at least one identity is always populated (per ADR-0177).
	if m.IdentityName != "" {
		cfg.Identities = []config.Identity{
			{Name: m.IdentityName, Email: m.Email},
		}
	}

	return cfg, nil
}
```

(If `config.AccountConfig` field names differ — `URL` vs `SessionURL` vs something else — adjust the Apply code accordingly. Read `internal/config/accounts.go` first.)

- [ ] **Step 6.5: Run wizard tests, expect pass**

```bash
go test ./internal/wizard/... -v
```

- [ ] **Step 6.6: Commit**

```bash
git add internal/wizard/
git commit -m "Pass 14 task 6: wizard domain types + Strategy + Apply"
```

---

## Task 7: `wizard.Probe` dispatcher

**Files:**
- Create: `internal/wizard/probe.go`
- Create: `internal/wizard/probe_test.go`

- [ ] **Step 7.1: Write failing test**

`internal/wizard/probe_test.go`:

```go
package wizard

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// fake probes — set at test time
var (
	fakeIMAPProbe = func(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
		return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "imap-fake", Status: mail.ProbeOK}}}
	}
	fakeJMAPProbe = func(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
		return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "jmap-fake", Status: mail.ProbeOK}}}
	}
)

func TestProbeRoutesIMAP(t *testing.T) {
	prevImap, prevJmap := imapProbeFn, jmapProbeFn
	defer func() { imapProbeFn, jmapProbeFn = prevImap, prevJmap }()
	imapProbeFn, jmapProbeFn = fakeIMAPProbe, fakeJMAPProbe

	cfg := config.AccountConfig{Provider: "yahoo"}
	r := Probe(context.Background(), cfg)
	if len(r.Steps) != 1 || r.Steps[0].Label != "imap-fake" {
		t.Fatalf("expected imap-fake, got %+v", r.Steps)
	}
}

func TestProbeRoutesJMAP(t *testing.T) {
	prevImap, prevJmap := imapProbeFn, jmapProbeFn
	defer func() { imapProbeFn, jmapProbeFn = prevImap, prevJmap }()
	imapProbeFn, jmapProbeFn = fakeIMAPProbe, fakeJMAPProbe

	cfg := config.AccountConfig{Provider: "fastmail"}
	r := Probe(context.Background(), cfg)
	if len(r.Steps) != 1 || r.Steps[0].Label != "jmap-fake" {
		t.Fatalf("expected jmap-fake, got %+v", r.Steps)
	}
}
```

- [ ] **Step 7.2: Run, expect compile failure**

```bash
go test ./internal/wizard/... -run TestProbe -v
```

- [ ] **Step 7.3: Implement**

`internal/wizard/probe.go`:

```go
package wizard

import (
	"context"
	"fmt"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailimap"
	"github.com/glw907/poplar/internal/mailjmap"
)

// Indirection points so tests can substitute fakes without spinning
// up real backends.
var (
	imapProbeFn = mailimap.Probe
	jmapProbeFn = mailjmap.Probe
)

// Probe routes to the right backend based on cfg.Provider's preset and
// returns a step-by-step transcript. SMTP probing is appended for IMAP
// accounts only; JMAP submission rides the JMAP session.
func Probe(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
	preset, ok := config.Providers[cfg.Provider]
	if !ok {
		return mail.ProbeResult{
			Err: fmt.Errorf("unknown provider %q", cfg.Provider),
		}
	}
	switch preset.Backend {
	case "imap":
		r := imapProbeFn(ctx, cfg)
		if !r.OK() {
			return r
		}
		// Append SMTP probe step
		smtpErr := mailimap.ProbeSMTP(cfg)
		step := mail.ProbeStep{Label: "SMTP submission", Status: mail.ProbeOK}
		if smtpErr != nil {
			step.Status = mail.ProbeFail
			step.Detail = smtpErr.Error()
			r.Err = fmt.Errorf("SMTP: %w", smtpErr)
		}
		r.Steps = append(r.Steps, step)
		return r
	case "jmap":
		return jmapProbeFn(ctx, cfg)
	}
	return mail.ProbeResult{
		Err: fmt.Errorf("provider %q has unknown backend %q", cfg.Provider, preset.Backend),
	}
}
```

- [ ] **Step 7.4: Run wizard tests**

```bash
go test ./internal/wizard/... -v
```

- [ ] **Step 7.5: Commit**

```bash
git add internal/wizard/probe.go internal/wizard/probe_test.go
git commit -m "Pass 14 task 7: wizard.Probe dispatcher (IMAP+SMTP, JMAP)"
```

---

## Task 8: Config writer — `Render`

**Files:**
- Modify: `internal/config/writer.go`
- Modify: `internal/config/writer_test.go`

- [ ] **Step 8.1: Read existing writer**

```bash
sed -n '1,80p' internal/config/writer.go
```

Note what it currently exports. We're adding `Render`.

- [ ] **Step 8.2: Write failing test**

Add to `internal/config/writer_test.go`:

```go
func TestRenderRoundTrip(t *testing.T) {
	src := []byte(`
[[account]]
name         = "Fastmail"
provider     = "fastmail"
email        = "you@example.com"
password-cmd = "echo secret"

[ui]
theme = "gruvbox"
`)
	accts, err := ParseAccounts(src)
	if err != nil {
		t.Fatalf("ParseAccounts: %v", err)
	}
	ui, err := ParseUI(src)
	if err != nil {
		t.Fatalf("ParseUI: %v", err)
	}

	out := Render(accts, ui, CacheConfig{})
	got, _ := ParseAccounts(out)
	if got[0].Name != "Fastmail" || got[0].Email != "you@example.com" {
		t.Errorf("round-trip lost fields: %+v", got[0])
	}
	gotUI, _ := ParseUI(out)
	if gotUI.Theme != "gruvbox" {
		t.Errorf("UI.Theme = %q after round-trip", gotUI.Theme)
	}
}

func TestRenderOmitsDefaultTheme(t *testing.T) {
	out := Render(
		[]AccountConfig{{Name: "fm", Provider: "fastmail", Email: "u@x", Password: "p"}},
		UIConfig{Theme: ""}, // empty == default
		CacheConfig{},
	)
	if strings.Contains(string(out), "[ui]") || strings.Contains(string(out), "theme") {
		t.Errorf("Render emitted [ui] for default theme:\n%s", out)
	}
}
```

(Use whatever the existing parse functions are named — the spec uses `LoadUI` for file-path-based loads, but for in-memory parsing the function may be named differently. Adjust to what `internal/config/` already exposes.)

- [ ] **Step 8.3: Run, expect compile failure**

```bash
go test ./internal/config/ -run TestRender -v
```

- [ ] **Step 8.4: Implement Render**

In `internal/config/writer.go`, add:

```go
// Render emits canonical TOML for the given accounts + UI + cache
// config. Idempotent: rendering a config that was just loaded yields
// byte-identical output. Default values are omitted to keep wizard-
// generated files minimal.
func Render(accts []AccountConfig, ui UIConfig, cache CacheConfig) []byte {
	var b strings.Builder

	for _, a := range accts {
		b.WriteString("[[account]]\n")
		fmt.Fprintf(&b, "name         = %q\n", a.Name)
		fmt.Fprintf(&b, "provider     = %q\n", a.Provider)
		fmt.Fprintf(&b, "email        = %q\n", a.Email)
		switch {
		case a.PasswordCmd != "":
			fmt.Fprintf(&b, "password-cmd = %q\n", a.PasswordCmd)
		case a.Password != "":
			fmt.Fprintf(&b, "password     = %q\n", a.Password)
		}
		if a.Host != "" {
			fmt.Fprintf(&b, "host         = %q\n", a.Host)
		}
		if a.Port != 0 && a.Port != 993 {
			fmt.Fprintf(&b, "port         = %d\n", a.Port)
		}
		if a.URL != "" {
			fmt.Fprintf(&b, "url          = %q\n", a.URL)
		}
		if a.Username != "" && a.Username != a.Email {
			fmt.Fprintf(&b, "username     = %q\n", a.Username)
		}
		if a.InsecureTLS {
			b.WriteString("insecure-tls = true\n")
		}
		b.WriteString("\n")
	}

	if ui.Theme != "" && ui.Theme != "one-dark" {
		b.WriteString("[ui]\n")
		fmt.Fprintf(&b, "theme = %q\n", ui.Theme)
	}

	// CacheConfig: only emit if non-default
	if cache.MaxSize != "" {
		b.WriteString("\n[cache]\n")
		fmt.Fprintf(&b, "max-size = %q\n", cache.MaxSize)
	}

	return []byte(b.String())
}
```

(Field names — `AccountConfig.Username`, `AccountConfig.URL`, `CacheConfig.MaxSize` — must match what the existing config types use. Read `accounts.go` and `cache.go` first; adjust.)

- [ ] **Step 8.5: Run config tests**

```bash
go test ./internal/config/ -v
```

- [ ] **Step 8.6: Commit**

```bash
git add internal/config/writer.go internal/config/writer_test.go
git commit -m "Pass 14 task 8: config.Render canonical TOML emission"
```

---

## Task 9: Template rewrite (#29 fix part 2)

**Files:**
- Modify: `internal/config/template.go`
- Modify: `internal/config/template.golden`
- Modify: `internal/config/template_test.go` (regenerate golden if needed)

- [ ] **Step 9.1: Update template.go**

Find the comment block containing "Until poplar's first-run wizard ships..." (currently around `template.go:84-95` per the file we read). Replace with:

```
# OAuth providers
#
#   gmail and outlook authenticate with a short-lived access
#   token (XOAUTH2). poplar's wizard runs the OAuth consent flow
#   when you pick these providers — no password-cmd setup needed.
#   To add or re-authenticate an account, run:
#
#       poplar config init --interactive --section=account
```

Also: drop the `name = "Your Name"` line from the example `[[account]]` block (since `name` now defaults to email).

Replace the example block:

```
[[account]]
provider     = "fastmail"
email        = "you@yourdomain.com"
password-cmd = "op read op://Personal/Fastmail/credential"
```

(The current template already lacks `name`, so this part may already be done; verify by reading template.go before editing.)

- [ ] **Step 9.2: Regenerate golden file**

```bash
go test ./internal/config/ -run TestTemplate -v -update
```

(If `-update` flag isn't supported, manually copy the new template body to `template.golden`.)

- [ ] **Step 9.3: Verify all template tests pass**

```bash
go test ./internal/config/ -v
```

- [ ] **Step 9.4: Commit**

```bash
git add internal/config/template.go internal/config/template.golden
git commit -m "Pass 14 task 9: template rewrite — OAuth + wizard messaging"
```

---

## Task 10: `internal/ui/wizard/` skeleton + theme adapter + logo

**Files:**
- Create: `internal/ui/wizard/styles.go`
- Create: `internal/ui/wizard/theme_adapter.go`
- Create: `internal/ui/wizard/logo.go`
- Create: `internal/ui/wizard/msgs.go`
- Create: `internal/ui/wizard/model.go`
- Create: `internal/ui/wizard/model_test.go`
- Create: `art/poplar-logo.ans` (cbonsai output, generated below)
- Create: `art/regen-logo.sh` (reproducer)
- Create: `art/CREDITS.md`

Read `docs/poplar/bubbletea-conventions.md` and invoke `elm-conventions` skill before this task.

- [ ] **Step 10.1: Generate cbonsai artifact**

```bash
sudo apt install -y cbonsai tmux
mkdir -p art
cat > art/regen-logo.sh <<'EOF'
#!/bin/sh
# Regenerate art/poplar-logo.ans — committed for future logo swap.
# Pass 14 ships a wordmark; whoever picks up the proper logo can
# either use this baked tree or replace it.
set -eu
SESSION="poplar-logo-regen-$$"
tmux new-session -d -s "$SESSION" -x 50 -y 26 "cbonsai -p -s 7 -L 32 -b 1; sleep 0.5"
sleep 0.4
tmux capture-pane -e -t "$SESSION" -p > art/poplar-logo.ans.raw
tmux kill-session -t "$SESSION"

# Strip trailing blank lines + the bottom 4 pot lines.
python3 - <<'PY'
import re
data = open('art/poplar-logo.ans.raw').read()
lines = data.split('\n')
def vis(l): return re.sub(r'\x1b\[[0-9;?]*[A-Za-z]', '', l).strip()
while lines and not vis(lines[-1]): lines.pop()
removed = 0
while lines and removed < 4:
    if vis(lines[-1]):
        lines.pop(); removed += 1
    else:
        lines.pop()
while lines and not vis(lines[-1]): lines.pop()
while lines and not vis(lines[0]): lines.pop(0)
open('art/poplar-logo.ans', 'w').write('\n'.join(lines) + '\n')
PY
rm art/poplar-logo.ans.raw
echo "wrote art/poplar-logo.ans"
EOF
chmod +x art/regen-logo.sh
./art/regen-logo.sh
ls -l art/poplar-logo.ans
```

Expected: `art/poplar-logo.ans` exists, contains ANSI-coloured tree.

- [ ] **Step 10.2: Write CREDITS.md**

```bash
cat > art/CREDITS.md <<'EOF'
# Art credits

## poplar-logo.ans

Generated with cbonsai by John Allbritten,
https://gitlab.com/jallbrit/cbonsai (GPLv3). The text artifact is
the program's output, not the program itself; cbonsai is a
build-time-only dependency for whoever chooses to regenerate the
logo. Reproduce with `art/regen-logo.sh` (requires cbonsai + tmux).

The active wizard logo is a typographic wordmark
(internal/ui/wizard/logo.go). The .ans artifact is committed for a
future pass to swap in if/when a chosen tree art lands.
EOF
```

- [ ] **Step 10.3: Implement Styles**

`internal/ui/wizard/styles.go`:

```go
package wizard

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/glw907/poplar/internal/theme"
)

// Styles is the wizard's projection of the compiled theme. Construct
// once at New and treat as read-only.
type Styles struct {
	Wordmark      lipgloss.Style
	Rule          lipgloss.Style
	Tagline       lipgloss.Style
	StepCounter   lipgloss.Style
	Frame         lipgloss.Style
	ProbeStepOK   lipgloss.Style
	ProbeStepFail lipgloss.Style
	ProbeStepPending lipgloss.Style
	Help          lipgloss.Style
}

func NewStyles(t *theme.CompiledTheme) Styles {
	return Styles{
		Wordmark:         lipgloss.NewStyle().Bold(true).Foreground(t.Accent),
		Rule:             lipgloss.NewStyle().Foreground(t.Border),
		Tagline:          lipgloss.NewStyle().Foreground(t.Subtle),
		StepCounter:      lipgloss.NewStyle().Foreground(t.Subtle),
		Frame:            lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border),
		ProbeStepOK:      lipgloss.NewStyle().Foreground(t.Success),
		ProbeStepFail:    lipgloss.NewStyle().Foreground(t.Danger),
		ProbeStepPending: lipgloss.NewStyle().Foreground(t.Subtle),
		Help:             lipgloss.NewStyle().Foreground(t.Subtle),
	}
}
```

(Field names on `theme.CompiledTheme` — `Accent`, `Border`, `Subtle`, `Success`, `Danger` — must match. Read `internal/theme/` first; adjust.)

- [ ] **Step 10.4: Implement theme adapter**

`internal/ui/wizard/theme_adapter.go`:

```go
package wizard

import (
	"charm.land/huh/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/glw907/poplar/internal/theme"
)

// HuhTheme builds a huh.Theme from poplar's compiled theme so the
// wizard's huh forms inherit the user's palette without maintaining
// a parallel theme system.
func HuhTheme(t *theme.CompiledTheme) *huh.Theme {
	base := huh.ThemeBase(true)
	base.Focused.Title = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	base.Focused.Description = lipgloss.NewStyle().Foreground(t.Subtle)
	base.Focused.Base = lipgloss.NewStyle().Foreground(t.Foreground)
	base.Focused.SelectSelector = lipgloss.NewStyle().Foreground(t.Accent)
	base.Focused.Option = lipgloss.NewStyle().Foreground(t.Foreground)
	base.Focused.SelectedOption = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	base.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(t.Accent)
	base.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(t.Subtle)
	base.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(t.Foreground)
	base.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(t.Danger)
	base.Help.ShortKey = lipgloss.NewStyle().Foreground(t.Subtle)
	base.Help.ShortDesc = lipgloss.NewStyle().Foreground(t.Subtle)
	return base
}
```

(Field names on `huh.Theme` — these match v2; if the API differs in the version you pulled in step 0.3, adjust.)

- [ ] **Step 10.5: Implement logo (wordmark interim)**

`internal/ui/wizard/logo.go`:

```go
package wizard

import (
	_ "embed"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogoART is the cbonsai artifact, committed for a future logo swap.
// Currently unused at runtime; renderLogo emits the wordmark.
//
//go:embed ../../../art/poplar-logo.ans
var LogoART string

// renderLogo returns the wizard's interim typographic wordmark.
// Replaceable: a future pass can switch this to render LogoART or
// any other artifact without touching the wizard flow.
func renderLogo(s Styles) string {
	rule := s.Rule.Render(strings.Repeat("─", 13))
	wordmark := s.Wordmark.Render("poplar")
	tagline := s.Tagline.Render("a terminal email client")
	return lipgloss.JoinVertical(lipgloss.Center, rule, wordmark, rule, tagline)
}
```

(Embed path: the `//go:embed` is relative to the source file. From `internal/ui/wizard/logo.go` to `art/poplar-logo.ans` is `../../../art/poplar-logo.ans`.)

- [ ] **Step 10.6: Implement msgs**

`internal/ui/wizard/msgs.go`:

```go
package wizard

import (
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/theme"
)

// AdvanceMsg pushes the wizard to its next Step.
type AdvanceMsg struct{}

// BackMsg pops the wizard to its previous Step (used by [e]dit on
// probe-failure to jump back to credentials).
type BackMsg struct{}

// ThemeChangedMsg propagates a live-preview theme swap from the theme
// section to the parent wizard model. The parent rebuilds its Styles
// and huh.Theme on receipt.
type ThemeChangedMsg struct {
	Theme *theme.CompiledTheme
	Name  string
}

// ProbeStartMsg kicks off the probe in a tea.Cmd.
type ProbeStartMsg struct{}

// ProbeStepMsg streams individual probe steps to the screen.
type ProbeStepMsg struct {
	Step mail.ProbeStep
}

// ProbeDoneMsg signals all probe steps have completed.
type ProbeDoneMsg struct {
	Result mail.ProbeResult
}

// CancelMsg is the user's confirmed cancel-this-wizard signal.
type CancelMsg struct{}
```

- [ ] **Step 10.7: Skeleton model**

`internal/ui/wizard/model.go`:

```go
package wizard

import (
	tea "github.com/charmbracelet/bubbletea"

	wizdomain "github.com/glw907/poplar/internal/wizard"
	"github.com/glw907/poplar/internal/theme"
)

// Model is the wizard parent. Owns the active Section and threads
// theme + style updates through it.
type Model struct {
	Domain  wizdomain.Model
	Theme   *theme.CompiledTheme
	Styles  Styles
	Width   int
	Height  int

	// Sections — populated in NewModel from the registry; one Section
	// is active at a time.
	sections []sectionAdapter
	active   int
}

// sectionAdapter is the UI-side wrapper around a domain Section.
// It holds the current bubbletea sub-model for the active step inside
// that section.
type sectionAdapter struct {
	domain wizdomain.Section
	view   tea.Model // current sub-model (huh.Form or custom)
}

// NewModel constructs the wizard with the section registry.
func NewModel(t *theme.CompiledTheme) Model {
	m := Model{
		Theme:  t,
		Styles: NewStyles(t),
	}
	m.sections = defaultSections(&m)
	return m
}

func (m Model) Init() tea.Cmd {
	if len(m.sections) == 0 {
		return nil
	}
	return m.sections[m.active].view.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		// Forward to active sub-model
	case ThemeChangedMsg:
		m.Theme = msg.Theme
		m.Styles = NewStyles(msg.Theme)
		// Re-style every sub-model (in practice, recreate them with
		// new theme; section reuse pattern lives in section_*.go).
		return m, nil
	case AdvanceMsg:
		if m.active+1 < len(m.sections) {
			m.active++
			return m, m.sections[m.active].view.Init()
		}
		// No more sections — write config and exit; orchestrated via
		// section_confirm.go in task 12.
	}

	// Forward to active sub-model
	if len(m.sections) > 0 {
		view, cmd := m.sections[m.active].view.Update(msg)
		m.sections[m.active].view = view
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.sections) == 0 {
		return ""
	}
	return m.sections[m.active].view.View()
}

// defaultSections is implemented in section_*.go files in tasks 11–12.
func defaultSections(m *Model) []sectionAdapter {
	return nil // populated by tasks 11–12
}
```

- [ ] **Step 10.8: Add basic Update/View test**

`internal/ui/wizard/model_test.go`:

```go
package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/theme"
)

func TestNewModelBuildsStyles(t *testing.T) {
	tc := theme.Themes["one-dark"]
	m := NewModel(tc)
	if m.Theme != tc {
		t.Fatalf("Theme not set")
	}
	// Init should not panic with empty section list (defaultSections
	// returns nil until task 11).
	_ = m.Init()
}

func TestModelHandlesWindowSize(t *testing.T) {
	m := NewModel(theme.Themes["one-dark"])
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	got := m2.(Model)
	if got.Width != 80 || got.Height != 24 {
		t.Errorf("size = %dx%d, want 80x24", got.Width, got.Height)
	}
}
```

- [ ] **Step 10.9: Run UI tests**

```bash
go test ./internal/ui/wizard/... -v
```

- [ ] **Step 10.10: Commit**

```bash
git add internal/ui/wizard/ art/
git commit -m "Pass 14 task 10: wizard UI skeleton, theme adapter, logo"
```

---

## Task 11: Account section

**Files:**
- Create: `internal/ui/wizard/section_account.go`
- Create: `internal/ui/wizard/section_account_test.go`

The account section is the wizard's biggest single piece. Build it as one cohesive sub-model that internally drives a state machine across provider → email → credentials → probe → identity → label.

- [ ] **Step 11.1: Read huh examples**

```bash
ls /tmp/huh-check/examples/ 2>/dev/null || (cd /tmp && git clone --depth 1 https://github.com/charmbracelet/huh huh-check)
ls /tmp/huh-check/examples/
```

Read the `bubbletea-app` and `simple` examples to confirm the embedding pattern (huh.Form is a tea.Model).

- [ ] **Step 11.2: Implement account section sub-model**

`internal/ui/wizard/section_account.go`:

```go
package wizard

import (
	"context"
	"fmt"

	"charm.land/huh/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/config"
	wizdomain "github.com/glw907/poplar/internal/wizard"
	"github.com/glw907/poplar/internal/uicore"
)

type accountStage int

const (
	stageProvider accountStage = iota
	stageEmail
	stageCredentials
	stageProbe
	stageIdentity
	stageLabel
)

// accountSection is the credentials-bearing section.
type accountSection struct {
	parent *Model
	stage  accountStage
	form   *huh.Form         // active form for the current stage
	probe  *probeScreen      // active probe sub-model when stage == stageProbe
	state  wizdomain.Model
}

func (s *accountSection) Name() string { return "account" }

// (Section interface impl mirrors the domain interface.)

func newAccountSection(parent *Model) *accountSection {
	s := &accountSection{parent: parent, stage: stageProvider}
	s.formForCurrentStage()
	return s
}

func (s *accountSection) formForCurrentStage() {
	switch s.stage {
	case stageProvider:
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose your mail provider").
				Options(providerOptions()...).
				Value(&s.state.Provider),
		)).WithTheme(HuhTheme(s.parent.Theme))

	case stageEmail:
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("What's your email address?").
				Validate(validateEmail).
				Value(&s.state.Email),
		)).WithTheme(HuhTheme(s.parent.Theme))

	case stageCredentials:
		strategy, _ := wizdomain.SelectStrategy(s.state.Provider)
		s.form = credentialsForm(strategy, &s.state, s.parent.Theme)

	case stageProbe:
		// Custom tea.Model — no huh form
		s.probe = newProbeScreen(s.state, s.parent.Styles)
		s.form = nil

	case stageIdentity:
		def := defaultIdentityName(s.state.Email)
		s.state.IdentityName = def
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Display name on outgoing mail?").
				Value(&s.state.IdentityName),
		)).WithTheme(HuhTheme(s.parent.Theme))

	case stageLabel:
		def := defaultAccountLabel(s.state.Provider, s.state.Email)
		s.state.AccountLabel = def
		s.form = huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("Sidebar label for this account?").
				Value(&s.state.AccountLabel),
		)).WithTheme(HuhTheme(s.parent.Theme))
	}
}

func (s *accountSection) Init() tea.Cmd {
	if s.form != nil {
		return s.form.Init()
	}
	if s.probe != nil {
		return s.probe.Init()
	}
	return nil
}

func (s *accountSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.probe != nil {
		probe, cmd := s.probe.Update(msg)
		s.probe = probe.(*probeScreen)
		if done, ok := msg.(ProbeDoneMsg); ok {
			s.state.Probe = done.Result
			if done.Result.OK() {
				s.advance()
				return s, s.Init()
			}
			// stay on probe screen; user picks [r], [e], [s]
		}
		return s, cmd
	}
	if s.form == nil {
		return s, nil
	}
	form, cmd := s.form.Update(msg)
	s.form = form.(*huh.Form)
	if s.form.State == huh.StateCompleted {
		s.advance()
		return s, s.Init()
	}
	return s, cmd
}

func (s *accountSection) advance() {
	switch s.stage {
	case stageProvider:
		s.stage = stageEmail
	case stageEmail:
		s.stage = stageCredentials
	case stageCredentials:
		s.stage = stageProbe
	case stageProbe:
		s.stage = stageIdentity
	case stageIdentity:
		s.stage = stageLabel
	case stageLabel:
		// Section done — emit AdvanceMsg via Cmd
	}
	s.formForCurrentStage()
}

func (s *accountSection) View() string {
	if s.probe != nil {
		return s.probe.View()
	}
	if s.form == nil {
		return ""
	}
	return s.form.View()
}

// providerOptions returns the huh.Options list. Order matches the
// wireframe in the spec.
func providerOptions() []huh.Option[string] {
	return []huh.Option[string]{
		huh.NewOption("Fastmail              JMAP, paste an API token", "fastmail"),
		huh.NewOption("Gmail                 OAuth (browser flow)", "gmail"),
		huh.NewOption("Outlook / Microsoft   OAuth (browser flow)", "outlook"),
		huh.NewOption("iCloud                IMAP, app password", "icloud"),
		huh.NewOption("Yahoo                 IMAP, app password", "yahoo"),
		huh.NewOption("Zoho                  IMAP, app password", "zoho"),
		huh.NewOption("Mailbox.org           IMAP, app password", "mailbox-org"),
		huh.NewOption("Posteo                IMAP, app password", "posteo"),
		huh.NewOption("Runbox                IMAP, app password", "runbox"),
		huh.NewOption("GMX                   IMAP, app password", "gmx"),
		huh.NewOption("ProtonMail (Bridge)   local IMAP, Bridge required", "protonmail"),
		huh.NewOption("Other / self-hosted IMAP   manual IMAP host + port", "imap"),
		huh.NewOption("Other / self-hosted JMAP   manual JMAP session URL", "jmap"),
	}
}

func validateEmail(s string) error {
	if !looksLikeEmail(s) {
		return fmt.Errorf("looks like a malformed email")
	}
	return nil
}

func looksLikeEmail(s string) bool {
	at := -1
	for i, r := range s {
		if r == '@' {
			at = i
			break
		}
	}
	return at > 0 && at < len(s)-1
}

func defaultIdentityName(email string) string {
	for i, r := range email {
		if r == '@' {
			local := email[:i]
			if local == "" { return "" }
			// Title-case the local part
			return uicore.TitleCase(local)
		}
	}
	return ""
}

func defaultAccountLabel(provider, email string) string {
	if p, ok := config.Providers[provider]; ok && p.HelpURL != "" {
		// preset name itself works as a label, e.g. "Fastmail"
		return uicore.TitleCase(provider)
	}
	for i, r := range email {
		if r == '@' {
			return email[i+1:]
		}
	}
	return email
}

// credentialsForm dispatches on Strategy.Kind and returns the right
// huh.Form for the credential surface.
func credentialsForm(strategy wizdomain.Strategy, state *wizdomain.Model, t *theme.CompiledTheme) *huh.Form {
	switch strategy.Kind() {
	case config.StrategyAppPassword:
		preset := config.Providers[state.Provider]
		return huh.NewForm(huh.NewGroup(
			huh.NewNote().
				Title("App password required").
				Description(fmt.Sprintf("Generate one at:\n  %s", preset.HelpURL)),
			huh.NewInput().
				Title("App password").
				EchoMode(huh.EchoModePassword).
				Value(&state.Password),
		)).WithTheme(HuhTheme(t))

	case config.StrategyAPIToken:
		preset := config.Providers[state.Provider]
		return huh.NewForm(huh.NewGroup(
			huh.NewNote().
				Title("API token").
				Description(fmt.Sprintf("Generate one at:\n  %s\n\nTokens don't expire unless revoked.", preset.HelpURL)),
			huh.NewInput().
				Title("API token").
				EchoMode(huh.EchoModePassword).
				Value(&state.Token),
		)).WithTheme(HuhTheme(t))

	case config.StrategyOAuth:
		// Pass 14: placeholder password-cmd. Pass 14.1 replaces this.
		return huh.NewForm(huh.NewGroup(
			huh.NewNote().
				Title("OAuth (placeholder)").
				Description("Pass 14.1 will configure OAuth interactively.\nFor now, set a password-cmd that returns a fresh access token."),
			huh.NewInput().
				Title("password-cmd").
				Value(&state.Password),
		)).WithTheme(HuhTheme(t))

	case config.StrategyPlainIMAP:
		state.Port = 993
		state.Username = state.Email
		return huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Host").Value(&state.Host),
			huh.NewInput().Title("Port").Validate(validatePort).Value(portRef(state)),
			huh.NewInput().Title("Username").Value(&state.Username),
			huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).Value(&state.Password),
			huh.NewConfirm().
				Title("Skip TLS verify? (only for self-signed certs)").
				Value(&state.InsecureTLS).
				WithHideFunc(func() bool { return !looksLocalHost(state.Host) }),
		)).WithTheme(HuhTheme(t))

	case config.StrategyPlainJMAP:
		state.Username = state.Email
		return huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Session URL").Value(&state.SessionURL),
			huh.NewInput().Title("Username").Value(&state.Username),
			huh.NewInput().Title("Token").EchoMode(huh.EchoModePassword).Value(&state.Token),
			huh.NewConfirm().
				Title("Skip TLS verify? (only for self-signed certs)").
				Value(&state.InsecureTLS).
				WithHideFunc(func() bool { return !looksLocalURL(state.SessionURL) }),
		)).WithTheme(HuhTheme(t))
	}
	return nil
}

func validatePort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil { return fmt.Errorf("port must be a number") }
	if n < 1 || n > 65535 { return fmt.Errorf("port out of range") }
	return nil
}

func portRef(state *wizdomain.Model) *string {
	// huh.Input wants *string; bridge to *int via a small adapter.
	// (Implementation: keep Port as int on Model, marshal at form-build
	// time, parse at form-completed.)
	// For brevity, the executing engineer can choose to keep Port as
	// string on wizdomain.Model and parse in Apply, or implement the
	// adapter here. The simpler path is to use a string field for the
	// form and convert in Apply — recommend that.
	return nil
}

func looksLocalHost(host string) bool {
	// RFC1918, .local, 127.x — same heuristic the spec describes
	return strings.HasSuffix(host, ".local") ||
		strings.HasPrefix(host, "127.") ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		(strings.HasPrefix(host, "172.") && between(host, 16, 31))
}

func looksLocalURL(url string) bool {
	// strip scheme, take host, run looksLocalHost
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(url, prefix) {
			rest := url[len(prefix):]
			if i := strings.IndexAny(rest, "/:"); i >= 0 {
				rest = rest[:i]
			}
			return looksLocalHost(rest)
		}
	}
	return false
}

func between(host string, lo, hi int) bool {
	parts := strings.Split(host, ".")
	if len(parts) < 2 { return false }
	n, err := strconv.Atoi(parts[1])
	if err != nil { return false }
	return n >= lo && n <= hi
}
```

(Recommendation noted in the code: convert `Port` to `string` on `wizdomain.Model` to avoid the int<->string bridging dance with huh. Update task 6 if you prefer that — same module, same commit cycle is fine since this all goes together.)

If you take that recommendation, change `Model.Port` from `int` to `string` and adjust `Apply` to parse it. The Apply test from task 6 needs an updated literal.

- [ ] **Step 11.3: Implement probe screen**

`internal/ui/wizard/probe_screen.go`:

```go
package wizard

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	wizdomain "github.com/glw907/poplar/internal/wizard"
	"github.com/glw907/poplar/internal/uicore"
)

type probeScreen struct {
	cfg       config.AccountConfig
	steps     []mail.ProbeStep
	result    mail.ProbeResult
	spinner   uicore.Spinner
	done      bool
	failure   bool
	styles    Styles
}

func newProbeScreen(state wizdomain.Model, styles Styles) *probeScreen {
	cfg, _ := wizdomain.Apply(state)
	return &probeScreen{
		cfg:     cfg,
		spinner: uicore.NewSpinner(),
		styles:  styles,
	}
}

func (p *probeScreen) Init() tea.Cmd {
	return tea.Batch(p.spinner.Tick, p.runProbe)
}

// runProbe executes wizard.Probe in a goroutine and emits ProbeDoneMsg.
func (p *probeScreen) runProbe() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := wizdomain.Probe(ctx, p.cfg)
	return ProbeDoneMsg{Result: result}
}

func (p *probeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ProbeDoneMsg:
		p.result = msg.Result
		p.steps = msg.Result.Steps
		p.done = true
		p.failure = !msg.Result.OK()
		if !p.failure {
			// auto-advance after 800ms
			return p, tea.Tick(800*time.Millisecond, func(time.Time) tea.Msg { return AdvanceMsg{} })
		}
	case tea.KeyMsg:
		if p.done && p.failure {
			switch msg.String() {
			case "r": // retry
				p.done, p.failure = false, false
				p.steps = nil
				return p, p.runProbe
			case "e": // edit credentials — emit BackMsg, parent jumps section back
				return p, func() tea.Msg { return BackMsg{} }
			case "s": // save and quit
				return p, func() tea.Msg { return CancelMsg{SaveCommented: true} }
			}
		}
	}
	var cmd tea.Cmd
	p.spinner, cmd = p.spinner.Update(msg)
	return p, cmd
}

func (p *probeScreen) View() string {
	var b strings.Builder
	if p.cfg.Backend == "jmap" {
		b.WriteString("Testing JMAP connection\n\n")
	} else {
		b.WriteString(fmt.Sprintf("Testing connection to %s:%d\n\n", p.cfg.Host, p.cfg.Port))
	}
	for _, step := range p.steps {
		mark := " "
		var marker string
		switch step.Status {
		case mail.ProbeOK:
			marker = p.styles.ProbeStepOK.Render("✓")
		case mail.ProbeFail:
			marker = p.styles.ProbeStepFail.Render("✗")
		default:
			marker = p.styles.ProbeStepPending.Render(p.spinner.View())
		}
		_ = mark
		fmt.Fprintf(&b, "  %-40s %s", step.Label, marker)
		if step.Detail != "" {
			fmt.Fprintf(&b, " %s", step.Detail)
		}
		b.WriteString("\n")
	}
	if p.done {
		if p.failure {
			b.WriteString("\n  ")
			b.WriteString(p.styles.Help.Render("[r] retry   [e] edit credentials   [s] save and quit"))
		} else {
			b.WriteString("\n  Connected. Continuing…\n")
		}
	}
	return b.String()
}
```

Update `msgs.go` to extend `CancelMsg`:

```go
type CancelMsg struct {
	SaveCommented bool // [s] save and quit on probe failure
}
```

(Add `Backend` field to `config.AccountConfig` if it isn't already — or read `Provider`'s preset to determine "imap" vs "jmap" instead. Read `accounts.go` first to confirm.)

- [ ] **Step 11.4: Run UI wizard tests**

```bash
go test ./internal/ui/wizard/... -v
```

- [ ] **Step 11.5: Commit**

```bash
git add internal/ui/wizard/section_account.go internal/ui/wizard/probe_screen.go internal/ui/wizard/msgs.go
git commit -m "Pass 14 task 11: account section + probe screen"
```

---

## Task 12: Theme section + section registry + orchestrator

**Files:**
- Create: `internal/ui/wizard/section_theme.go`
- Create: `internal/ui/wizard/sections.go`
- Modify: `internal/ui/wizard/model.go` (orchestrator)
- Add: confirm + write screen as part of model.go or `section_confirm.go`

- [ ] **Step 12.1: Implement theme section with live preview**

`internal/ui/wizard/section_theme.go`:

```go
package wizard

import (
	"strings"

	"charm.land/huh/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/glw907/poplar/internal/theme"
)

type themeSection struct {
	parent *Model
	form   *huh.Form
	pick   string // bound to huh.Select's Value
}

func newThemeSection(parent *Model) *themeSection {
	s := &themeSection{parent: parent, pick: "one-dark"}
	opts := []huh.Option[string]{}
	for _, name := range theme.ThemeNames() {
		opts = append(opts, huh.NewOption(name, name))
	}
	s.form = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Choose a color theme — preview updates as you scroll").
			Options(opts...).
			Value(&s.pick),
	)).WithTheme(HuhTheme(parent.Theme))
	return s
}

func (s *themeSection) Init() tea.Cmd { return s.form.Init() }

func (s *themeSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := s.form.Update(msg)
	s.form = form.(*huh.Form)

	// Detect selection change → emit ThemeChangedMsg so the parent
	// rebuilds Styles + huh.Theme.
	if t, ok := theme.Themes[s.pick]; ok && t != s.parent.Theme {
		return s, tea.Batch(cmd, func() tea.Msg {
			return ThemeChangedMsg{Theme: t, Name: s.pick}
		})
	}

	return s, cmd
}

func (s *themeSection) View() string {
	form := s.form.View()
	preview := renderThemePreview(s.parent.Theme)
	// Split-pane layout per the spec wireframe
	return joinSplit(form, preview)
}

func renderThemePreview(t *theme.CompiledTheme) string {
	// Static "fake email view" rendered with the candidate theme.
	// Pulls from the same lipgloss styles the real app uses but
	// hardcodes content. Mirror the wireframe in the spec.
	var b strings.Builder
	b.WriteString("Inbox\n")
	b.WriteString("──────\n")
	b.WriteString("• Geoff Wright    11:47\n")
	b.WriteString("  Re: poplar setup...\n")
	b.WriteString("  Hannah W.        9:14\n")
	b.WriteString("  weekend plans\n")
	b.WriteString("• Sarah K.        Tue\n")
	b.WriteString("  build status: green\n")
	b.WriteString("\n")
	b.WriteString("q quit  / search  c new\n")
	// Apply theme's foreground/background to the block as a whole.
	return t.PreviewStyle().Render(b.String())
}

func joinSplit(left, right string) string {
	// Use lipgloss.JoinHorizontal here. SPUA cell width is 1 in the
	// wizard context (no fancy icons), so JoinHorizontal is safe.
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}
```

(`theme.CompiledTheme.PreviewStyle()` may need to be added to the theme package — a single bg-set Style. If not, use `lipgloss.NewStyle().Foreground(t.Foreground).Background(t.Background)` inline.)

- [ ] **Step 12.2: Implement section registry**

`internal/ui/wizard/sections.go`:

```go
package wizard

import (
	tea "github.com/charmbracelet/bubbletea"
)

// allSections returns the ordered registry. Stub sections (contacts,
// signatures, tidy) return Hide()=true and never run; they're listed
// so future passes can flip the flag without restructuring.
func defaultSections(m *Model) []sectionAdapter {
	return []sectionAdapter{
		{view: newAccountSection(m)},
		{view: newThemeSection(m)},
		{view: newConfirmSection(m)},
	}
}

// confirmSection shows the assembled TOML and writes it to disk.
type confirmSection struct {
	parent *Model
	// ...
}

func newConfirmSection(parent *Model) *confirmSection {
	return &confirmSection{parent: parent}
}

func (s *confirmSection) Init() tea.Cmd { return nil }
func (s *confirmSection) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Render assembled TOML; on Confirm, write atomically and emit
	// AdvanceMsg → wizard exits.
	return s, nil
}
func (s *confirmSection) View() string {
	// Render the TOML preview via config.Render(...) inside a frame.
	return ""
}
```

- [ ] **Step 12.3: Wire orchestrator in `model.go`**

Replace the placeholder `defaultSections` body with the real one. Wire `BackMsg` handling to jump back to credentials in the active section. Wire `CancelMsg{SaveCommented:true}` to write the partial commented-out block and exit.

- [ ] **Step 12.4: Run all tests**

```bash
make check
```

Expected: PASS.

- [ ] **Step 12.5: Commit**

```bash
git add internal/ui/wizard/
git commit -m "Pass 14 task 12: theme section + registry + orchestrator"
```

---

## Task 13: Cobra wiring + first-run auto-launch

**Files:**
- Create: `cmd/poplar/config_init.go`
- Modify: `cmd/poplar/config.go` (add `init` to the cmd tree)
- Modify: `cmd/poplar/root.go` (auto-launch + repair flag + opt-out)

- [ ] **Step 13.1: Implement `config init --interactive`**

`cmd/poplar/config_init.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/theme"
	uiwiz "github.com/glw907/poplar/internal/ui/wizard"
)

var (
	configInitInteractive bool
	configInitForce       bool
	configInitSection     string
)

func newConfigInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a fresh config.toml or run the interactive wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Resolve("")
			if err != nil { return err }

			if !configInitInteractive {
				// Existing template-write path
				if _, err := os.Stat(path); err == nil && !configInitForce {
					return fmt.Errorf("%s already exists; use --force to overwrite", path)
				}
				return os.WriteFile(path, []byte(config.Template()), 0o600)
			}

			// Interactive wizard
			t := theme.Themes["one-dark"]
			model := uiwiz.NewModel(t)
			if configInitSection != "" {
				model = model.WithSections(strings.Split(configInitSection, ","))
			}
			prog := tea.NewProgram(model, tea.WithAltScreen())
			_, err = prog.Run()
			return err
		},
	}
	cmd.Flags().BoolVar(&configInitInteractive, "interactive", false, "Run the interactive wizard")
	cmd.Flags().BoolVar(&configInitForce, "force", false, "Overwrite existing config.toml")
	cmd.Flags().StringVar(&configInitSection, "section", "", "Run only the named section(s) (comma-separated)")
	return cmd
}
```

(`Model.WithSections(names []string)` is a new helper that filters the section registry by name. Add it to `internal/ui/wizard/model.go`.)

- [ ] **Step 13.2: Wire into the existing cobra tree**

Find `newConfigCmd` (or equivalent) in `cmd/poplar/config_cmd.go`; add `cmd.AddCommand(newConfigInitCmd())`.

- [ ] **Step 13.3: First-run auto-launch in `runRoot`**

In `cmd/poplar/root.go`, replace the existing `ErrFirstRun` branch:

```go
accts, configPath, err := config.Load(f.config)
if errors.Is(err, config.ErrFirstRun) {
	if f.noWizard || os.Getenv("POPLAR_NO_WIZARD") != "" {
		fmt.Fprintln(os.Stderr, err.Error())
		fmt.Fprintln(os.Stderr, "Edit the file and run poplar again.")
		os.Exit(78)
	}
	// Auto-launch the wizard
	t := theme.Themes["one-dark"]
	model := uiwiz.NewModel(t)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return fmt.Errorf("wizard: %v", err)
	}
	// Re-load config now that the wizard wrote it.
	accts, configPath, err = config.Load(f.config)
	if err != nil {
		return fmt.Errorf("post-wizard config load: %v", err)
	}
}
```

Also handle `ConfigError`:

```go
var ce *config.ConfigError
if errors.As(err, &ce) {
	fmt.Fprintln(os.Stderr, "poplar:", ce.Error())
	if ce.Account != "" {
		fmt.Fprintf(os.Stderr,
			"Run `poplar --repair=%s` to fix this account interactively.\n", ce.Account)
	}
	fmt.Fprintln(os.Stderr, "Or edit the file by hand and rerun poplar.")
	os.Exit(78)
}
```

- [ ] **Step 13.4: Add `--no-wizard` and `--repair=<name>` flags**

Find `rootFlags` struct, add:

```go
type rootFlags struct {
	// existing fields…
	noWizard bool
	repair   string
}
```

Wire in the cobra setup:

```go
cmd.Flags().BoolVar(&f.noWizard, "no-wizard", false, "Don't auto-launch the wizard on first run")
cmd.Flags().StringVar(&f.repair, "repair", "", "Repair a specific account interactively")
```

Handle `--repair` early in `runRoot` (before `config.Load`):

```go
if f.repair != "" {
	t := theme.Themes["one-dark"]
	model := uiwiz.NewModel(t).WithRepair(f.repair)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	_, err := prog.Run()
	return err
}
```

(`Model.WithRepair(name)` pre-populates the wizard with the named account's existing config + jumps to its account section. New helper on the model.)

- [ ] **Step 13.5: Run end-to-end build + test**

```bash
make check
```

Expected: PASS.

- [ ] **Step 13.6: Commit**

```bash
git add cmd/poplar/
git commit -m "Pass 14 task 13: cobra wiring + first-run auto-launch"
```

---

## Task 14: Live tmux smoke test

- [ ] **Step 14.1: Build + install**

```bash
make install
```

- [ ] **Step 14.2: Run wizard in tmux at 80×24**

```bash
mv ~/.config/poplar ~/.config/poplar.bak  # backup current config
tmux new-session -d -s poplar-test -x 80 -y 24 'poplar'
sleep 1
tmux capture-pane -t poplar-test -p > /tmp/poplar-wizard-80x24.txt
tmux kill-session -t poplar-test
mv ~/.config/poplar.bak ~/.config/poplar  # restore
cat /tmp/poplar-wizard-80x24.txt
```

Expected: welcome screen with wordmark + tagline + start prompt.

- [ ] **Step 14.3: Run wizard at 120×40, walk through to provider picker**

```bash
mv ~/.config/poplar ~/.config/poplar.bak
tmux new-session -d -s poplar-test -x 120 -y 40 'poplar'
sleep 1
tmux send-keys -t poplar-test Enter   # advance from welcome
sleep 0.5
tmux capture-pane -t poplar-test -p > /tmp/poplar-wizard-120x40-provider.txt
tmux kill-session -t poplar-test
mv ~/.config/poplar.bak ~/.config/poplar
cat /tmp/poplar-wizard-120x40-provider.txt
```

Expected: provider picker visible with Fastmail at top.

- [ ] **Step 14.4: Test malformed-config error path**

```bash
# Write a bad config
mv ~/.config/poplar ~/.config/poplar.bak
mkdir -p ~/.config/poplar
cat > ~/.config/poplar/config.toml <<'EOF'
[[account]]
provider = "fastmail"
# missing email — should hit ConfigError path
EOF
poplar 2>&1 | head -10
mv ~/.config/poplar.bak ~/.config/poplar
```

Expected: friendly error pointing at the missing field, with `--repair=` hint.

- [ ] **Step 14.5: No commit yet — fix any issues found**

If anything looked wrong in the captures, fix in-place and recapture before continuing.

---

## Task 15: ADR + invariants update

**Files:**
- Create: `docs/poplar/decisions/0189-firstrun-wizard.md`
- Modify: `docs/poplar/invariants.md`
- Modify: `docs/poplar/decisions/INDEX.md`

- [ ] **Step 15.1: Write ADR-0189**

`docs/poplar/decisions/0189-firstrun-wizard.md`:

```markdown
---
title: First-run wizard + section-pluggable architecture
status: accepted
date: 2026-05-09
---

## Context

#27 (first-run wizard) and #29 (config template + malformed-config
UX) are both v1 blockers. They share a surface — the user's
first interaction with poplar's configuration — so they ship in one
pass. OAuth refresh-token handling (#27's hardest dependency) splits
out as Pass 14.1 to keep this pass within the 8–12 task budget.

## Decision

Pass 14 introduces a section-pluggable wizard at
`internal/wizard/` (UI-free domain) and `internal/ui/wizard/`
(bubbletea + huh UI). Adding a new configurable surface (contacts,
signatures, tidy, future auth modes) means adding a `Section` to the
registry; sections are addressable from the CLI by name.

`config.Provider` gains `CredentialStrategy` (enum routing per-preset
to `appPassword`, `apiToken`, `oauth`, `plainIMAP`, or `plainJMAP`)
and `HelpURL` (the page where users generate credentials).

Probing is a step-by-step transcript. `mailimap.Probe` and
`mailjmap.Probe` mirror the existing `Connect`/`ProbeSMTP` flows
into per-step output; `wizard.Probe` dispatches on backend.

Malformed-config errors get a typed wrapper (`config.ConfigError`)
that surfaces file path, line, account, field, message, and a
suggested fix. `runRoot` formats them and points users at
`poplar --repair=<account>` for interactive recovery.

Logo: typographic wordmark for now (no tree art ready for
production); `art/poplar-logo.ans` carries a cbonsai artifact for
later swap. Replacing the wordmark is a one-line change in
`internal/ui/wizard/logo.go`.

UI library: `charm.land/huh/v2` for stepped form pages. Probe and
(eventually) OAuth use a custom `tea.Model` orchestrated around the
form. The probe screen is the only deviation from "huh handles all
form pages" — its live spinner + transcript transitions can't fit
huh's static `Note` field.

## Consequences

- New direct dep: `charm.land/huh/v2`. Charm-maintained, MIT,
  matches poplar's existing ecosystem.
- The wizard is *the* startup path for new users. Pre-existing
  scripts that depended on `ErrFirstRun → exit 78` need to set
  `POPLAR_NO_WIZARD=1` or `--no-wizard` to keep that behavior.
- Pass 14.1 will replace the OAuth strategy's placeholder
  `password-cmd` with a real device-code flow and keyring storage.
  No data migration needed because the placeholder is just a
  string in `config.toml` — the wizard rewrites it on next run.
- `name`-defaults-to-`email` is a behavior change: existing configs
  with empty `name` no longer fail, they silently default. This
  matches the template's longstanding promise.
```

- [ ] **Step 15.2: Update invariants**

In `docs/poplar/invariants.md`, find the architecture section and add the wizard binding facts. Keep the file ≤400 lines — replace or narrow existing facts where this pass changes them. Specifically:

- Add `internal/wizard/` and `internal/ui/wizard/` to the package layout description.
- Update the "First-run flow" entry under "Config & theming" — it currently says `ErrFirstRun` writes template + returns; replace with auto-launches wizard, opt out via `--no-wizard`/`POPLAR_NO_WIZARD=1`.
- Add `CredentialStrategy` + `HelpURL` to the Provider description.
- Add `mail.ProbeResult`/`ProbeStep`/`ProbeStatus` to the Mail model section.
- Add `mailimap.Probe`/`mailjmap.Probe`/`wizard.Probe` (replacing/extending the existing `mailimap.ProbeSMTP` mention).
- Add `config.ConfigError` to the Config section.

- [ ] **Step 15.3: Update INDEX.md**

In `docs/poplar/decisions/INDEX.md`, add a row for ADR-0189 under the appropriate theme (likely "Onboarding" or "Configuration").

- [ ] **Step 15.4: make check**

```bash
make check
```

Expected: pass. Voice check (`scripts/voice-check.sh`) flags any AI-tells in the new docs — fix any hits before committing.

- [ ] **Step 15.5: Commit**

```bash
git add docs/
git commit -m "Pass 14 task 15: ADR-0189 + invariants + INDEX"
```

---

## Task 16: Pass-end consolidation ritual

Invoke the `poplar-pass` skill — it covers /simplify, the
idiomatic-bubbletea check (since `internal/ui/` changed), STATUS.md
update, plan archival, and final make check + commit + push +
install.

- [ ] **Step 16.1: Run /simplify**

Invoke `simplify` skill on the diff. Apply genuine wins.

- [ ] **Step 16.2: Run §10 idiomatic-bubbletea checklist**

Open `docs/poplar/bubbletea-conventions.md` §10 and verify each item against the wizard diff. Fix any deviations.

- [ ] **Step 16.3: Update STATUS.md**

Mark Pass 14 done. Replace starter prompt with Pass 14.1 (OAuth) per the format in the `poplar-pass` skill.

- [ ] **Step 16.4: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-09-pass-14-firstrun.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-09-pass-14-firstrun-design.md docs/superpowers/archive/specs/
```

- [ ] **Step 16.5: Final make check + commit + push + install**

```bash
make check
git add -A
git commit -m "Pass 14: STATUS, archive plan + spec"
git push
make install
```

---

## Self-review notes

- **Spec coverage**: every section of the spec maps to a task. Task 1 = ProbeResult types; tasks 2–3 = mailimap/mailjmap.Probe; task 4 = CredentialStrategy + HelpURL; task 5 = ConfigError + #29 fix; task 6 = wizard skeleton + Strategy + Apply; task 7 = wizard.Probe dispatcher; task 8 = config.Render; task 9 = template rewrite; task 10 = wizard UI skeleton + theme adapter + logo; task 11 = account section + probe screen; task 12 = theme section + registry; task 13 = cobra wiring; task 14 = tmux smoke; task 15 = ADR + invariants; task 16 = pass-end ritual.
- **Type consistency**: `ProbeResult` / `ProbeStep` / `ProbeStatus` used identically across tasks 1–7. `CredentialStrategy` enum names match between tasks 4, 6, 11. `Model.Port` flagged in task 11 as a recommend-string-not-int change to task 6's domain model — apply when you reach task 11 if you take the recommendation.
- **Known underspecifications**: a few function signatures (`uicore.TitleCase`, `theme.CompiledTheme.PreviewStyle`, exact field names on existing types) are written as the engineer should expect them and may need to be added or renamed when reading the actual current code. These are noted inline in the relevant tasks.
