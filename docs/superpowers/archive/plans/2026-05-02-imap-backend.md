# IMAP Backend (Pass 8) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a generic IMAP backend (`internal/mailimap`) plus a provider-registry config layer that enables `imap`/`yahoo`/`icloud`/`zoho`/`jmap`/`fastmail` as `backend = "..."` values, leaving Gmail and the configuration UX to follow-on passes (8.1, 8.2).

**Architecture:** Two physical IMAP connections per `Backend` (command + IDLE), capability-negotiated behavior (UIDPLUS-required, MOVE-with-fallback, SPECIAL-USE-with-alias-fallback, IDLE-with-poll-fallback), 9-minute IDLE refresh, exponential reconnect mirroring `mailjmap.pushLoop`. SASL via `emersion/go-sasl` (PLAIN/LOGIN/CRAM-MD5; XOAUTH2 adapter scaffolded but routed only when `auth = "xoauth2"` is set). Provider registry is a small data table in `internal/config/providers.go`.

**Tech Stack:** Go 1.26, `github.com/emersion/go-imap` v1, `github.com/emersion/go-sasl`, `github.com/BurntSushi/toml`, `github.com/spf13/cobra`. Unit tests use a fake IMAP client interface; integration tests opt-in against local Dovecot (`docker run -d -p 1143:143 dovecot/dovecot`).

**Reference:** Spec at `docs/superpowers/specs/2026-05-01-imap-backend-design.md`. Mirror patterns from `internal/mailjmap/` (especially `jmap.go` Connect-phase ordering, `push.go` reconnect loop, `fake_test.go` client interface). Conform to `go-conventions` skill — no unnecessary interfaces, errors wrapped with `%w`, table-driven tests.

---

## File Structure

| Action | File | Responsibility |
|---|---|---|
| Modify | `go.mod`, `go.sum` | Add `github.com/emersion/go-imap` v1 |
| Create | `internal/config/providers.go` | `Provider` struct + `Providers` registry |
| Create | `internal/config/providers_test.go` | Registry lookup tests |
| Modify | `internal/config/account.go` | New fields: `Auth`, `Email`, `Host`, `Port`, `StartTLS`, `OAuthClientID`, `OAuthClientSecret`, `OAuthRefreshToken` |
| Modify | `internal/config/accounts.go` | `accountEntry` decoding for new fields; preset resolution against `Providers` |
| Modify | `internal/config/accounts_test.go` | Tests for preset resolution + new fields |
| Modify | `internal/mail/classify.go` | Add IMAP-style folder name aliases (already has Outlook variants; add Yahoo "Bulk Mail" et al. — verify what's missing) |
| Modify | `internal/mail/classify_test.go` | Cover new aliases |
| Create | `internal/mailimap/imap.go` | `Backend` struct, `New`, `Connect`, `Disconnect`, capability negotiation, `AccountName`/`AccountEmail`/`Updates` |
| Create | `internal/mailimap/auth.go` | `dialCommand`/`dialIdle` — TLS/STARTTLS dial, keepalive setup, SASL mechanism dispatch |
| Create | `internal/mailimap/folders.go` | `ListFolders` (SPECIAL-USE + alias fallback), `OpenFolder` |
| Create | `internal/mailimap/messages.go` | `QueryFolder`, `FetchHeaders`, `FetchBody`, `Search` |
| Create | `internal/mailimap/actions.go` | `Move` (UID MOVE or COPY+STORE+EXPUNGE fallback), `Copy`, `Delete`, `Destroy`, `Flag`, `MarkRead`, `MarkUnread`, `MarkAnswered`, `Send` (stub) |
| Create | `internal/mailimap/idle.go` | Idle goroutine: IDLE-or-poll, 9-min refresh, folder switch, reconnect loop, emit `mail.Update` |
| Create | `internal/mailimap/fake_test.go` | Fake IMAP client implementing the `imapClient` interface for unit tests |
| Create | `internal/mailimap/imap_test.go` | Tests for Connect/Disconnect/capability negotiation |
| Create | `internal/mailimap/folders_test.go` | Tests for `ListFolders`, `OpenFolder` |
| Create | `internal/mailimap/messages_test.go` | Tests for query/fetch/search |
| Create | `internal/mailimap/actions_test.go` | Tests for triage actions (incl. Move fallback path) |
| Create | `internal/mailimap/idle_test.go` | Tests for idle loop, refresh, reconnect, polling fallback |
| Create | `internal/mailimap/integration_test.go` | Tagged `//go:build integration` Dovecot suite |
| Create | `internal/mailimap/README.md` | Package overview + Dovecot test-server setup |
| Modify | `cmd/poplar/backend.go` | Dispatch `imap`/`yahoo`/`icloud`/`zoho` → `mailimap.New`, `jmap`/`fastmail` → `mailjmap.New` |
| Modify | `Makefile` | `test-imap` target for integration tests |

---

### Task 1: Add emersion/go-imap v1 dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
cd /home/glw907/Projects/poplar
go get github.com/emersion/go-imap/v2@latest
```

(Note: emersion/go-imap v2 is the current mainline as of 2026; the package import path is `github.com/emersion/go-imap/v2`. If v2 turns out to lack required surface during implementation, fall back to v1 at `github.com/emersion/go-imap` and adjust imports.)

- [ ] **Step 2: Verify go.mod was updated**

```bash
grep emersion/go-imap go.mod
```

Expected: a `require` line containing `github.com/emersion/go-imap/v2`.

- [ ] **Step 3: Run `make check` — must still pass**

```bash
make check
```

Expected: PASS (no code uses the new import yet).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "Pass 8: add emersion/go-imap dependency"
```

---

### Task 2: Provider registry struct + tests (no resolution yet)

**Files:**
- Create: `internal/config/providers.go`
- Create: `internal/config/providers_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/providers_test.go`:

```go
// SPDX-License-Identifier: MIT

package config

import "testing"

func TestProviderRegistryLookup(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantOK   bool
		wantHost string
		wantURL  string
	}{
		{"fastmail preset", "fastmail", true, "", "https://api.fastmail.com/jmap/session"},
		{"yahoo preset", "yahoo", true, "imap.mail.yahoo.com", ""},
		{"icloud preset", "icloud", true, "imap.mail.me.com", ""},
		{"zoho preset", "zoho", true, "imap.zoho.com", ""},
		{"unknown preset", "nonesuch", false, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := LookupProvider(tc.key)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if p.Host != tc.wantHost {
				t.Errorf("host = %q, want %q", p.Host, tc.wantHost)
			}
			if p.URL != tc.wantURL {
				t.Errorf("url = %q, want %q", p.URL, tc.wantURL)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/ -run TestProviderRegistryLookup
```

Expected: FAIL — `LookupProvider` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/config/providers.go`:

```go
// SPDX-License-Identifier: MIT

package config

// Provider is a built-in account preset that fills in protocol,
// host/port (IMAP), or session URL (JMAP) so users don't have to
// look those up. Auth is still supplied per-account in accounts.toml.
type Provider struct {
	Name        string
	Backend     string // "imap" or "jmap"
	Host        string // IMAP presets only
	Port        int
	StartTLS    bool
	URL         string // JMAP presets only
	AuthHint    string // "app-password" | "bearer" | "xoauth2"
	HelpURL     string
	GmailQuirks bool // Pass 8.1 — assert-not-negotiate + Trash precondition
}

// Providers maps preset name → Provider. Adding a new well-known
// service is one struct literal.
var Providers = map[string]Provider{
	"fastmail": {
		Name:     "fastmail",
		Backend:  "jmap",
		URL:      "https://api.fastmail.com/jmap/session",
		AuthHint: "bearer",
		HelpURL:  "https://app.fastmail.com/settings/security/tokens",
	},
	"yahoo": {
		Name:     "yahoo",
		Backend:  "imap",
		Host:     "imap.mail.yahoo.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://login.yahoo.com/account/security",
	},
	"icloud": {
		Name:     "icloud",
		Backend:  "imap",
		Host:     "imap.mail.me.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://appleid.apple.com",
	},
	"zoho": {
		Name:     "zoho",
		Backend:  "imap",
		Host:     "imap.zoho.com",
		Port:     993,
		AuthHint: "app-password",
		HelpURL:  "https://accounts.zoho.com/home#security/app_password",
	},
}

// LookupProvider returns the Provider for key and true if known.
func LookupProvider(key string) (Provider, bool) {
	p, ok := Providers[key]
	return p, ok
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/config/ -run TestProviderRegistryLookup
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/providers.go internal/config/providers_test.go
git commit -m "Pass 8: add provider registry (fastmail, yahoo, icloud, zoho)"
```

---

### Task 3: Extend `AccountConfig` with new fields

**Files:**
- Modify: `internal/config/account.go`

- [ ] **Step 1: Add new fields**

Edit `internal/config/account.go`. Insert the new fields into the existing `AccountConfig` struct, after the existing `Backend`/`Source` block. Final shape:

```go
// AccountConfig holds the configuration for a single email account.
type AccountConfig struct {
	Name           string
	Display        string
	Backend        string
	Source         string
	Params         map[string]string
	Folders        []string
	FoldersExclude []string
	Headers        []string
	HeadersExclude []string
	CheckMail      time.Duration

	// Identity
	From    *mail.Address
	Aliases []*mail.Address
	CopyTo  []string

	// Credentials
	Password string

	// Auth (Pass 8). Recognized values: "plain", "login", "cram-md5",
	// "xoauth2", "bearer". Empty string defers to backend default.
	Auth string

	// Email is the user's address. May be empty for backends that
	// auto-discover (JMAP session). Used as SASL username for IMAP.
	Email string

	// IMAP transport (set directly via accounts.toml or via a
	// provider preset).
	Host     string
	Port     int
	StartTLS bool

	// XOAUTH2 inputs. All env-var-substituted via $VAR.
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRefreshToken string

	// Outgoing
	Outgoing        string
	OutgoingCredCmd string
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 3: Run existing config tests — must still pass**

```bash
go test ./internal/config/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/config/account.go
git commit -m "Pass 8: extend AccountConfig with auth/host/port/oauth fields"
```

---

### Task 4: Decode new TOML fields + provider preset resolution

**Files:**
- Modify: `internal/config/accounts.go`
- Modify: `internal/config/accounts_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/accounts_test.go`:

```go
func TestParseAccountsResolvesYahooPreset(t *testing.T) {
	t.Setenv("YAHOO_APP_PASSWORD", "secret-app-pw")
	toml := `
[[account]]
name     = "personal"
backend  = "yahoo"
email    = "user@yahoo.com"
auth     = "plain"
password = "$YAHOO_APP_PASSWORD"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	a := got[0]
	if a.Backend != "imap" {
		t.Errorf("Backend = %q, want imap (preset should resolve)", a.Backend)
	}
	if a.Host != "imap.mail.yahoo.com" {
		t.Errorf("Host = %q, want imap.mail.yahoo.com", a.Host)
	}
	if a.Port != 993 {
		t.Errorf("Port = %d, want 993", a.Port)
	}
	if a.Email != "user@yahoo.com" {
		t.Errorf("Email = %q, want user@yahoo.com", a.Email)
	}
	if a.Auth != "plain" {
		t.Errorf("Auth = %q, want plain", a.Auth)
	}
	if a.Password != "secret-app-pw" {
		t.Errorf("Password = %q, want resolved env value", a.Password)
	}
}

func TestParseAccountsExplicitImap(t *testing.T) {
	t.Setenv("IMAP_PASS", "raw-pw")
	toml := `
[[account]]
name     = "self"
backend  = "imap"
email    = "u@example.com"
host     = "mail.example.com"
port     = 143
starttls = true
auth     = "plain"
password = "$IMAP_PASS"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := got[0]
	if a.Host != "mail.example.com" || a.Port != 143 || !a.StartTLS {
		t.Errorf("transport mis-set: %+v", a)
	}
}

func TestParseAccountsResolvesFastmailPreset(t *testing.T) {
	t.Setenv("FASTMAIL_TOKEN", "tok")
	toml := `
[[account]]
name     = "fm"
backend  = "fastmail"
email    = "u@fastmail.com"
auth     = "bearer"
password = "$FASTMAIL_TOKEN"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := got[0]
	if a.Backend != "jmap" {
		t.Errorf("Backend = %q, want jmap", a.Backend)
	}
	if a.Source != "https://api.fastmail.com/jmap/session" {
		t.Errorf("Source = %q, want preset URL", a.Source)
	}
}

func TestParseAccountsOAuthFieldsResolved(t *testing.T) {
	t.Setenv("OA_CID", "the-client-id")
	t.Setenv("OA_CS", "the-client-secret")
	t.Setenv("OA_RT", "the-refresh-token")
	toml := `
[[account]]
name                = "wk"
backend             = "imap"
email               = "u@example.com"
host                = "imap.example.com"
port                = 993
auth                = "xoauth2"
oauth-client-id     = "$OA_CID"
oauth-client-secret = "$OA_CS"
oauth-refresh-token = "$OA_RT"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a := got[0]
	if a.OAuthClientID != "the-client-id" {
		t.Errorf("OAuthClientID = %q", a.OAuthClientID)
	}
	if a.OAuthClientSecret != "the-client-secret" {
		t.Errorf("OAuthClientSecret = %q", a.OAuthClientSecret)
	}
	if a.OAuthRefreshToken != "the-refresh-token" {
		t.Errorf("OAuthRefreshToken = %q", a.OAuthRefreshToken)
	}
}
```

Also adjust the existing `toAccountConfig` validation: the rule "Source is required" must not fire for IMAP-style presets that supply Host instead of Source. Either remove the requirement and add a per-backend validation later, or allow Source-or-Host. Pick remove-the-eager-check; protocol packages already validate their own inputs.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/ -run TestParseAccounts
```

Expected: FAIL — preset resolution + new field decoding not implemented.

- [ ] **Step 3: Implement**

Edit `internal/config/accounts.go`. Replace the `accountEntry` struct and `toAccountConfig` method with:

```go
type accountEntry struct {
	Name              string            `toml:"name"`
	Display           string            `toml:"display"`
	Backend           string            `toml:"backend"`
	Source            string            `toml:"source"`
	Email             string            `toml:"email"`
	Host              string            `toml:"host"`
	Port              int               `toml:"port"`
	StartTLS          bool              `toml:"starttls"`
	Auth              string            `toml:"auth"`
	Password          string            `toml:"password"`
	OAuthClientID     string            `toml:"oauth-client-id"`
	OAuthClientSecret string            `toml:"oauth-client-secret"`
	OAuthRefreshToken string            `toml:"oauth-refresh-token"`
	CredentialCmd     string            `toml:"credential-cmd"`
	CopyTo            string            `toml:"copy-to"`
	FoldersSort       []string          `toml:"folders-sort"`
	FoldersExclude    []string          `toml:"folders-exclude"`
	From              string            `toml:"from"`
	Outgoing          string            `toml:"outgoing"`
	OutgoingCredCmd   string            `toml:"outgoing-credential-cmd"`
	Params            map[string]string `toml:"params"`
}
```

Then in `toAccountConfig`, after the existing name check, replace the Source-required check with preset resolution:

```go
func (e *accountEntry) toAccountConfig(index int) (*AccountConfig, error) {
	if e.Name == "" {
		return nil, fmt.Errorf("account %d: name is required", index)
	}

	backend := e.Backend
	host := e.Host
	port := e.Port
	startTLS := e.StartTLS
	source := e.Source

	if preset, ok := LookupProvider(e.Backend); ok {
		backend = preset.Backend
		if host == "" {
			host = preset.Host
		}
		if port == 0 {
			port = preset.Port
		}
		if !startTLS {
			startTLS = preset.StartTLS
		}
		if source == "" {
			source = preset.URL
		}
	}

	password, err := resolveEnv(e.Password)
	if err != nil {
		return nil, fmt.Errorf("account %q password: %w", e.Name, err)
	}
	clientID, err := resolveEnv(e.OAuthClientID)
	if err != nil {
		return nil, fmt.Errorf("account %q oauth-client-id: %w", e.Name, err)
	}
	clientSecret, err := resolveEnv(e.OAuthClientSecret)
	if err != nil {
		return nil, fmt.Errorf("account %q oauth-client-secret: %w", e.Name, err)
	}
	refresh, err := resolveEnv(e.OAuthRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("account %q oauth-refresh-token: %w", e.Name, err)
	}

	if e.CredentialCmd != "" {
		cred, err := runCredentialCmd(e.CredentialCmd)
		if err != nil {
			return nil, fmt.Errorf("account %q: credential command: %w", e.Name, err)
		}
		source, err = injectCredential(source, cred)
		if err != nil {
			return nil, fmt.Errorf("account %q: injecting credential: %w", e.Name, err)
		}
	}

	acct := &AccountConfig{
		Name:              e.Name,
		Display:           e.Display,
		Backend:           backend,
		Source:            source,
		Email:             e.Email,
		Host:              host,
		Port:              port,
		StartTLS:          startTLS,
		Auth:              e.Auth,
		Password:          password,
		OAuthClientID:     clientID,
		OAuthClientSecret: clientSecret,
		OAuthRefreshToken: refresh,
		Folders:           e.FoldersSort,
		FoldersExclude:    e.FoldersExclude,
		Params:            e.Params,
		Outgoing:          e.Outgoing,
		OutgoingCredCmd:   e.OutgoingCredCmd,
	}

	if e.CopyTo != "" {
		acct.CopyTo = []string{e.CopyTo}
	}

	if e.From != "" {
		addrs, err := mail.ParseAddressList(e.From)
		if err != nil {
			return nil, fmt.Errorf("account %q: parsing from address: %w", e.Name, err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("account %q: from address is empty", e.Name)
		}
		acct.From = addrs[0]
	}

	return acct, nil
}
```

- [ ] **Step 4: Run all config tests**

```bash
go test ./internal/config/...
```

Expected: PASS. If any pre-existing test relied on the "Source is required" error message, update it — Source is now backend-validated, not eagerly validated.

- [ ] **Step 5: Commit**

```bash
git add internal/config/accounts.go internal/config/accounts_test.go
git commit -m "Pass 8: decode new TOML fields and resolve provider presets"
```

---

### Task 5: Skeleton `mailimap.Backend`

**Files:**
- Create: `internal/mailimap/imap.go`

- [ ] **Step 1: Create the skeleton**

Create `internal/mailimap/imap.go`:

```go
// SPDX-License-Identifier: MIT

// Package mailimap implements mail.Backend over IMAP4rev1 using
// emersion/go-imap. Capabilities are negotiated at Connect; UIDPLUS
// is required, MOVE / SPECIAL-USE / IDLE are used opportunistically.
//
// A Backend owns two physical IMAP connections: a synchronous
// "command" connection used by every mail.Backend method, and an
// "idle" connection that runs in a goroutine emitting mail.Update
// values. Both share the auth path defined in auth.go.
package mailimap

import (
	"context"
	"sync"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// Backend is one IMAP account.
type Backend struct {
	cfg config.AccountConfig

	mu      sync.Mutex
	cmd     imapClient // command connection (nil before Connect)
	idle    imapClient // idle connection
	caps    capSet
	current string // currently-selected folder on cmd
	updates chan mail.Update

	idleCancel context.CancelFunc
	idleDone   chan struct{}
	switchCh   chan string // folder-switch signal to idle goroutine
}

// capSet records the capabilities advertised by the server. UIDPLUS
// is required and Connect refuses to proceed without it.
type capSet struct {
	UIDPLUS     bool
	MOVE        bool
	IDLE        bool
	SpecialUse  bool
	XGM         bool // X-GM-EXT-1, set by Pass 8.1 when GmailQuirks is on
}

// New constructs an unconnected Backend for cfg.
func New(cfg config.AccountConfig) *Backend {
	return &Backend{cfg: cfg}
}

// AccountName satisfies mail.Backend.
func (b *Backend) AccountName() string {
	if b.cfg.Display != "" {
		return b.cfg.Display
	}
	if b.cfg.Email != "" {
		return b.cfg.Email
	}
	return b.cfg.Name
}

// AccountEmail satisfies mail.Backend.
func (b *Backend) AccountEmail() string {
	if b.cfg.From != nil && b.cfg.From.Address != "" {
		return b.cfg.From.Address
	}
	return b.cfg.Email
}

// Updates satisfies mail.Backend. Returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }
```

Also create the `imapClient` interface in a separate file so tests can fake it. Create `internal/mailimap/client.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"io"

	"github.com/glw907/poplar/internal/mail"
)

// imapClient is the subset of go-imap's client surface that mailimap
// uses. The real *imapclient.Client satisfies it (via a thin adapter
// in auth.go); tests substitute a fake.
//
// Method signatures will be fleshed out as each task lands. Each
// method should return errors with the wrapped IMAP server response
// when applicable so the error banner can surface useful detail.
type imapClient interface {
	// Authenticate runs SASL with the given mechanism name + client.
	// Logout closes the connection cleanly.
	Logout() error

	// Capabilities returns the advertised capability set as a map.
	Capabilities() (map[string]bool, error)

	// List runs LIST/LSUB and returns folders. specialUse causes
	// the LIST RETURN (SPECIAL-USE) variant when supported.
	List(ref, pattern string, specialUse bool) ([]listEntry, error)

	// Select selects a folder and returns its summary.
	Select(folder string, readOnly bool) (mail.Folder, error)

	// Search runs UID SEARCH with the criteria and returns matching UIDs.
	Search(criteria mail.SearchCriteria) ([]mail.UID, error)

	// Fetch runs UID FETCH; resultFn is called once per message.
	Fetch(uids []mail.UID, items []string, resultFn func(uid mail.UID, items map[string]any)) error

	// FetchBody returns a reader for the full RFC 822 body of one UID.
	FetchBody(uid mail.UID) (io.ReadCloser, error)

	// Store runs UID STORE.
	Store(uids []mail.UID, item string, value any) error

	// Copy and Move are UID COPY / UID MOVE.
	Copy(uids []mail.UID, dest string) error
	Move(uids []mail.UID, dest string) error

	// Expunge runs plain EXPUNGE; UIDExpunge runs UID EXPUNGE (UIDPLUS).
	UIDExpunge(uids []mail.UID) error

	// Idle blocks until the server tears down or DONE is sent;
	// onUpdate is called per unilateral response.
	Idle(onUpdate func(mail.Update)) error
	IdleStop() // sends DONE
}

// listEntry is the result of a LIST command for one folder.
type listEntry struct {
	Name        string
	Attributes  []string // includes \Drafts, \Sent, \Trash, etc. when SPECIAL-USE
	HasChildren bool
}
```

(The exact emersion/go-imap v1/v2 method names will differ — adapt the adapter in `auth.go` later. The interface above is what mailimap *uses*; the real client wraps the library.)

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/mailimap/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/mailimap/imap.go internal/mailimap/client.go
git commit -m "Pass 8: scaffold mailimap.Backend + imapClient interface"
```

---

### Task 6: Fake IMAP client for unit tests

**Files:**
- Create: `internal/mailimap/fake_test.go`

- [ ] **Step 1: Implement the fake**

Create `internal/mailimap/fake_test.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"
	"io"

	"github.com/glw907/poplar/internal/mail"
)

// fakeClient is an in-memory imapClient for unit tests. Tests
// populate the maps directly; methods return canned data or run
// caller-supplied funcs.
type fakeClient struct {
	caps         map[string]bool
	folders      []listEntry
	folderSummary map[string]mail.Folder
	selected     string

	bodies map[mail.UID]string

	storeCalls    [][3]any // {uids, item, value}
	moveCalls     [][2]any // {uids, dest}
	copyCalls     [][2]any
	expungeCalls  [][]mail.UID

	onIdle    func(emit func(mail.Update)) error
	idleStop  func()

	logoutErr error
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		caps:          map[string]bool{},
		folderSummary: map[string]mail.Folder{},
		bodies:        map[mail.UID]string{},
	}
}

func (f *fakeClient) Logout() error { return f.logoutErr }

func (f *fakeClient) Capabilities() (map[string]bool, error) { return f.caps, nil }

func (f *fakeClient) List(ref, pattern string, specialUse bool) ([]listEntry, error) {
	return f.folders, nil
}

func (f *fakeClient) Select(folder string, readOnly bool) (mail.Folder, error) {
	f.selected = folder
	if s, ok := f.folderSummary[folder]; ok {
		return s, nil
	}
	return mail.Folder{Name: folder}, nil
}

func (f *fakeClient) Search(c mail.SearchCriteria) ([]mail.UID, error) { return nil, nil }

func (f *fakeClient) Fetch(uids []mail.UID, items []string, resultFn func(mail.UID, map[string]any)) error {
	return nil
}

func (f *fakeClient) FetchBody(uid mail.UID) (io.ReadCloser, error) {
	body, ok := f.bodies[uid]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(stringReader(body)), nil
}

func (f *fakeClient) Store(uids []mail.UID, item string, value any) error {
	f.storeCalls = append(f.storeCalls, [3]any{uids, item, value})
	return nil
}

func (f *fakeClient) Copy(uids []mail.UID, dest string) error {
	f.copyCalls = append(f.copyCalls, [2]any{uids, dest})
	return nil
}

func (f *fakeClient) Move(uids []mail.UID, dest string) error {
	f.moveCalls = append(f.moveCalls, [2]any{uids, dest})
	return nil
}

func (f *fakeClient) UIDExpunge(uids []mail.UID) error {
	f.expungeCalls = append(f.expungeCalls, uids)
	return nil
}

func (f *fakeClient) Idle(onUpdate func(mail.Update)) error {
	if f.onIdle != nil {
		return f.onIdle(onUpdate)
	}
	return nil
}

func (f *fakeClient) IdleStop() {
	if f.idleStop != nil {
		f.idleStop()
	}
}

type stringReader string

func (s stringReader) Read(p []byte) (int, error) {
	if len(s) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s)
	return n, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/mailimap/...
go test -count=1 ./internal/mailimap/... -run NONE  # build tests
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/mailimap/fake_test.go
git commit -m "Pass 8: add fakeClient for mailimap unit tests"
```

---

### Task 7: Connect / Disconnect with capability negotiation

**Files:**
- Modify: `internal/mailimap/imap.go`
- Create: `internal/mailimap/imap_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mailimap/imap_test.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/config"
)

// newWithFake returns a Backend wired to a fake client for tests.
// Construction bypasses the network dial so unit tests don't need
// a live server.
func newWithFake(cfg config.AccountConfig, cmd, idle imapClient) *Backend {
	b := New(cfg)
	b.cmd = cmd
	b.idle = idle
	return b
}

func TestConnectFailsWithoutUIDPLUS(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true} // no UIDPLUS
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	err := b.finishConnect(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "UIDPLUS") {
		t.Errorf("error = %v, want UIDPLUS mention", err)
	}
}

func TestConnectStoresCapabilities(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "MOVE": true, "IDLE": true, "SPECIAL-USE": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !b.caps.UIDPLUS || !b.caps.MOVE || !b.caps.IDLE || !b.caps.SpecialUse {
		t.Errorf("caps = %+v", b.caps)
	}
}

func TestDisconnectLogsOutBoth(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	cmd.logoutErr = errors.New("cmd-err")
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Disconnect should attempt both even if cmd fails.
	if err := b.Disconnect(); err == nil {
		t.Errorf("expected error from cmd Logout, got nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/mailimap/...
```

Expected: FAIL — `finishConnect` and `Disconnect` undefined.

- [ ] **Step 3: Implement `finishConnect`, `Disconnect`, and a `Connect` shell**

Append to `internal/mailimap/imap.go`:

```go
import (
	// ... existing imports ...
	"errors"
	"fmt"
)

const (
	updatesBuffer = 64
)

// Connect satisfies mail.Backend. It dials both connections,
// authenticates, negotiates capabilities, and starts the idle
// goroutine. The dial happens in auth.go; tests bypass by setting
// b.cmd / b.idle directly and calling finishConnect.
func (b *Backend) Connect(ctx context.Context) error {
	cmd, err := dialCommand(b.cfg)
	if err != nil {
		return fmt.Errorf("connect cmd: %w", err)
	}
	idle, err := dialIdle(b.cfg)
	if err != nil {
		_ = cmd.Logout()
		return fmt.Errorf("connect idle: %w", err)
	}
	b.mu.Lock()
	b.cmd = cmd
	b.idle = idle
	b.mu.Unlock()

	return b.finishConnect(ctx)
}

// finishConnect runs the post-dial bringup: capability negotiation,
// channel setup, idle-goroutine spawn. Split out so unit tests can
// drive it with fakes.
func (b *Backend) finishConnect(ctx context.Context) error {
	caps, err := b.cmd.Capabilities()
	if err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
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

	updates := make(chan mail.Update, updatesBuffer)

	b.mu.Lock()
	b.caps = cs
	b.updates = updates
	b.switchCh = make(chan string, 1)
	idleCtx, cancel := context.WithCancel(context.Background())
	b.idleCancel = cancel
	b.idleDone = make(chan struct{})
	b.mu.Unlock()

	go b.idleLoop(idleCtx)

	return nil
}

// Disconnect satisfies mail.Backend. Tears down the idle goroutine
// then logs out both connections. Returns the first non-nil error.
func (b *Backend) Disconnect() error {
	b.mu.Lock()
	cancel := b.idleCancel
	done := b.idleDone
	cmd := b.cmd
	idle := b.idle
	b.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	var firstErr error
	if cmd != nil {
		if err := cmd.Logout(); err != nil {
			firstErr = err
		}
	}
	if idle != nil {
		if err := idle.Logout(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// idleLoop is implemented in idle.go. Stub here so finishConnect
// compiles before Task 13 lands.
func (b *Backend) idleLoop(ctx context.Context) {
	defer close(b.idleDone)
	<-ctx.Done()
}
```

Also create a stub `auth.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"

	"github.com/glw907/poplar/internal/config"
)

// dialCommand and dialIdle are filled in by Task 8.
func dialCommand(cfg config.AccountConfig) (imapClient, error) {
	return nil, errors.New("dialCommand: not implemented (Task 8)")
}

func dialIdle(cfg config.AccountConfig) (imapClient, error) {
	return nil, errors.New("dialIdle: not implemented (Task 8)")
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/mailimap/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/imap.go internal/mailimap/imap_test.go internal/mailimap/auth.go
git commit -m "Pass 8: Connect/Disconnect with capability negotiation"
```

---

### Task 8: Real IMAP dial with TLS, keepalive, SASL dispatch

**Files:**
- Modify: `internal/mailimap/auth.go`

**Note:** This task interacts with the actual `emersion/go-imap` package. Confirm the import path and method names against the library's docs at install time. The signatures below are illustrative; adapt them to whatever the v1/v2 client surface actually exposes. Aim for: dial TCP, optional STARTTLS upgrade or implicit TLS, set TCP keepalive, run SASL.

- [ ] **Step 1: Implement the dial helpers**

Replace `internal/mailimap/auth.go` with:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/mailauth/keepalive"

	imapclient "github.com/emersion/go-imap/v2/imapclient" // adjust path if v1
)

const (
	dialTimeout      = 30 * time.Second
	keepAliveProbes  = 3
	keepAliveInterval = 30 // seconds
)

func dialCommand(cfg config.AccountConfig) (imapClient, error) {
	return dial(cfg, "command")
}

func dialIdle(cfg config.AccountConfig) (imapClient, error) {
	return dial(cfg, "idle")
}

func dial(cfg config.AccountConfig, role string) (imapClient, error) {
	if cfg.Host == "" {
		return nil, errors.New("imap: host is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)

	d := &net.Dialer{Timeout: dialTimeout, KeepAlive: time.Duration(keepAliveInterval) * time.Second}
	var raw net.Conn
	var err error
	if cfg.StartTLS {
		raw, err = d.Dial("tcp", addr)
	} else {
		raw, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: cfg.Host})
	}
	if err != nil {
		return nil, fmt.Errorf("dial %s (%s): %w", addr, role, err)
	}
	if tcp, ok := raw.(*net.TCPConn); ok {
		applyKeepalive(tcp)
	}

	cli := imapclient.New(raw, nil) // fill in options as v2 requires
	if cfg.StartTLS {
		if err := cli.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			_ = raw.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}

	if err := authenticate(cli, cfg); err != nil {
		_ = cli.Logout()
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	return wrapClient(cli), nil
}

func applyKeepalive(c *net.TCPConn) {
	_ = c.SetKeepAlive(true)
	f, err := c.File()
	if err != nil {
		return
	}
	defer f.Close()
	_ = keepalive.SetTcpKeepaliveProbes(int(f.Fd()), keepAliveProbes)
	_ = keepalive.SetTcpKeepaliveInterval(int(f.Fd()), keepAliveInterval)
}

func authenticate(cli *imapclient.Client, cfg config.AccountConfig) error {
	mech := cfg.Auth
	if mech == "" {
		mech = "plain"
	}
	switch mech {
	case "plain":
		return cli.Authenticate(sasl.NewPlainClient("", cfg.Email, cfg.Password))
	case "login":
		return cli.Login(cfg.Email, cfg.Password)
	case "cram-md5":
		return cli.Authenticate(sasl.NewCramMD5Client(cfg.Email, cfg.Password))
	case "xoauth2":
		token := cfg.Password
		if token == "" {
			return errors.New("xoauth2: access token (password field) required for Pass 8 (refresh-from-clientID lands in Pass 8.1)")
		}
		return cli.Authenticate(mailauth.NewXoauth2Client(cfg.Email, token))
	default:
		return fmt.Errorf("unsupported auth mechanism %q", mech)
	}
}

// wrapClient adapts *imapclient.Client to the imapClient interface.
// Implement once the actual library surface is known. For Pass 8 it
// translates each method call into the appropriate go-imap call.
func wrapClient(c *imapclient.Client) imapClient {
	return &realClient{c: c}
}

type realClient struct {
	c *imapclient.Client
}

// Method implementations are filled in alongside each task that
// uses them. Each starts as a stub returning errors.New("not yet
// implemented") to keep the build green.
func (r *realClient) Logout() error                                                       { return r.c.Logout() }
func (r *realClient) Capabilities() (map[string]bool, error)                              { return nil, errors.New("TODO") }
func (r *realClient) List(string, string, bool) ([]listEntry, error)                      { return nil, errors.New("TODO") }
func (r *realClient) Select(string, bool) (mail.Folder, error)                            { return mail.Folder{}, errors.New("TODO") }
func (r *realClient) Search(mail.SearchCriteria) ([]mail.UID, error)                      { return nil, errors.New("TODO") }
func (r *realClient) Fetch([]mail.UID, []string, func(mail.UID, map[string]any)) error    { return errors.New("TODO") }
func (r *realClient) FetchBody(mail.UID) (io.ReadCloser, error)                           { return nil, errors.New("TODO") }
func (r *realClient) Store([]mail.UID, string, any) error                                 { return errors.New("TODO") }
func (r *realClient) Copy([]mail.UID, string) error                                       { return errors.New("TODO") }
func (r *realClient) Move([]mail.UID, string) error                                       { return errors.New("TODO") }
func (r *realClient) UIDExpunge([]mail.UID) error                                         { return errors.New("TODO") }
func (r *realClient) Idle(func(mail.Update)) error                                        { return errors.New("TODO") }
func (r *realClient) IdleStop()                                                           {}
```

(Add `"io"` and `"github.com/glw907/poplar/internal/mail"` imports as needed for the realClient stubs. Comment line 1 of every TODO method with a one-line note pointing back to the task that fills it in.)

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: success. If `imapclient` import path differs in the installed version, fix it here and update the comment in Task 1.

- [ ] **Step 3: Run the existing tests — must still pass**

```bash
go test ./internal/mailimap/...
```

Expected: PASS (tests still use `fakeClient`).

- [ ] **Step 4: Commit**

```bash
git add internal/mailimap/auth.go
git commit -m "Pass 8: dial helpers (TLS/STARTTLS, keepalive, SASL dispatch)"
```

---

### Task 9: ListFolders + OpenFolder

**Files:**
- Create: `internal/mailimap/folders.go`
- Create: `internal/mailimap/folders_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mailimap/folders_test.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestListFoldersWithSpecialUse(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "SPECIAL-USE": true}
	cmd.folders = []listEntry{
		{Name: "INBOX"},
		{Name: "Sent", Attributes: []string{"\\Sent"}},
		{Name: "Trash", Attributes: []string{"\\Trash"}},
		{Name: "Custom"},
	}
	cmd.folderSummary = map[string]mail.Folder{
		"INBOX": {Name: "INBOX", Exists: 12, Unseen: 3},
		"Sent":  {Name: "Sent", Exists: 1},
		"Trash": {Name: "Trash"},
		"Custom":{Name: "Custom"},
	}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	got, err := b.ListFolders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	wantRoles := map[string]string{"INBOX": "", "Sent": "sent", "Trash": "trash", "Custom": ""}
	for _, f := range got {
		if got, want := f.Role, wantRoles[f.Name]; got != want {
			t.Errorf("folder %q role = %q, want %q", f.Name, got, want)
		}
	}
}

func TestOpenFolderTracksCurrent(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient()
	idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.OpenFolder("INBOX"); err != nil {
		t.Fatalf("open: %v", err)
	}
	if cmd.selected != "INBOX" {
		t.Errorf("selected = %q, want INBOX", cmd.selected)
	}
	if b.current != "INBOX" {
		t.Errorf("b.current = %q, want INBOX", b.current)
	}
}
```

- [ ] **Step 2: Verify they fail**

```bash
go test ./internal/mailimap/ -run "TestListFolders|TestOpenFolder"
```

Expected: FAIL — `ListFolders` / `OpenFolder` undefined.

- [ ] **Step 3: Implement**

Create `internal/mailimap/folders.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"fmt"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// ListFolders satisfies mail.Backend. Uses LIST RETURN (SPECIAL-USE)
// when the server advertises it; falls back to plain LIST otherwise.
// The role is derived from RFC 6154 attributes when present; the
// classifier (mail.Classify) handles name-based fallback at the UI
// layer.
func (b *Backend) ListFolders() ([]mail.Folder, error) {
	b.mu.Lock()
	cmd := b.cmd
	useSpecial := b.caps.SpecialUse
	b.mu.Unlock()

	entries, err := cmd.List("", "*", useSpecial)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	out := make([]mail.Folder, 0, len(entries))
	for _, e := range entries {
		f := mail.Folder{Name: e.Name, Role: roleFromAttrs(e.Attributes)}
		// Populate Exists/Unseen via STATUS if needed. For Pass 8
		// initial drop, leave at zero; the UI re-fetches via Select.
		out = append(out, f)
	}
	return out, nil
}

// roleFromAttrs maps RFC 6154 LIST attributes to mail.Folder.Role
// values used by mail.Classify ("inbox", "drafts", "sent", "trash",
// "junk", "archive"). Unknown attributes return "".
func roleFromAttrs(attrs []string) string {
	for _, a := range attrs {
		switch strings.ToLower(strings.TrimPrefix(a, "\\")) {
		case "drafts":
			return "drafts"
		case "sent":
			return "sent"
		case "trash":
			return "trash"
		case "junk":
			return "junk"
		case "archive", "all":
			return "archive"
		case "important", "flagged":
			// Not currently surfaced as a canonical role.
		}
	}
	return ""
}

// OpenFolder satisfies mail.Backend. Selects the folder on the
// command connection and signals the idle goroutine to re-IDLE on
// the new folder.
func (b *Backend) OpenFolder(name string) error {
	b.mu.Lock()
	cmd := b.cmd
	switchCh := b.switchCh
	b.mu.Unlock()

	if _, err := cmd.Select(name, false); err != nil {
		return fmt.Errorf("select %q: %w", name, err)
	}

	b.mu.Lock()
	b.current = name
	b.mu.Unlock()

	if switchCh != nil {
		// Non-blocking — drop earlier pending switches.
		select {
		case <-switchCh:
		default:
		}
		select {
		case switchCh <- name:
		default:
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/mailimap/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/folders.go internal/mailimap/folders_test.go
git commit -m "Pass 8: ListFolders (SPECIAL-USE) + OpenFolder"
```

---

### Task 10: QueryFolder, FetchHeaders, FetchBody, Search

**Files:**
- Create: `internal/mailimap/messages.go`
- Create: `internal/mailimap/messages_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mailimap/messages_test.go` with table-driven tests for each method. Cover:

- `QueryFolder` returning the right offset/limit slice + total.
- `FetchHeaders` populating `MessageInfo` from FETCH results (ENVELOPE, INTERNALDATE, FLAGS).
- `FetchBody` returning the body reader for a UID.
- `Search` translating `mail.SearchCriteria` to UID SEARCH.

(Tests use `fakeClient`. Sketch the cases — the engineer fills them in to match the behavior described in the spec.)

- [ ] **Step 2: Implement `internal/mailimap/messages.go`**

The implementation:

- `QueryFolder(name, offset, limit)` calls `Select` to get total, then UID SEARCH ALL (or the appropriate query) and slices the result newest-first.
- `FetchHeaders(uids)` calls `Fetch` with items = `["UID", "ENVELOPE", "INTERNALDATE", "FLAGS", "BODY.PEEK[HEADER.FIELDS (FROM TO CC BCC SUBJECT DATE IN-REPLY-TO REFERENCES)]"]` and translates each result into `mail.MessageInfo` using a helper `infoFromFetch`.
- `FetchBody(uid)` calls `FetchBody` on the client, returning the reader.
- `Search` translates `SearchCriteria` into UID SEARCH terms (HEADER, BODY, TEXT) and returns the UIDs.

Each method acquires `b.mu` to read `b.cmd`, then runs the client call without the lock held.

- [ ] **Step 3: Run tests to verify they pass**

```bash
go test ./internal/mailimap/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/mailimap/messages.go internal/mailimap/messages_test.go
git commit -m "Pass 8: QueryFolder/FetchHeaders/FetchBody/Search"
```

---

### Task 11: Move/Copy/Delete + flag/mark methods

**Files:**
- Create: `internal/mailimap/actions.go`
- Create: `internal/mailimap/actions_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mailimap/actions_test.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func TestMoveUsesUIDMoveWhenAdvertised(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true, "MOVE": true}
	idle := newFakeClient(); idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Move([]mail.UID{"1", "2"}, "Trash"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(cmd.moveCalls) != 1 {
		t.Fatalf("moveCalls = %d, want 1", len(cmd.moveCalls))
	}
	if len(cmd.copyCalls) != 0 {
		t.Errorf("copyCalls should be 0 with MOVE advertised, got %d", len(cmd.copyCalls))
	}
}

func TestMoveFallsBackToCopyExpungeWithoutMOVE(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true} // no MOVE
	idle := newFakeClient(); idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Move([]mail.UID{"1", "2"}, "Trash"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if len(cmd.copyCalls) != 1 {
		t.Errorf("copyCalls = %d, want 1", len(cmd.copyCalls))
	}
	if len(cmd.storeCalls) != 1 {
		t.Errorf("storeCalls = %d, want 1", len(cmd.storeCalls))
	}
	if len(cmd.expungeCalls) != 1 {
		t.Errorf("expungeCalls = %d, want 1", len(cmd.expungeCalls))
	}
}

func TestDestroyEmptyIsNoOp(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient(); idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Destroy(nil); err != nil {
		t.Errorf("Destroy(nil) = %v, want nil", err)
	}
	if len(cmd.storeCalls) != 0 || len(cmd.expungeCalls) != 0 {
		t.Errorf("Destroy(nil) should not call store/expunge")
	}
}

func TestDestroyStoresDeletedThenExpunges(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = map[string]bool{"IMAP4REV1": true, "UIDPLUS": true}
	idle := newFakeClient(); idle.caps = cmd.caps

	b := newWithFake(config.AccountConfig{Name: "t"}, cmd, idle)
	if err := b.finishConnect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := b.Destroy([]mail.UID{"7", "8"}); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if len(cmd.storeCalls) != 1 || len(cmd.expungeCalls) != 1 {
		t.Errorf("expected one store + one expunge, got %d / %d",
			len(cmd.storeCalls), len(cmd.expungeCalls))
	}
}
```

- [ ] **Step 2: Verify failure**

```bash
go test ./internal/mailimap/ -run "TestMove|TestDestroy"
```

Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement**

Create `internal/mailimap/actions.go`:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"
	"fmt"

	"github.com/glw907/poplar/internal/mail"
)

// Move satisfies mail.Backend. Uses UID MOVE (RFC 6851) when the
// server advertises MOVE; falls back to COPY + STORE \Deleted +
// UID EXPUNGE otherwise. The fallback is a single logical
// operation; partial failure leaves the source folder in a known
// state by surfacing the error before the EXPUNGE fires.
func (b *Backend) Move(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	hasMove := b.caps.MOVE
	b.mu.Unlock()

	if hasMove {
		if err := cmd.Move(uids, dest); err != nil {
			return fmt.Errorf("uid move: %w", err)
		}
		return nil
	}
	if err := cmd.Copy(uids, dest); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("store deleted: %w", err)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("uid expunge: %w", err)
	}
	return nil
}

// Copy satisfies mail.Backend.
func (b *Backend) Copy(uids []mail.UID, dest string) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()
	if err := cmd.Copy(uids, dest); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Delete satisfies mail.Backend. Soft delete to Trash. Resolves the
// trash folder name from ListFolders + Classify; if no trash folder
// is found, returns an error rather than silently EXPUNGEing in place.
func (b *Backend) Delete(uids []mail.UID) error {
	trash, err := b.resolveTrashFolder()
	if err != nil {
		return err
	}
	return b.Move(uids, trash)
}

// resolveTrashFolder finds the folder name with role "trash". Cached
// after first lookup. Returns an error if no trash folder exists.
func (b *Backend) resolveTrashFolder() (string, error) {
	folders, err := b.ListFolders()
	if err != nil {
		return "", fmt.Errorf("list folders: %w", err)
	}
	for _, f := range mail.Classify(folders) {
		if f.Canonical == "Trash" {
			return f.Folder.Name, nil
		}
	}
	return "", errors.New("no Trash folder")
}

// Destroy satisfies mail.Backend. Permanent-delete via STORE \Deleted
// then UID EXPUNGE. Per ADR-0092: empty input is a no-op, the
// operation is irreversible, missing UIDs are treated as success
// (server silently ignores).
func (b *Backend) Destroy(uids []mail.UID) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return fmt.Errorf("store deleted: %w", err)
	}
	if err := cmd.UIDExpunge(uids); err != nil {
		return fmt.Errorf("uid expunge: %w", err)
	}
	return nil
}

// Flag satisfies mail.Backend.
func (b *Backend) Flag(uids []mail.UID, f mail.Flag, set bool) error {
	if len(uids) == 0 {
		return nil
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	op := "+FLAGS.SILENT"
	if !set {
		op = "-FLAGS.SILENT"
	}
	flags := imapFlagsFor(f)
	if len(flags) == 0 {
		return nil
	}
	if err := cmd.Store(uids, op, flags); err != nil {
		return fmt.Errorf("store flags: %w", err)
	}
	return nil
}

// MarkRead/MarkUnread/MarkAnswered are convenience wrappers.
func (b *Backend) MarkRead(uids []mail.UID) error    { return b.Flag(uids, mail.FlagSeen, true) }
func (b *Backend) MarkUnread(uids []mail.UID) error  { return b.Flag(uids, mail.FlagSeen, false) }
func (b *Backend) MarkAnswered(uids []mail.UID) error{ return b.Flag(uids, mail.FlagAnswered, true) }

// Send satisfies mail.Backend. Returns not-implemented until Pass 9.
func (b *Backend) Send(from string, rcpts []string, body interface{}) error {
	return errors.New("send: not implemented (lands in Pass 9)")
}

// imapFlagsFor maps mail.Flag bits to IMAP flag strings.
func imapFlagsFor(f mail.Flag) []string {
	var out []string
	if f&mail.FlagSeen != 0 {
		out = append(out, "\\Seen")
	}
	if f&mail.FlagAnswered != 0 {
		out = append(out, "\\Answered")
	}
	if f&mail.FlagFlagged != 0 {
		out = append(out, "\\Flagged")
	}
	if f&mail.FlagDeleted != 0 {
		out = append(out, "\\Deleted")
	}
	if f&mail.FlagDraft != 0 {
		out = append(out, "\\Draft")
	}
	return out
}
```

(Note: the `Send` signature here uses `interface{}` to avoid pulling in the full `mail.Backend` signature; adjust to `io.Reader` to match `mail.Backend.Send` exactly. Verify with `go build` and fix.)

- [ ] **Step 4: Run tests + build**

```bash
go test ./internal/mailimap/...
go build ./...
```

Expected: PASS, build success.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/actions.go internal/mailimap/actions_test.go
git commit -m "Pass 8: Move (with fallback), Copy, Delete, Destroy, Flag/Mark"
```

---

### Task 12: Idle goroutine — IDLE or polling, refresh, reconnect, folder-switch

**Files:**
- Create: `internal/mailimap/idle.go` (replace stub from Task 7)
- Create: `internal/mailimap/idle_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/mailimap/idle_test.go`. Cover:

- IDLE goroutine emits `ConnConnected` after first idle starts.
- Folder-switch signal causes re-IDLE on new folder.
- IDLE failure triggers reconnect with backoff (mock the dial helpers via a swappable function on the backend).
- Polling fallback runs when IDLE capability absent.

(Sketch — engineer fills in the table per the patterns established. Use `idle.onIdle` and `idle.idleStop` hooks on `fakeClient` to drive flow.)

- [ ] **Step 2: Implement `internal/mailimap/idle.go`**

Replace the stub `idleLoop` from Task 7. The implementation:

```go
// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

const (
	idleRefreshInterval = 9 * time.Minute  // under Gmail's 10-min cap
	pollFallbackInterval = 60 * time.Second
	reconnectInitial    = 1 * time.Second
	reconnectMax        = 30 * time.Second
)

// idleLoop runs until ctx is cancelled. It selects the current
// folder on the idle connection, runs IDLE (or poll fallback),
// honors folder-switch signals from OpenFolder, and reconnects
// with exponential backoff on failure.
func (b *Backend) idleLoop(ctx context.Context) {
	defer close(b.idleDone)

	backoff := reconnectInitial
	for {
		if ctx.Err() != nil {
			return
		}
		err := b.runIdleSession(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnReconnecting})
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > reconnectMax {
				backoff = reconnectMax
			}
			continue
		}
		backoff = reconnectInitial
	}
}

// runIdleSession selects the current folder on the idle connection,
// runs IDLE (or poll), and listens for folder-switch signals. It
// returns nil on clean refresh-cycle completion (caller re-loops),
// or an error on connection failure.
func (b *Backend) runIdleSession(ctx context.Context) error {
	b.mu.Lock()
	idle := b.idle
	current := b.current
	switchCh := b.switchCh
	hasIDLE := b.caps.IDLE
	b.mu.Unlock()

	if current == "" {
		// Wait for the first OpenFolder.
		select {
		case <-ctx.Done():
			return nil
		case f := <-switchCh:
			b.mu.Lock()
			b.current = f
			current = f
			b.mu.Unlock()
		}
	}

	if _, err := idle.Select(current, true); err != nil {
		return err
	}

	b.emit(mail.Update{Type: mail.UpdateConnState, ConnState: mail.ConnConnected})

	if !hasIDLE {
		return b.pollLoop(ctx, current, switchCh)
	}

	timer := time.NewTimer(idleRefreshInterval)
	defer timer.Stop()

	idleErrCh := make(chan error, 1)
	go func() {
		idleErrCh <- idle.Idle(b.handleUnilateral)
	}()

	for {
		select {
		case <-ctx.Done():
			idle.IdleStop()
			<-idleErrCh
			return nil
		case <-timer.C:
			idle.IdleStop()
			if err := <-idleErrCh; err != nil {
				return err
			}
			return nil // re-enter loop, fresh IDLE
		case f := <-switchCh:
			idle.IdleStop()
			if err := <-idleErrCh; err != nil {
				return err
			}
			b.mu.Lock()
			b.current = f
			b.mu.Unlock()
			return nil // re-enter loop with new folder
		case err := <-idleErrCh:
			return err
		}
	}
}

// pollLoop runs when the server lacks IDLE. STATUS every 60s and
// emit UpdateFolderInfo on change. Honors folder-switch signals.
func (b *Backend) pollLoop(ctx context.Context, folder string, switchCh chan string) error {
	t := time.NewTicker(pollFallbackInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case f := <-switchCh:
			b.mu.Lock()
			b.current = f
			b.mu.Unlock()
			return nil
		case <-t.C:
			// Fire UpdateFolderInfo unconditionally; UI re-fetches on receipt.
			b.emit(mail.Update{Type: mail.UpdateFolderInfo, Folder: folder})
		}
	}
}

// handleUnilateral receives unilateral IDLE responses (translated by
// the realClient adapter into mail.Update values) and forwards them.
func (b *Backend) handleUnilateral(u mail.Update) {
	b.emit(u)
}

// emit sends u to the updates channel, dropping if buffer is full.
func (b *Backend) emit(u mail.Update) {
	b.mu.Lock()
	ch := b.updates
	b.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- u:
	default:
	}
}
```

- [ ] **Step 3: Run tests + build**

```bash
go test ./internal/mailimap/...
go build ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/mailimap/idle.go internal/mailimap/idle_test.go
git commit -m "Pass 8: idle goroutine with 9-min refresh + poll fallback + reconnect"
```

---

### Task 13: Wire `mailimap.New` into cmd/poplar dispatch

**Files:**
- Modify: `cmd/poplar/backend.go`

- [ ] **Step 1: Add the dispatch case**

Edit `cmd/poplar/backend.go`:

```go
import (
	// ... existing imports ...
	"github.com/glw907/poplar/internal/mailimap"
)

func openBackend(acct config.AccountConfig) (mail.Backend, error) {
	switch acct.Backend {
	case "mock", "":
		return mail.NewMockBackend(), nil
	case "jmap":
		return mailjmap.New(acct), nil
	case "imap":
		return mailimap.New(acct), nil
	default:
		return nil, fmt.Errorf("unknown backend %q for account %q", acct.Backend, acct.Name)
	}
}
```

(Note: presets `yahoo`/`icloud`/`zoho`/`fastmail` are resolved to canonical `imap`/`jmap` by the config layer in Task 4; they don't need their own cases here.)

- [ ] **Step 2: Verify it compiles + existing tests pass**

```bash
go build ./...
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/poplar/backend.go
git commit -m "Pass 8: dispatch imap accounts to mailimap.New"
```

---

### Task 14: Extend `mail/classify.go` IMAP aliases

**Files:**
- Modify: `internal/mail/classify.go`
- Modify: `internal/mail/classify_test.go`

- [ ] **Step 1: Audit the existing alias table**

Read `internal/mail/classify.go`. The table already covers Outlook ("Sent Items", "Deleted Items"), Yahoo ("Bulk Mail"), Gmail bracketed names. Identify any gaps surfaced by Yahoo/iCloud/Zoho real-server testing:

- Yahoo: "Bulk Mail" (covered), "Trash" (covered)
- iCloud: "Sent Messages" (covered), "Deleted Messages" (covered), "Junk" (covered)
- Zoho: "Sent" (covered), "Trash" (covered), "Spam" (covered)

The table looks complete. If real-server testing in Task 17 surfaces a missing alias, add it here and a corresponding test row.

- [ ] **Step 2: Add a single test that asserts each preset's expected folder names classify correctly**

Append to `internal/mail/classify_test.go`:

```go
func TestClassifyKnownProviderFolders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Yahoo
		{"yahoo bulk", "Bulk Mail", "Spam"},
		// iCloud
		{"icloud sent", "Sent Messages", "Sent"},
		{"icloud trash", "Deleted Messages", "Trash"},
		// Zoho — uses standard names
		{"zoho sent", "Sent", "Sent"},
		// Gmail (covered fully in Pass 8.1; sample here)
		{"gmail trash", "[Gmail]/Trash", "Trash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyOne(Folder{Name: tc.in})
			if got.Canonical != tc.want {
				t.Errorf("Canonical = %q, want %q", got.Canonical, tc.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/mail/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/mail/classify_test.go
git commit -m "Pass 8: assert provider-folder aliases classify correctly"
```

---

### Task 15: Dovecot integration test harness

**Files:**
- Create: `internal/mailimap/integration_test.go`
- Create: `internal/mailimap/README.md`
- Modify: `Makefile`

- [ ] **Step 1: Write the integration test**

Create `internal/mailimap/integration_test.go`:

```go
//go:build integration

// SPDX-License-Identifier: MIT

package mailimap

import (
	"context"
	"os"
	"testing"

	"github.com/glw907/poplar/internal/config"
)

// Integration tests require a running Dovecot instance reachable at
// $POPLAR_TEST_IMAP (default 127.0.0.1:1143) with a test user.
// See README.md for setup instructions.
func TestLiveIMAPLifecycle(t *testing.T) {
	host := os.Getenv("POPLAR_TEST_IMAP_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	user := os.Getenv("POPLAR_TEST_IMAP_USER")
	pass := os.Getenv("POPLAR_TEST_IMAP_PASS")
	if user == "" || pass == "" {
		t.Skip("set POPLAR_TEST_IMAP_USER and POPLAR_TEST_IMAP_PASS")
	}

	cfg := config.AccountConfig{
		Name:     "test",
		Email:    user,
		Host:     host,
		Port:     1143,
		StartTLS: true,
		Auth:     "plain",
		Password: pass,
	}
	b := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer b.Disconnect()

	folders, err := b.ListFolders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	hasInbox := false
	for _, f := range folders {
		if f.Name == "INBOX" {
			hasInbox = true
		}
	}
	if !hasInbox {
		t.Errorf("INBOX not in folders: %v", folders)
	}
}
```

- [ ] **Step 2: Document the test server setup**

Create `internal/mailimap/README.md`:

```markdown
# internal/mailimap

Generic IMAP backend implementing `mail.Backend` over IMAP4rev1
via `emersion/go-imap`. UIDPLUS required; MOVE / SPECIAL-USE / IDLE
used opportunistically. Two physical connections per backend
(command + idle).

See `docs/superpowers/specs/2026-05-01-imap-backend-design.md`.

## Tests

Unit tests use a fake `imapClient` (see `fake_test.go`) and run
under plain `go test ./internal/mailimap/...`.

Integration tests require a live IMAP server. They are guarded by
`//go:build integration` and run via `make test-imap`.

### Local Dovecot setup

```sh
docker run -d --name poplar-dovecot \
  -p 1143:143 \
  -e DOVECOT_USERS="testuser:{plain}testpass:::::" \
  dovecot/dovecot

export POPLAR_TEST_IMAP_HOST=127.0.0.1
export POPLAR_TEST_IMAP_USER=testuser@example.com
export POPLAR_TEST_IMAP_PASS=testpass

make test-imap
```

Tear down: `docker rm -f poplar-dovecot`.

(The `dovecot/dovecot` image's exact env-var contract may vary by
version; consult its README and adjust the command above. Goal:
one IMAP user with a known password reachable on `localhost:1143`
with STARTTLS available.)
```

- [ ] **Step 3: Add Makefile target**

Edit `Makefile`. Add after the existing `test` target:

```makefile
test-imap:
	go test -tags=integration ./internal/mailimap/...
```

- [ ] **Step 4: Verify make targets parse**

```bash
make -n test-imap
```

Expected: prints the `go test` command.

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/integration_test.go internal/mailimap/README.md Makefile
git commit -m "Pass 8: Dovecot integration test harness + docs"
```

---

### Task 16: Fill `realClient` adapter methods against go-imap

**Files:**
- Modify: `internal/mailimap/auth.go` (the `realClient` block)

- [ ] **Step 1: Replace each TODO method with the real go-imap call**

Open `internal/mailimap/auth.go`. For each `realClient` method that returns `errors.New("TODO")`, implement it using the corresponding `emersion/go-imap` API. Use the library's docs (godoc.org or the local `go doc github.com/emersion/go-imap/v2/imapclient`) to find the exact method names and types.

Key translations:

- `Capabilities`: read `c.Caps()` (or the v2 equivalent), build the bool map.
- `List(ref, pattern, specialUse)`: call the `List` command with appropriate options for SPECIAL-USE; iterate the streaming response into `listEntry`.
- `Select`: call `c.Select(folder, ...)`, translate the returned `SelectData` into `mail.Folder` (Name, Exists, Unseen).
- `Search`: build a `SearchCriteria` from `mail.SearchCriteria` and call UID search.
- `Fetch`: call `c.Fetch(uids, items)` and stream messages into `resultFn`.
- `FetchBody`: fetch `BODY[]` for a single UID and return the section reader.
- `Store`: call `c.Store(uids, items)` with the appropriate `StoreFlagsOp`.
- `Copy`: call `c.Copy(uids, dest)`.
- `Move`: call `c.Move(uids, dest)`.
- `UIDExpunge`: call `c.UIDExpunge(uids)`.
- `Idle`: call `c.Idle()` and translate unilateral responses (EXISTS, EXPUNGE, FETCH FLAGS) into `mail.Update` values via the `onUpdate` callback.
- `IdleStop`: cancel the idle handle.

Translation guidance:

- IMAP `EXISTS` increase → `mail.Update{Type: UpdateNewMail}`.
- IMAP `EXPUNGE` → `mail.Update{Type: UpdateExpunge, UIDs: [uid]}`.
- IMAP unilateral `FETCH FLAGS` → `mail.Update{Type: UpdateFlagsChanged, UIDs: [uid]}`.

- [ ] **Step 2: Add a small test that round-trips a Capabilities call against the fake**

(Tests for `realClient` can be deferred to Task 17 / integration; the unit-test surface is `imapClient`. Confirm `realClient` compiles and is referenced from `wrapClient`.)

- [ ] **Step 3: Run all tests + build**

```bash
go test ./...
go build ./...
```

Expected: PASS, build success.

- [ ] **Step 4: Run integration tests if a Dovecot is up**

```bash
make test-imap
```

Expected: PASS (skipped if env vars not set).

- [ ] **Step 5: Commit**

```bash
git add internal/mailimap/auth.go
git commit -m "Pass 8: realClient adapter — go-imap method translations"
```

---

### Task 17: Live verification

**Files:** none (manual)

- [ ] **Step 1: Spin up local Dovecot per the README**

Follow the steps in `internal/mailimap/README.md`. Verify the integration test passes.

- [ ] **Step 2: Configure a real account in `~/.config/poplar/accounts.toml`**

For example, an iCloud account using an app-specific password:

```toml
[[account]]
name     = "icloud"
backend  = "icloud"
email    = "you@icloud.com"
auth     = "plain"
password = "$ICLOUD_APP_PASSWORD"
```

- [ ] **Step 3: Launch poplar and walk the smoke test**

```bash
export ICLOUD_APP_PASSWORD="..."  # from 1Password
poplar
```

Verify in order:

- Sidebar populates with Inbox / Sent / Trash / Drafts / etc. classified correctly.
- Inbox shows recent messages with correct From / Subject / Date columns.
- Open a message — body renders, mark-read fires.
- Move a message to Archive (`a`), then `u` undo — message returns.
- Move a message to Trash (`d`), let toast expire — confirm gone from Inbox, present in Trash.
- Open Trash, `E` to empty (modal confirm) — Trash empties.

- [ ] **Step 4: Note any observed issues**

If anything is broken, capture details and fix before proceeding to Pass 8.1. If everything works, proceed to consolidation.

- [ ] **Step 5: No commit**

(Manual verification, no code change.)

---

## Pass-end ritual

After Task 17 passes, invoke the `poplar-pass` skill to run the
end-of-pass consolidation:

1. `/simplify` — fix anything it flags.
2. (No `internal/ui/` changes — skip the bubbletea checklist.)
3. Write ADRs in `docs/poplar/decisions/`:
   - One for the provider registry pattern.
   - One for the two-connection IMAP model + 9-min IDLE refresh.
   - One for the IMAP `Destroy` mapping (extends ADR-0092).
   - One for the v1 backend roster expansion.
4. Update `docs/poplar/invariants.md`: backend roster expanded from
   "Fastmail JMAP + Gmail IMAP" to the four-way taxonomy.
5. Update `docs/poplar/STATUS.md`: mark Pass 8 done, write Pass 8.1
   starter prompt (Gmail support).
6. Archive `docs/superpowers/plans/2026-05-02-imap-backend.md` and
   `docs/superpowers/specs/2026-05-01-imap-backend-design.md` to
   their `archive/` subdirectories with `git mv`.
7. `make check`, commit, push, `make install`.

---

## Open risks tracked during execution

- `emersion/go-imap` v1 vs v2 method-name differences. Resolve in
  Task 1 (decide which version) and Task 8 / Task 16 (adapter).
- Dovecot's default capabilities — if SPECIAL-USE isn't enabled,
  Task 9 unit tests pass but Task 17 live test fails on
  classification. Document the dovecot.conf tweak in the README if
  needed.
- `cli.Authenticate` may want a different signature than shown for
  the chosen go-imap version. Adapt in Task 8.
