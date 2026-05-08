# Pass 9m — CardDAV ingest + autocomplete

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `contacts.FixtureSuggestions` and the
fixture-backed `LookupByEmail` with a CardDAV-driven contacts
cache feeding the compose autocomplete seam (`SuggestFn`) and the
`i`-popover sender lookup. Read-only this pass; form write-back
splits into Pass 9m.1.

**Architecture:** New `internal/contacts/` (UI-free) wraps
`emersion/go-webdav/carddav` for sync and `emersion/go-vcard` for
parse. New cache schema v8 adds five tables: `addressbooks`,
`contacts`, `contact_emails`, `contact_phones`,
`message_recipients`. Ranking SQL runs a recency-decayed score over
recipients pulled from existing message rows, joined to the carded
pool. App swaps function pointers; `compose.SuggestFn` signature is
unchanged (ADR-0174).

**Tech Stack:** Go 1.26, `modernc.org/sqlite`,
`emersion/go-webdav v0.6+`, `emersion/go-vcard v0.0.0-...`,
`nyaruka/phonenumbers`, bubbletea v1.x.

**Spec:** `docs/superpowers/specs/2026-05-07-carddav-ingest-design.md`

---

## File map

**Created**

- `internal/contacts/types.go` — value types moved from
  `internal/ui/contacts/types.go`: `Kind`, `Contact`, `Email`,
  `Phone`, `Suggestion`, `AddressBook`.
- `internal/contacts/client.go` — CardDAV HTTP client wrapping
  `go-webdav/carddav` for discovery, multiget, and the
  `sync-collection` REPORT.
- `internal/contacts/vcard.go` — vCard ↔ projection mapping.
- `internal/contacts/sync.go` — orchestrator: discover books, pick
  sync strategy per book, apply changeset, persist.
- `internal/contacts/sync_test.go` — `httptest`-backed CardDAV
  server fixtures.
- `internal/contacts/vcard_test.go` — table-driven parse tests.
- `internal/cache/contacts.go` — `SyncContacts`,
  `SuggestAddresses`, `LookupContact`, recipient projection helper.
- `internal/cache/contacts_test.go` — schema-migration + ranking
  unit tests.

**Modified**

- `internal/cache/schema.go` — add `migrateV8`, bump
  `schemaVersion` to 8, append to `migrations`.
- `internal/cache/syncer.go` — populate `message_recipients` in the
  same transaction as `messages` upserts.
- `internal/config/config.go` (or wherever `AccountConfig` lives) —
  add `ContactsConfig` field + decode + validation.
- `internal/ui/contacts/types.go` — replaced with re-imports from
  `internal/contacts`; UI sub-models keep using `contacts.Contact`
  etc. without source changes.
- `internal/ui/contacts/fixtures.go` — keep `Fixtures()` for tests;
  remove App callers.
- `internal/ui/contacts/form.go` — phone validation via
  `phonenumbers.Parse`.
- `internal/ui/app.go` — swap `contacts.FixtureSuggestions` and
  `contacts.LookupByEmail(contacts.Fixtures(),...)` for cache
  methods; install 15-min sync ticker.
- `go.mod` / `go.sum` — `emersion/go-webdav`, `emersion/go-vcard`,
  `nyaruka/phonenumbers`.

---

### Task 1: Move value types from `internal/ui/contacts/` to `internal/contacts/`

The cache will return `[]contacts.Suggestion`. Cache cannot import
`internal/ui/`, so the value types move to a UI-free package. UI
sub-models continue referring to `contacts.Contact` etc. — only
the import path changes.

**Files:**
- Create: `internal/contacts/types.go`
- Delete content (replace with new package decl): none — see step 4
- Modify: `internal/ui/contacts/detail.go`, `form.go`, `list.go`,
  `popover.go`, `sidebar.go`, `fixtures.go`, plus their `_test.go`
  files — each gets an alias import.

- [ ] **Step 1: Create the new package with the value types**

Write `internal/contacts/types.go`:

```go
// Package contacts holds poplar's plain-value contact types and
// the CardDAV ingest path. UI surfaces live in internal/ui/contacts;
// they import these types directly.
package contacts

// Kind distinguishes a person card from an organization card.
type Kind int

const (
	KindPerson Kind = iota
	KindOrg
)

// Contact is the value rendered by every UI surface. Storage
// columns (uid, href, etag, vcard blob) live in the cache layer
// and are not part of this projection.
type Contact struct {
	Kind   Kind
	Name   string
	Family string
	Given  string
	Org    string
	Title  string
	Note   string
	Emails []Email
	Phones []Phone
}

// Email pairs an address with an optional label. Index 0 is the
// primary; the form reorders the slice on change.
type Email struct {
	Address string
	Label   string
}

// Phone pairs a number with an optional label. Stored as the user
// typed it; phonenumbers parsing only validates.
type Phone struct {
	E164  string
	Label string
}

// Suggestion is one row in the compose autocomplete dropdown.
type Suggestion struct {
	Name  string
	Email string
	Org   string
	IsOrg bool
}

// AddressBook is one CardDAV collection.
type AddressBook struct {
	Href        string
	DisplayName string
	Description string
}
```

- [ ] **Step 2: Delete the duplicated types from `internal/ui/contacts/types.go`**

Replace the entire file contents with:

```go
// Package contacts provides poplar's address-book UI surfaces:
// the i-popover, Contacts mode, and the contact edit form. The
// plain-value types (Contact, Email, Phone, Kind, Suggestion,
// AddressBook) live in github.com/glw907/poplar/internal/contacts;
// callers in this package import the same name from there.
package contacts

import core "github.com/glw907/poplar/internal/contacts"

// Re-exports keep existing call sites compiling. No behavior here.
type (
	Kind        = core.Kind
	Contact     = core.Contact
	Email       = core.Email
	Phone       = core.Phone
	Suggestion  = core.Suggestion
	AddressBook = core.AddressBook
)

const (
	KindPerson = core.KindPerson
	KindOrg    = core.KindOrg
)
```

This is the only place where a type alias is permitted in the
codebase — it bridges two packages with the same name during the
ownership move. Once Pass 9m.1 finishes the form save flow, the
alias file may stay or be inlined; it is not load-bearing.

- [ ] **Step 3: Verify compile**

Run: `go build ./...`
Expected: clean build. Both `contacts.Contact` (UI package) and
`contacts.Contact` (core package) resolve to the same type.

- [ ] **Step 4: Run existing tests**

Run: `go test ./internal/ui/contacts/... ./...`
Expected: all green. No call-site changes required.

- [ ] **Step 5: Commit**

```bash
git add internal/contacts/types.go internal/ui/contacts/types.go
git commit -m "Pass 9m.1: move contact value types to internal/contacts

$(cat <<'EOF'
Cache layer needs to return Suggestion/Contact without depending
on internal/ui/. Move the plain-value types to a new UI-free
package; UI package keeps a thin alias file so call sites are
unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: `[[account.contacts]]` config block

Add `ContactsConfig` to `AccountConfig` with TOML decode and
validation. JMAP-backed accounts may carry the block independently
(Fastmail JMAP for mail + Fastmail CardDAV for contacts is the
canonical case). Mirrors the IMAP/JMAP credential resolution
shape.

**Files:**
- Modify: `internal/config/config.go` (add type + decode hook)
- Modify: `internal/config/template.go` if a TOML template stub
  exists; mirror existing `[[account]]` style.
- Modify: `internal/config/config_test.go` — table-driven decode +
  validation cases.

- [ ] **Step 1: Write the failing decode test**

Add to `internal/config/config_test.go`:

```go
func TestAccountContactsDecode(t *testing.T) {
	const src = `
[[account]]
name        = "personal"
provider    = "fastmail"
password    = "stub"

[account.contacts]
url                 = "https://carddav.fastmail.com/dav/addressbooks/user/me/"
username            = "me@example.com"
password-cmd        = "cmd"
default-addressbook = "Default"
refresh-interval    = "10m"
`
	cfg, err := DecodeString(src)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := cfg.Accounts[0].Contacts
	if got == nil {
		t.Fatal("Contacts nil")
	}
	if got.URL != "https://carddav.fastmail.com/dav/addressbooks/user/me/" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.DefaultAddressbook != "Default" {
		t.Errorf("DefaultAddressbook = %q", got.DefaultAddressbook)
	}
	if got.RefreshInterval != 10*time.Minute {
		t.Errorf("RefreshInterval = %v", got.RefreshInterval)
	}
}
```

(`DecodeString` is the existing test helper; if it doesn't exist
under that name, find the equivalent in `config_test.go` and
mirror it.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run TestAccountContactsDecode`
Expected: FAIL — no `Contacts` field on `AccountConfig`.

- [ ] **Step 3: Add `ContactsConfig`**

In `internal/config/config.go` (or the file that defines
`AccountConfig`):

```go
// ContactsConfig configures CardDAV ingest for one account. Optional
// — accounts without a [account.contacts] block skip contact sync.
type ContactsConfig struct {
	URL                string        `toml:"url"`
	Username           string        `toml:"username"`
	Password           string        `toml:"password"`
	PasswordCmd        string        `toml:"password-cmd"`
	DefaultAddressbook string        `toml:"default-addressbook"`
	RefreshInterval    time.Duration `toml:"refresh-interval"`
	InsecureTLS        bool          `toml:"insecure-tls"`
}
```

Add to `AccountConfig`:

```go
type AccountConfig struct {
	// ... existing fields ...
	Contacts *ContactsConfig `toml:"contacts"`
}
```

Note the pointer — absence is a real distinct state, not the same
as a zero-value block.

- [ ] **Step 4: Re-run, expect pass**

Run: `go test ./internal/config/ -run TestAccountContactsDecode`
Expected: PASS.

- [ ] **Step 5: Add validation test**

```go
func TestContactsConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		c    ContactsConfig
		want string // substring of error; "" = ok
	}{
		{"empty url", ContactsConfig{}, "url"},
		{"non-https", ContactsConfig{URL: "ftp://x"}, "url"},
		{"http allowed when insecure-tls", ContactsConfig{URL: "http://x.local/", InsecureTLS: true}, ""},
		{"refresh too small", ContactsConfig{URL: "https://x/", RefreshInterval: 30 * time.Second}, "refresh-interval"},
		{"both password and cmd", ContactsConfig{URL: "https://x/", Password: "a", PasswordCmd: "b"}, "password"},
		{"defaults", ContactsConfig{URL: "https://x/"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.validate()
			if tc.want == "" && err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %q; got %v", tc.want, err)
			}
		})
	}
}
```

- [ ] **Step 6: Implement `validate`**

```go
func (c *ContactsConfig) validate() error {
	if c == nil {
		return nil
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("contacts: url: not parseable")
	}
	switch u.Scheme {
	case "https":
		// fine
	case "http":
		if !c.InsecureTLS {
			return fmt.Errorf("contacts: url: http requires insecure-tls = true")
		}
	default:
		return fmt.Errorf("contacts: url: scheme must be https (or http with insecure-tls)")
	}
	if c.Password != "" && c.PasswordCmd != "" {
		return fmt.Errorf("contacts: set password OR password-cmd, not both")
	}
	if c.RefreshInterval == 0 {
		c.RefreshInterval = 15 * time.Minute
	} else if c.RefreshInterval < time.Minute {
		return fmt.Errorf("contacts: refresh-interval must be ≥ 1m")
	}
	return nil
}
```

Wire it from the existing `AccountConfig.validate` (or equivalent
top-level validation function) so it fires on `Load`. The default-
addressbook existence check happens at sync time, not load time.

- [ ] **Step 7: Credential fallback**

When `Username`/`Password`/`PasswordCmd` are unset in the
contacts block, mirror the parent `[[account]]` credentials. Add
to `validate` or a post-decode hook:

```go
func (a *AccountConfig) finalizeContacts() {
	if a.Contacts == nil {
		return
	}
	if a.Contacts.Username == "" {
		a.Contacts.Username = a.Username
	}
	if a.Contacts.Password == "" && a.Contacts.PasswordCmd == "" {
		a.Contacts.Password = a.Password
		a.Contacts.PasswordCmd = a.PasswordCmd
	}
}
```

Call from the top-level `Load` after each account validates.
Document in the existing config template comment block.

- [ ] **Step 8: Run all config tests**

Run: `go test ./internal/config/...`
Expected: green.

- [ ] **Step 9: Commit**

```bash
git add internal/config/
git commit -m "Pass 9m.2: [[account.contacts]] config block

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: vCard parser (`internal/contacts/vcard.go`)

Map vCards to the projection. Skip groups (`KIND:group`). Preserve
the raw bytes alongside the projection — the cache stores both.

**Files:**
- Create: `internal/contacts/vcard.go`
- Create: `internal/contacts/vcard_test.go`
- Modify: `go.mod` (add `github.com/emersion/go-vcard`)

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/emersion/go-vcard@latest`
Then: `go mod tidy`

- [ ] **Step 2: Write the failing test**

Write `internal/contacts/vcard_test.go`:

```go
package contacts

import (
	"strings"
	"testing"
)

func TestParseVCard_Person(t *testing.T) {
	src := `BEGIN:VCARD
VERSION:3.0
UID:abc-123
REV:20260101T120000Z
FN:Geoff Wright
N:Wright;Geoff;;;
EMAIL;TYPE=PREF:geoff@907.life
EMAIL:work@example.com
TEL;TYPE=CELL:+15555550100
ORG:907 Life
TITLE:Captain
NOTE:Test note
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got.Skip {
		t.Fatal("person card flagged Skip")
	}
	if got.UID != "abc-123" {
		t.Errorf("UID = %q", got.UID)
	}
	if got.Contact.Kind != KindPerson || got.Contact.Name != "Geoff Wright" {
		t.Errorf("kind/name = %v %q", got.Contact.Kind, got.Contact.Name)
	}
	if len(got.Contact.Emails) != 2 || got.Contact.Emails[0].Address != "geoff@907.life" {
		t.Errorf("emails = %+v (PREF should sort first)", got.Contact.Emails)
	}
	if len(got.Contact.Phones) != 1 || got.Contact.Phones[0].E164 != "+15555550100" {
		t.Errorf("phones = %+v", got.Contact.Phones)
	}
	if got.Rev != "20260101T120000Z" {
		t.Errorf("Rev = %q", got.Rev)
	}
}

func TestParseVCard_Group_Skipped(t *testing.T) {
	src := `BEGIN:VCARD
VERSION:4.0
UID:g1
KIND:group
FN:Team
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Skip {
		t.Fatal("group should be flagged Skip")
	}
}

func TestParseVCard_Org(t *testing.T) {
	src := `BEGIN:VCARD
VERSION:3.0
UID:o1
FN:Acme Co.
ORG:Acme Co.
EMAIL:hello@acme.example
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got.Contact.Kind != KindOrg {
		t.Errorf("Kind = %v; want KindOrg", got.Contact.Kind)
	}
	if got.Contact.Org != "Acme Co." || got.Contact.Name != "Acme Co." {
		t.Errorf("name/org = %q %q", got.Contact.Name, got.Contact.Org)
	}
}

func TestParseVCard_PrefSemantics_v4(t *testing.T) {
	// vCard 4.0: PREF=1..100, lower wins; vCard 3.0: TYPE=PREF.
	src := `BEGIN:VCARD
VERSION:4.0
UID:v4
FN:Test
EMAIL;PREF=2:second@x
EMAIL;PREF=1:first@x
END:VCARD
`
	got, err := ParseVCard(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if got.Contact.Emails[0].Address != "first@x" {
		t.Errorf("PREF=1 should sort first; got %+v", got.Contact.Emails)
	}
}
```

- [ ] **Step 3: Run, expect FAIL (function undefined)**

Run: `go test ./internal/contacts/ -run TestParseVCard`
Expected: FAIL — `ParseVCard` undefined.

- [ ] **Step 4: Implement `ParseVCard`**

Write `internal/contacts/vcard.go`:

```go
package contacts

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/emersion/go-vcard"
)

// Parsed is one vCard mapped to the projection plus metadata for
// the cache. Skip is true when the card is a vCard group; the
// caller drops it from the ingest path.
type Parsed struct {
	UID     string
	Rev     string
	Raw     []byte // round-trip bytes, stored as-is
	Contact Contact
	Skip    bool
}

// ParseVCard reads one vCard from r and returns its projection. It
// reads the entire stream into memory for round-tripping.
func ParseVCard(r io.Reader) (Parsed, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return Parsed{}, fmt.Errorf("read: %w", err)
	}
	dec := vcard.NewDecoder(bytes.NewReader(buf))
	card, err := dec.Decode()
	if err != nil {
		return Parsed{}, fmt.Errorf("decode: %w", err)
	}
	out := Parsed{
		UID: card.Value(vcard.FieldUID),
		Rev: card.Value(vcard.FieldRevision),
		Raw: buf,
	}
	if strings.EqualFold(card.Value(vcard.FieldKind), "group") {
		out.Skip = true
		return out, nil
	}
	out.Contact = mapContact(card)
	return out, nil
}

func mapContact(card vcard.Card) Contact {
	c := Contact{
		Name:  card.PreferredValue(vcard.FieldFormattedName),
		Org:   card.PreferredValue(vcard.FieldOrganization),
		Title: card.PreferredValue(vcard.FieldTitle),
		Note:  card.PreferredValue(vcard.FieldNote),
	}
	if name := card.Name(); name != nil {
		c.Family = name.FamilyName
		c.Given = name.GivenName
	}
	// Heuristic: card with ORG and no N component → KindOrg.
	if c.Org != "" && c.Family == "" && c.Given == "" {
		c.Kind = KindOrg
	}
	c.Emails = collectEmails(card)
	c.Phones = collectPhones(card)
	return c
}

type sortedField struct {
	value string
	label string
	pref  int // 1..100; 0 = unset
}

// fieldsSorted pulls a multi-valued field, normalizes PREF
// (3.0 TYPE=PREF → 1, missing → 0) and 4.0 numeric PREF, then
// sorts by pref ascending (0 last) so primaries land at index 0.
func fieldsSorted(card vcard.Card, key string) []sortedField {
	out := []sortedField{}
	for _, f := range card[key] {
		s := sortedField{value: f.Value}
		if p := f.Params.Get(vcard.ParamPreferred); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				s.pref = n
			} else {
				s.pref = 1 // PREF without value (3.0 TYPE=PREF)
			}
		} else if hasType(f.Params, "pref") {
			s.pref = 1
		}
		s.label = primaryLabel(f.Params)
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ip, jp := out[i].pref, out[j].pref
		if ip == 0 {
			ip = 1<<31 - 1
		}
		if jp == 0 {
			jp = 1<<31 - 1
		}
		return ip < jp
	})
	return out
}

func collectEmails(card vcard.Card) []Email {
	rows := fieldsSorted(card, vcard.FieldEmail)
	out := make([]Email, 0, len(rows))
	for _, r := range rows {
		out = append(out, Email{Address: r.value, Label: r.label})
	}
	return out
}

func collectPhones(card vcard.Card) []Phone {
	rows := fieldsSorted(card, vcard.FieldTelephone)
	out := make([]Phone, 0, len(rows))
	for _, r := range rows {
		out = append(out, Phone{E164: r.value, Label: r.label})
	}
	return out
}

func hasType(p vcard.Params, want string) bool {
	for _, t := range p[vcard.ParamType] {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

func primaryLabel(p vcard.Params) string {
	for _, t := range p[vcard.ParamType] {
		switch strings.ToLower(t) {
		case "home", "work", "cell", "mobile", "fax":
			return strings.ToLower(t)
		}
	}
	return ""
}
```

If `go-vcard`'s API differs (method names, param-key constants),
adjust at this step — the contract above is what the rest of the
plan relies on, not the upstream surface. Verify with
`go doc github.com/emersion/go-vcard` after the import lands.

- [ ] **Step 5: Run, expect PASS**

Run: `go test ./internal/contacts/ -run TestParseVCard -v`
Expected: all four sub-tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/contacts/vcard.go internal/contacts/vcard_test.go go.mod go.sum
git commit -m "Pass 9m.3: vCard parser

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: CardDAV client (`internal/contacts/client.go`)

Thin wrapper around `go-webdav/carddav`. Five public methods:
`DiscoverHomeSet`, `ListAddressBooks`, `Multiget`, `SyncCollection`,
`PropfindCTAG`. Each returns its raw shape; orchestration is in
`sync.go`.

**Files:**
- Create: `internal/contacts/client.go`
- Modify: `go.mod` (add `github.com/emersion/go-webdav`)

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/emersion/go-webdav@latest`
Then: `go mod tidy`

- [ ] **Step 2: Verify the upstream surface**

Run: `go doc github.com/emersion/go-webdav/carddav`
Capture the exact `Client` constructor + `AddressBook`,
`AddressObject`, `SyncQuery`, `SyncResponse` types.

This task ships a wrapper — the wrapper's contract is the
contract this plan relies on, regardless of upstream renames.

- [ ] **Step 3: Implement the client**

Write `internal/contacts/client.go`:

```go
package contacts

import (
	"context"
	"fmt"
	"net/http"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
)

// Client is poplar's CardDAV face. It wraps go-webdav so the rest
// of the package depends on a small, stable surface.
type Client struct {
	cl   *carddav.Client
	base string // home-set URL when known
}

// NewClient builds a CardDAV client for the given URL with HTTP
// Basic auth. insecureTLS skips certificate verification (used
// for self-hosted servers with self-signed certs).
func NewClient(serverURL, username, password string, insecureTLS bool) (*Client, error) {
	httpClient := webdav.HTTPClientWithBasicAuth(http.DefaultClient, username, password)
	if insecureTLS {
		// shallow copy and replace transport — keep go-webdav's
		// auth wrapper intact.
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = insecureTLSConfig()
		httpClient = webdav.HTTPClientWithBasicAuth(&http.Client{Transport: tr}, username, password)
	}
	cl, err := carddav.NewClient(httpClient, serverURL)
	if err != nil {
		return nil, fmt.Errorf("carddav: new client: %w", err)
	}
	return &Client{cl: cl, base: serverURL}, nil
}

// HomeSet resolves the principal's addressbook-home-set. Falls back
// to the configured URL when discovery returns nothing (some self-
// hosted servers expect a direct URL).
func (c *Client) HomeSet(ctx context.Context) (string, error) {
	principal, err := c.cl.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return c.base, nil // fall back; not all servers expose principal
	}
	home, err := c.cl.FindAddressBookHomeSet(ctx, principal)
	if err != nil || home == "" {
		return c.base, nil
	}
	return home, nil
}

// AddressBooks lists all collections under the home set.
func (c *Client) AddressBooks(ctx context.Context, homeSet string) ([]carddav.AddressBook, error) {
	return c.cl.FindAddressBooks(ctx, homeSet)
}

// SyncQuery is a typed re-export so callers don't import go-webdav.
type SyncQuery = carddav.SyncQuery
type SyncResponse = carddav.SyncResponse

// SyncCollection runs a sync-collection REPORT. An empty token in
// q means "give me everything from now on" — the server returns the
// new token without enumerating existing rows. The first sync
// passes a non-empty marker via Multiget instead.
func (c *Client) SyncCollection(ctx context.Context, bookHref string, q *SyncQuery) (*SyncResponse, error) {
	return c.cl.SyncCollection(ctx, bookHref, q)
}

// CTAG fetches the collection's getctag value. Returns "" when the
// server does not advertise the property.
func (c *Client) CTAG(ctx context.Context, bookHref string) (string, error) {
	// Implementation: PROPFIND depth=0 for cs:getctag. go-webdav
	// exposes a generic Propfind helper; if the carddav package
	// doesn't, fall through to webdav.Client.Propfind directly.
	props, err := c.cl.QueryAddressBook(ctx, bookHref, &carddav.AddressBookQuery{ /* CTAG-only */ })
	_ = props
	_ = err
	// TODO at implementation time: replace with the actual
	// PROPFIND surface go-webdav exposes; the function shape is
	// frozen, the body is not.
	return "", nil
}

// Multiget fetches the named hrefs as full vCards. Used both for
// the initial baseline pull and for sync-collection's "added"
// entries when the server returned hrefs without bodies.
func (c *Client) Multiget(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error) {
	return c.cl.MultiGetAddressBook(ctx, bookHref, hrefs, nil)
}
```

Note the deliberate `TODO at implementation time` comment in
`CTAG` — the exact PROPFIND surface depends on the go-webdav
version pinned. The wrapper boundary insulates the rest of the
plan; finalize the body when implementing.

- [ ] **Step 4: Helper: insecure TLS config**

Add to the same file:

```go
func insecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true}
}
```

(Import `crypto/tls`.)

- [ ] **Step 5: Verify it builds**

Run: `go build ./internal/contacts/...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/contacts/client.go go.mod go.sum
git commit -m "Pass 9m.4: CardDAV client wrapper

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Schema v8 migration

Five new tables + backfill of `message_recipients` from existing
`messages` rows. Single transactional migration.

**Files:**
- Modify: `internal/cache/schema.go`
- Create: `internal/cache/contacts_test.go` (test stub for the
  migration; expanded in Task 7)

- [ ] **Step 1: Write the failing migration test**

Add `internal/cache/contacts_test.go`:

```go
package cache

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateV8_TablesExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := openWithMigrations(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	want := []string{
		"addressbooks", "contacts", "contact_emails",
		"contact_phones", "message_recipients",
	}
	for _, name := range want {
		var got string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
			name,
		).Scan(&got)
		if err == sql.ErrNoRows {
			t.Errorf("table %s missing after migrateV8", name)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}
```

(`openWithMigrations` is the existing test helper; mirror whatever
`cache_test.go` already uses.)

- [ ] **Step 2: Run, expect FAIL (tables missing)**

Run: `go test ./internal/cache/ -run TestMigrateV8`
Expected: FAIL — schemaVersion is 7, no tables created.

- [ ] **Step 3: Add `migrateV8`**

Append to `internal/cache/schema.go`:

```go
// migrateV8 adds the contacts cache: addressbook collections,
// contacts (with full vCard blob and a normalized projection),
// child email/phone tables, and message_recipients (the recency
// projection used by SuggestAddresses ranking). Backfills
// message_recipients from existing messages rows in the same
// transaction.
func migrateV8(tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE addressbooks (
			href           TEXT PRIMARY KEY,
			display_name   TEXT NOT NULL,
			description    TEXT NOT NULL DEFAULT '',
			sync_token     TEXT NOT NULL DEFAULT '',
			ctag           TEXT NOT NULL DEFAULT '',
			supports_sync  INTEGER NOT NULL DEFAULT 0,
			last_synced_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE contacts (
			uid              TEXT PRIMARY KEY,
			addressbook_href TEXT NOT NULL REFERENCES addressbooks(href) ON DELETE CASCADE,
			href             TEXT NOT NULL UNIQUE,
			etag             TEXT NOT NULL,
			vcard            BLOB NOT NULL,
			rev              TEXT NOT NULL DEFAULT '',
			fn               TEXT NOT NULL,
			family           TEXT NOT NULL DEFAULT '',
			given            TEXT NOT NULL DEFAULT '',
			org              TEXT NOT NULL DEFAULT '',
			title            TEXT NOT NULL DEFAULT '',
			note             TEXT NOT NULL DEFAULT '',
			last_synced_at   INTEGER NOT NULL
		)`,
		`CREATE INDEX contacts_by_book ON contacts(addressbook_href)`,
		`CREATE TABLE contact_emails (
			contact_uid TEXT NOT NULL REFERENCES contacts(uid) ON DELETE CASCADE,
			address     TEXT NOT NULL,
			label       TEXT NOT NULL DEFAULT '',
			pref        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (contact_uid, address)
		)`,
		`CREATE INDEX contact_emails_by_addr ON contact_emails(address COLLATE NOCASE)`,
		`CREATE TABLE contact_phones (
			contact_uid TEXT NOT NULL REFERENCES contacts(uid) ON DELETE CASCADE,
			number      TEXT NOT NULL,
			label       TEXT NOT NULL DEFAULT '',
			pref        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (contact_uid, number)
		)`,
		`CREATE TABLE message_recipients (
			message_uid INTEGER NOT NULL,
			role        TEXT NOT NULL CHECK (role IN ('from','to','cc')),
			address     TEXT NOT NULL,
			name        TEXT NOT NULL DEFAULT '',
			sent_at     INTEGER NOT NULL,
			PRIMARY KEY (message_uid, role, address)
		)`,
		`CREATE INDEX message_recipients_by_addr_sent ON message_recipients(address COLLATE NOCASE, sent_at DESC)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrateV8: %w", err)
		}
	}
	return backfillRecipients(tx)
}

// backfillRecipients re-parses from_addr/to_addr/cc_addr on existing
// messages rows into message_recipients. Runs once during the
// v7→v8 migration; idempotent on retry (PRIMARY KEY collision).
func backfillRecipients(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT uid, sent_at, from_addr, to_addr, cc_addr FROM messages`)
	if err != nil {
		return fmt.Errorf("backfill: scan: %w", err)
	}
	defer rows.Close()

	insert, err := tx.Prepare(
		`INSERT OR IGNORE INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("backfill: prepare: %w", err)
	}
	defer insert.Close()

	for rows.Next() {
		var uid, sentAt int64
		var from, to, cc sql.NullString
		if err := rows.Scan(&uid, &sentAt, &from, &to, &cc); err != nil {
			return fmt.Errorf("backfill: row: %w", err)
		}
		writeRoleAddrs(insert, uid, sentAt, "from", from.String)
		writeRoleAddrs(insert, uid, sentAt, "to", to.String)
		writeRoleAddrs(insert, uid, sentAt, "cc", cc.String)
	}
	return rows.Err()
}

func writeRoleAddrs(stmt *sql.Stmt, uid, sentAt int64, role, raw string) {
	if raw == "" {
		return
	}
	addrs, err := mail.ParseAddressList(raw)
	if err != nil {
		return // malformed legacy row; skip
	}
	for _, a := range addrs {
		_, _ = stmt.Exec(uid, role, strings.ToLower(a.Address), a.Name, sentAt)
	}
}
```

(Add imports: `net/mail`, `strings`, `fmt`.)

- [ ] **Step 4: Bump version + register migration**

Update the migration list:

```go
const schemaVersion = 8

var migrations = []migration{
	migrateV1,
	migrateV2,
	migrateV3,
	migrateV4,
	migrateV5,
	migrateV6,
	migrateV7,
	migrateV8, // v7 → v8: contacts cache + recipient projection
}
```

- [ ] **Step 5: Run, expect PASS**

Run: `go test ./internal/cache/ -run TestMigrateV8 -v`
Expected: PASS.

- [ ] **Step 6: Add a backfill round-trip test**

Append to `contacts_test.go`:

```go
func TestMigrateV8_BackfillRecipients(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open at v7 first so we can seed messages, then trigger v8.
	// Easier: use the helper that opens a fresh DB and inject one
	// message before running the v8 migration manually.
	// (Adapt to whatever cache_test.go's primitives provide.)
	db, err := openAtVersion(dbPath, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO messages(uid, sent_at, from_addr, to_addr, cc_addr) VALUES (?, ?, ?, ?, ?)`,
		int64(1), int64(1700000000),
		`Alice <alice@example.com>`,
		`Bob <bob@example.com>, carol@example.com`,
		``,
	); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Re-open: triggers v7→v8 with backfill.
	db, err = openWithMigrations(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_recipients WHERE message_uid=1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("backfilled rows = %d; want 3 (1 from, 2 to)", n)
	}
}
```

If `openAtVersion` doesn't exist as a helper, add it to
`cache_test.go` or inline its body — open a DB, run migrations
1..7 only, return the handle.

- [ ] **Step 7: Run + commit**

Run: `go test ./internal/cache/ -run TestMigrateV8 -v`
Expected: both sub-tests PASS.

```bash
git add internal/cache/schema.go internal/cache/contacts_test.go
git commit -m "Pass 9m.5: schema v8 — contacts cache + recipient projection

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Sync orchestrator (`internal/contacts/sync.go`)

Drives one full sync of one account: discover, choose strategy per
book, apply changeset, commit. Persistence is delegated to a small
`Store` interface implemented by `internal/cache` in Task 7.

**Files:**
- Create: `internal/contacts/sync.go`
- Create: `internal/contacts/sync_test.go`

- [ ] **Step 1: Define the Store seam**

Write `internal/contacts/sync.go`:

```go
package contacts

import (
	"context"
	"fmt"
	"time"
)

// Store is the persistence seam. internal/cache implements it; the
// sync engine has no SQLite dependency. All writes pass through
// Store, which decides on transactions.
type Store interface {
	// Books returns the cached set keyed by href.
	Books(ctx context.Context) (map[string]BookState, error)

	// UpsertBook persists discovered/refreshed metadata. Empty
	// SyncToken / CTAG are valid (uninitialized).
	UpsertBook(ctx context.Context, b BookState) error

	// ApplyChangeset writes added/updated contacts and removes
	// deleted ones. Atomic per book. token / ctag are stored
	// only on success.
	ApplyChangeset(ctx context.Context, bookHref string, added []Stored, removed []string, token, ctag string) error
}

// BookState is the cache projection of one address book row.
type BookState struct {
	Href          string
	DisplayName   string
	Description   string
	SyncToken     string
	CTAG          string
	SupportsSync  bool
	LastSyncedAt  time.Time
}

// Stored is one parsed vCard plus its server identity. The cache
// inserts/updates with these fields directly.
type Stored struct {
	Parsed      // embeds UID, Rev, Raw, Contact, Skip
	Href string
	ETag string
}

// Sync runs one pass over every discovered book.
func Sync(ctx context.Context, c *Client, s Store) error {
	home, err := c.HomeSet(ctx)
	if err != nil {
		return fmt.Errorf("home set: %w", err)
	}
	books, err := c.AddressBooks(ctx, home)
	if err != nil {
		return fmt.Errorf("list books: %w", err)
	}

	cached, err := s.Books(ctx)
	if err != nil {
		return fmt.Errorf("load cached books: %w", err)
	}

	for _, b := range books {
		state := cached[b.Path]
		state.Href = b.Path
		state.DisplayName = b.Name
		state.Description = b.Description
		if err := syncOne(ctx, c, s, &state); err != nil {
			return fmt.Errorf("book %s: %w", b.Path, err)
		}
		state.LastSyncedAt = time.Now()
		if err := s.UpsertBook(ctx, state); err != nil {
			return err
		}
	}
	return nil
}
```

(`carddav.AddressBook.Path` and `.Name` may differ; align after
verifying with `go doc`.)

- [ ] **Step 2: Implement the per-book strategy**

Append to `sync.go`:

```go
func syncOne(ctx context.Context, c *Client, s Store, state *BookState) error {
	if state.SupportsSync && state.SyncToken != "" {
		return syncIncremental(ctx, c, s, state)
	}
	if state.CTAG != "" {
		return syncCTAG(ctx, c, s, state)
	}
	return syncFull(ctx, c, s, state)
}

func syncIncremental(ctx context.Context, c *Client, s Store, state *BookState) error {
	resp, err := c.SyncCollection(ctx, state.Href, &SyncQuery{SyncToken: state.SyncToken})
	if err != nil {
		// Token rejection → fall back to full pull.
		state.SyncToken = ""
		return syncFull(ctx, c, s, state)
	}
	added, removed, err := materializeChangeset(ctx, c, state.Href, resp)
	if err != nil {
		return err
	}
	if err := s.ApplyChangeset(ctx, state.Href, added, removed, resp.SyncToken, state.CTAG); err != nil {
		return err
	}
	state.SyncToken = resp.SyncToken
	return nil
}

func syncCTAG(ctx context.Context, c *Client, s Store, state *BookState) error {
	ctag, err := c.CTAG(ctx, state.Href)
	if err == nil && ctag == state.CTAG && state.CTAG != "" {
		return nil // unchanged
	}
	if err := syncFull(ctx, c, s, state); err != nil {
		return err
	}
	if ctag != "" {
		state.CTAG = ctag
	}
	return nil
}

func syncFull(ctx context.Context, c *Client, s Store, state *BookState) error {
	// Full pull: list every href under the collection then multiget.
	// In go-webdav this is typically AddressBookQuery with no filter.
	hrefs, etags, err := listAllResources(ctx, c, state.Href)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	objs, err := c.Multiget(ctx, state.Href, hrefs)
	if err != nil {
		return fmt.Errorf("multiget: %w", err)
	}
	added := make([]Stored, 0, len(objs))
	for i, obj := range objs {
		p, err := parseObject(obj)
		if err != nil || p.Skip {
			continue
		}
		added = append(added, Stored{Parsed: p, Href: hrefs[i], ETag: etags[i]})
	}
	if err := s.ApplyChangeset(ctx, state.Href, added, nil, state.SyncToken, state.CTAG); err != nil {
		return err
	}
	// Probe sync-collection for next time.
	if !state.SupportsSync {
		if probe, err := c.SyncCollection(ctx, state.Href, &SyncQuery{SyncToken: ""}); err == nil {
			state.SupportsSync = true
			state.SyncToken = probe.SyncToken
		}
	}
	return nil
}
```

- [ ] **Step 3: Stub the helper functions**

`materializeChangeset`, `listAllResources`, and `parseObject` are
the upstream-API-touching pieces. Define their contracts:

```go
// materializeChangeset converts a SyncResponse into Stored rows for
// adds/changes plus a slice of removed hrefs. Adds/changes whose
// vCard body wasn't included in the REPORT are fetched via Multiget.
func materializeChangeset(ctx context.Context, c *Client, bookHref string, r *SyncResponse) ([]Stored, []string, error) {
	// Implementation: walk r.Added/r.Changed for hrefs missing
	// inline vCard data, batch them through c.Multiget, then map
	// each AddressObject to Stored via parseObject. r.Deleted is
	// returned as the removed slice.
	// Finalize against the actual SyncResponse field names.
	return nil, nil, fmt.Errorf("not implemented")
}

func listAllResources(ctx context.Context, c *Client, bookHref string) ([]string, []string, error) {
	// PROPFIND depth=1 for getetag — returns href + etag pairs.
	// go-webdav exposes this as a method on the carddav.Client;
	// finalize at implementation time.
	return nil, nil, fmt.Errorf("not implemented")
}

func parseObject(obj interface{}) (Parsed, error) {
	// obj is carddav.AddressObject; Data field holds the vCard.
	// Use the package's Data accessor and feed bytes to ParseVCard.
	return Parsed{}, fmt.Errorf("not implemented")
}
```

These three are the only places where upstream API drift might
require touch-up; they are isolated by design. Implement against
`go-webdav`'s actual surface during the task; the test in step 4
validates the orchestration layer above them.

- [ ] **Step 4: Write a fake-Store, fake-Client orchestration test**

Write `internal/contacts/sync_test.go`:

```go
package contacts

import (
	"context"
	"testing"
	"time"
)

type fakeStore struct {
	books    map[string]BookState
	apply    []applyCall
}

type applyCall struct {
	bookHref string
	added    []Stored
	removed  []string
	token    string
	ctag     string
}

func (f *fakeStore) Books(ctx context.Context) (map[string]BookState, error) {
	if f.books == nil {
		return map[string]BookState{}, nil
	}
	return f.books, nil
}

func (f *fakeStore) UpsertBook(ctx context.Context, b BookState) error {
	if f.books == nil {
		f.books = map[string]BookState{}
	}
	f.books[b.Href] = b
	return nil
}

func (f *fakeStore) ApplyChangeset(ctx context.Context, bookHref string, added []Stored, removed []string, token, ctag string) error {
	f.apply = append(f.apply, applyCall{bookHref, added, removed, token, ctag})
	return nil
}

func TestSync_FirstRun_FullPull_ProbesSyncCollection(t *testing.T) {
	// Stand up an httptest CardDAV server returning:
	//   - PROPFIND principal: stub principal
	//   - PROPFIND home-set: one book at /books/default/
	//   - PROPFIND book: two resources
	//   - REPORT addressbook-multiget: two vCards
	//   - REPORT sync-collection (probe): empty token, success
	// Verify ApplyChangeset called once with 2 adds, BookState
	// stored with SupportsSync=true.
	t.Skip("stand up httptest server; implement after Task 4 lands")
}

func TestSync_Incremental_TokenRejection_FallsBackToFull(t *testing.T) {
	// Server returns 412 Precondition Failed on sync-collection.
	// Engine should fall through to full pull and clear token.
	t.Skip("stand up httptest server")
}

func TestSync_CTAG_Unchanged_NoApply(t *testing.T) {
	// Cached state: CTAG="abc", SupportsSync=false.
	// Server PROPFIND returns CTAG="abc". No ApplyChangeset call.
	t.Skip("stand up httptest server")
}

func TestSync_GroupVCard_Skipped(t *testing.T) {
	// One vCard with KIND:group; verify it does not appear in
	// added.
	t.Skip("stand up httptest server")
}
```

The `t.Skip` placeholders are intentional: standing up an
httptest CardDAV server is the largest single sub-task in this
plan. Land the test stubs in this commit; flesh out the
fixtures incrementally after Task 7 (so the Store side is real,
not a fake).

- [ ] **Step 5: Build check**

Run: `go build ./internal/contacts/...`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/contacts/sync.go internal/contacts/sync_test.go
git commit -m "Pass 9m.6: CardDAV sync orchestrator

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Cache contacts methods + recipient hook

`internal/cache/contacts.go` implements the `contacts.Store`
interface, plus `SyncContacts`, `SuggestAddresses`, `LookupContact`.
Modify `syncer.go` to populate `message_recipients` on every
message upsert.

**Files:**
- Create: `internal/cache/contacts.go`
- Modify: `internal/cache/syncer.go`
- Modify: `internal/cache/contacts_test.go`

- [ ] **Step 1: Write the failing ranking test**

Append to `internal/cache/contacts_test.go`:

```go
func TestSuggestAddresses_RecencyDecay(t *testing.T) {
	acct := openTestAccount(t) // existing helper

	// Two emails: "alice@x" emailed 30 days ago twice; "bob@x"
	// emailed today once. Bob should outrank Alice (recency wins
	// in the decay formula).
	now := time.Now().Unix()
	d30 := now - 30*86400

	mustExec(t, acct.db, `INSERT INTO messages(uid, sent_at, from_addr, to_addr, cc_addr) VALUES (1, ?, '', 'alice@x', '')`, d30)
	mustExec(t, acct.db, `INSERT INTO messages(uid, sent_at, from_addr, to_addr, cc_addr) VALUES (2, ?, '', 'alice@x', '')`, d30)
	mustExec(t, acct.db, `INSERT INTO messages(uid, sent_at, from_addr, to_addr, cc_addr) VALUES (3, ?, '', 'bob@x', '')`, now)

	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (1, 'to', 'alice@x', ?)`, d30)
	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (2, 'to', 'alice@x', ?)`, d30)
	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (3, 'to', 'bob@x', ?)`, now)

	got, err := acct.SuggestAddresses(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("got %d suggestions; want ≥ 2", len(got))
	}
	if got[0].Email != "bob@x" {
		t.Errorf("recency-weighted top = %q; want bob@x", got[0].Email)
	}
}

func TestSuggestAddresses_PrefixFilter(t *testing.T) {
	acct := openTestAccount(t)
	now := time.Now().Unix()
	mustExec(t, acct.db, `INSERT INTO messages(uid, sent_at, from_addr, to_addr, cc_addr) VALUES (1, ?, '', 'alice@x', '')`, now)
	mustExec(t, acct.db, `INSERT INTO message_recipients(message_uid, role, address, sent_at) VALUES (1, 'to', 'alice@x', ?)`, now)

	got, _ := acct.SuggestAddresses(context.Background(), "ali")
	if len(got) != 1 || got[0].Email != "alice@x" {
		t.Errorf("prefix=ali → %+v", got)
	}
	got, _ = acct.SuggestAddresses(context.Background(), "zzz")
	if len(got) != 0 {
		t.Errorf("prefix=zzz → %+v", got)
	}
}

func TestLookupContact_HitMiss(t *testing.T) {
	acct := openTestAccount(t)
	mustExec(t, acct.db, `INSERT INTO addressbooks(href, display_name) VALUES ('/b/', 'Default')`)
	mustExec(t, acct.db,
		`INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, fn, last_synced_at) VALUES ('u1', '/b/', '/b/u1', 'e1', '', 'Alice', ?)`,
		time.Now().Unix())
	mustExec(t, acct.db, `INSERT INTO contact_emails(contact_uid, address) VALUES ('u1', 'alice@x')`)

	c, ok := acct.LookupContact(context.Background(), "alice@x")
	if !ok || c.Name != "Alice" {
		t.Errorf("hit: ok=%v c=%+v", ok, c)
	}
	_, ok = acct.LookupContact(context.Background(), "nobody@x")
	if ok {
		t.Error("miss should return ok=false")
	}
}
```

(`openTestAccount` and `mustExec` follow whatever pattern other
cache tests already use — adapt names.)

- [ ] **Step 2: Run, expect FAIL**

Run: `go test ./internal/cache/ -run "TestSuggest|TestLookup" -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement the methods**

Write `internal/cache/contacts.go`:

```go
package cache

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/glw907/poplar/internal/contacts"
)

const suggestQuery = `
WITH scored AS (
    SELECT mr.address                                                      AS addr,
           MAX(mr.name)                                                    AS name,
           SUM(1.0 / (1 + (julianday('now') - julianday(mr.sent_at, 'unixepoch')))) AS score
      FROM message_recipients mr
     WHERE LOWER(mr.address) LIKE ?
        OR LOWER(mr.name)    LIKE ?
     GROUP BY mr.address
)
SELECT s.addr,
       COALESCE(c.fn, s.name)              AS display,
       COALESCE(c.org, '')                 AS org,
       (c.uid IS NOT NULL AND c.family = '' AND c.given = '' AND c.org != '') AS is_org
  FROM scored s
  LEFT JOIN contact_emails ce ON LOWER(ce.address) = LOWER(s.addr)
  LEFT JOIN contacts c        ON c.uid = ce.contact_uid
 ORDER BY s.score DESC, s.addr ASC
 LIMIT 7;
`

// SuggestAddresses returns up to 7 ranked autocomplete rows for the
// prefix. Empty prefix returns the most-recent contacts (bounded
// by the LIMIT). prefix matches address OR display name; case
// insensitive.
func (a *Account) SuggestAddresses(ctx context.Context, prefix string) ([]contacts.Suggestion, error) {
	pat := strings.ToLower(prefix) + "%"
	rows, err := a.db.QueryContext(ctx, suggestQuery, pat, pat)
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", err)
	}
	defer rows.Close()

	var out []contacts.Suggestion
	for rows.Next() {
		var s contacts.Suggestion
		var isOrg int
		if err := rows.Scan(&s.Email, &s.Name, &s.Org, &isOrg); err != nil {
			return nil, err
		}
		s.IsOrg = isOrg == 1
		if s.Name == "" {
			s.Name = s.Email
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LookupContact returns the carded contact for an email address.
// Misses return ok=false; the caller falls back to whatever name
// the message itself carries.
func (a *Account) LookupContact(ctx context.Context, address string) (contacts.Contact, bool) {
	const q = `
SELECT c.fn, c.family, c.given, c.org, c.title, c.note
  FROM contact_emails ce
  JOIN contacts c ON c.uid = ce.contact_uid
 WHERE LOWER(ce.address) = LOWER(?)
 LIMIT 1
`
	var c contacts.Contact
	err := a.db.QueryRowContext(ctx, q, address).Scan(
		&c.Name, &c.Family, &c.Given, &c.Org, &c.Title, &c.Note,
	)
	if err == sql.ErrNoRows {
		return contacts.Contact{}, false
	}
	if err != nil {
		return contacts.Contact{}, false
	}
	if c.Family == "" && c.Given == "" && c.Org != "" {
		c.Kind = contacts.KindOrg
	}
	c.Emails = a.loadEmails(ctx, c.Name) // simplified; better: pass uid
	c.Phones = a.loadPhones(ctx, c.Name)
	return c, true
}

// loadEmails / loadPhones reload the full child rows. The Lookup
// query above doesn't return the uid for clarity; production
// implementation should join uid through and use it here. Adjust
// during implementation.
func (a *Account) loadEmails(ctx context.Context, fn string) []contacts.Email {
	rows, err := a.db.QueryContext(ctx,
		`SELECT ce.address, ce.label FROM contact_emails ce JOIN contacts c ON c.uid = ce.contact_uid WHERE c.fn = ? ORDER BY ce.pref ASC`, fn)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []contacts.Email
	for rows.Next() {
		var e contacts.Email
		if rows.Scan(&e.Address, &e.Label) == nil {
			out = append(out, e)
		}
	}
	return out
}

func (a *Account) loadPhones(ctx context.Context, fn string) []contacts.Phone {
	rows, err := a.db.QueryContext(ctx,
		`SELECT cp.number, cp.label FROM contact_phones cp JOIN contacts c ON c.uid = cp.contact_uid WHERE c.fn = ? ORDER BY cp.pref ASC`, fn)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []contacts.Phone
	for rows.Next() {
		var p contacts.Phone
		if rows.Scan(&p.E164, &p.Label) == nil {
			out = append(out, p)
		}
	}
	return out
}

// --- contacts.Store implementation ---

// Books returns every cached address book keyed by href.
func (a *Account) Books(ctx context.Context) (map[string]contacts.BookState, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT href, display_name, description, sync_token, ctag, supports_sync, last_synced_at FROM addressbooks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]contacts.BookState{}
	for rows.Next() {
		var b contacts.BookState
		var supports int
		var ts int64
		if err := rows.Scan(&b.Href, &b.DisplayName, &b.Description, &b.SyncToken, &b.CTAG, &supports, &ts); err != nil {
			return nil, err
		}
		b.SupportsSync = supports == 1
		b.LastSyncedAt = time.Unix(ts, 0)
		out[b.Href] = b
	}
	return out, rows.Err()
}

func (a *Account) UpsertBook(ctx context.Context, b contacts.BookState) error {
	supports := 0
	if b.SupportsSync {
		supports = 1
	}
	_, err := a.db.ExecContext(ctx,
		`INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(href) DO UPDATE SET
		   display_name=excluded.display_name,
		   description=excluded.description,
		   sync_token=excluded.sync_token,
		   ctag=excluded.ctag,
		   supports_sync=excluded.supports_sync,
		   last_synced_at=excluded.last_synced_at`,
		b.Href, b.DisplayName, b.Description, b.SyncToken, b.CTAG, supports, b.LastSyncedAt.Unix(),
	)
	return err
}

func (a *Account) ApplyChangeset(ctx context.Context, bookHref string, added []contacts.Stored, removed []string, token, ctag string) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, href := range removed {
		if _, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE href = ?`, href); err != nil {
			return err
		}
	}
	now := time.Now().Unix()
	for _, s := range added {
		if err := upsertContactTx(ctx, tx, bookHref, s, now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE addressbooks SET sync_token=?, ctag=? WHERE href=?`,
		token, ctag, bookHref); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertContactTx(ctx context.Context, tx *sql.Tx, bookHref string, s contacts.Stored, now int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   addressbook_href=excluded.addressbook_href,
		   href=excluded.href,
		   etag=excluded.etag,
		   vcard=excluded.vcard,
		   rev=excluded.rev,
		   fn=excluded.fn,
		   family=excluded.family,
		   given=excluded.given,
		   org=excluded.org,
		   title=excluded.title,
		   note=excluded.note,
		   last_synced_at=excluded.last_synced_at`,
		s.UID, bookHref, s.Href, s.ETag, s.Raw, s.Rev,
		s.Contact.Name, s.Contact.Family, s.Contact.Given,
		s.Contact.Org, s.Contact.Title, s.Contact.Note, now,
	)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM contact_emails WHERE contact_uid=?`, s.UID); err != nil {
		return err
	}
	for i, e := range s.Contact.Emails {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO contact_emails(contact_uid, address, label, pref) VALUES (?, ?, ?, ?)`,
			s.UID, e.Address, e.Label, i+1)
		if err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM contact_phones WHERE contact_uid=?`, s.UID); err != nil {
		return err
	}
	for i, p := range s.Contact.Phones {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO contact_phones(contact_uid, number, label, pref) VALUES (?, ?, ?, ?)`,
			s.UID, p.E164, p.Label, i+1)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncContacts is the App entry point. Builds a CardDAV client from
// the account's config and runs one full sync.
func (a *Account) SyncContacts(ctx context.Context, cfg *contacts.ClientConfig) error {
	if cfg == nil {
		return nil
	}
	cl, err := contacts.NewClient(cfg.URL, cfg.Username, cfg.Password, cfg.InsecureTLS)
	if err != nil {
		return err
	}
	return contacts.Sync(ctx, cl, a)
}
```

Add to `internal/contacts/sync.go`:

```go
// ClientConfig is the runtime input to NewClient — resolved from
// internal/config at call time.
type ClientConfig struct {
	URL         string
	Username    string
	Password    string
	InsecureTLS bool
}
```

- [ ] **Step 4: Hook recipient population in `syncer.go`**

Find the upsert path (`upsertMessage` or equivalent) in
`internal/cache/syncer.go`. Inside the same transaction, after
the messages-row insert/update, run:

```go
if err := writeRecipientsTx(ctx, tx, m); err != nil {
	return err
}
```

Add the helper:

```go
func writeRecipientsTx(ctx context.Context, tx *sql.Tx, m *mail.MessageInfo) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_recipients WHERE message_uid = ?`, m.UID); err != nil {
		return err
	}
	for _, role := range []struct {
		name string
		raw  string
	}{
		{"from", m.From},
		{"to", m.To},
		{"cc", m.Cc},
	} {
		if role.raw == "" {
			continue
		}
		addrs, err := mail.ParseAddressList(role.raw)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			_, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (?, ?, ?, ?, ?)`,
				m.UID, role.name, strings.ToLower(a.Address), a.Name, m.SentAt.Unix())
			if err != nil {
				return err
			}
		}
	}
	return nil
}
```

(Adjust `m.From` etc. to match the actual `MessageInfo`/syncer
row shape — `From`/`To`/`Cc` are RFC 5322-style strings on the
wire today.)

- [ ] **Step 5: Run all cache tests**

Run: `go test ./internal/cache/...`
Expected: green, including the three new tests from step 1.

- [ ] **Step 6: Commit**

```bash
git add internal/cache/contacts.go internal/cache/contacts_test.go internal/cache/syncer.go internal/contacts/sync.go
git commit -m "Pass 9m.7: cache contacts methods + recipient hook

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: App wiring

Three call sites swap. App also installs the 15-min sync ticker
and kicks an initial sync after first frame.

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Locate the call sites**

Run: `grep -n "FixtureSuggestions\|LookupByEmail" internal/ui/app.go`

Expected lines (per the spec):
- L247 — sender lookup
- L611 — `New(...)` for fresh compose
- L620 — `Open(...)` for restore-from-Draft
- L721 — second `New(...)` site (forward path)

- [ ] **Step 2: Add a `SuggestFn` accessor**

In `internal/ui/app.go`, add a method on App:

```go
// suggestAddresses adapts cache.Account.SuggestAddresses to the
// SuggestFn signature compose expects (synchronous, returns rows
// only). Errors degrade silently to "no suggestions" — autocomplete
// is best-effort, not a blocking I/O surface.
func (m *App) suggestAddresses(prefix string) []contacts.Suggestion {
	if m.acct == nil {
		return nil
	}
	out, err := m.acct.SuggestAddresses(context.Background(), prefix)
	if err != nil {
		return nil
	}
	return out
}
```

- [ ] **Step 3: Replace L611 / L620 / L721**

Replace `contacts.FixtureSuggestions` with `m.suggestAddresses` at
all three sites.

```go
// before
m.compose = uicompose.New(uicompose.NewStyles(m.theme), m.acct.AccountEmail(), contacts.FixtureSuggestions)
// after
m.compose = uicompose.New(uicompose.NewStyles(m.theme), m.acct.AccountEmail(), m.suggestAddresses)
```

(Same shape for `Open` and the second `New`.)

- [ ] **Step 4: Replace L247 sender lookup**

```go
// before
match, found := contacts.LookupByEmail(contacts.Fixtures(), msg.Email)
// after
match, found := m.acct.LookupContact(context.Background(), msg.Email)
```

- [ ] **Step 5: Add the sync ticker**

Find `App.Init` (or wherever the initial Cmd batch is composed).
Add:

```go
// syncContactsCmd runs a single CardDAV sync. Errors surface
// through the standard ErrorMsg banner.
func syncContactsCmd(acct *cache.Account, cfg *contacts.ClientConfig) tea.Cmd {
	return func() tea.Msg {
		if err := acct.SyncContacts(context.Background(), cfg); err != nil {
			return uicore.ErrorMsg{Op: "sync contacts", Err: err}
		}
		return contactsSyncedMsg{}
	}
}

type contactsSyncedMsg struct{}

// scheduleSyncCmd fires another sync after refreshInterval.
func scheduleSyncCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return contactsTickMsg{} })
}

type contactsTickMsg struct{}
```

In `App.Update`:

```go
case contactsTickMsg:
	cmds = append(cmds,
		syncContactsCmd(m.acct, m.contactsCfg),
		scheduleSyncCmd(m.contactsRefresh),
	)
case contactsSyncedMsg:
	// no-op; future passes might surface a "synced N contacts" toast
```

In `App.Init`, batch the initial sync + first tick:

```go
if m.contactsCfg != nil {
	cmds = append(cmds,
		syncContactsCmd(m.acct, m.contactsCfg),
		scheduleSyncCmd(m.contactsRefresh),
	)
}
```

`m.contactsCfg` and `m.contactsRefresh` are new App fields,
populated by the constructor from the resolved `AccountConfig`.

- [ ] **Step 6: Wire the constructor**

Update `NewApp` (or whatever the builder is named) to accept the
`*ContactsConfig` from the loaded account and translate it:

```go
if a.Contacts != nil {
	app.contactsCfg = &contacts.ClientConfig{
		URL:         a.Contacts.URL,
		Username:    a.Contacts.Username,
		Password:    resolvedPassword(a.Contacts), // existing helper pattern
		InsecureTLS: a.Contacts.InsecureTLS,
	}
	app.contactsRefresh = a.Contacts.RefreshInterval
}
```

(Re-use whatever password-resolve helper the IMAP/JMAP path
already uses; both honor `password-cmd`.)

- [ ] **Step 7: Build + test**

Run: `make check`
Expected: green.

- [ ] **Step 8: Live tmux smoke test**

Follow `.claude/docs/tmux-testing.md`. Capture at 80×24:

1. Open a real Fastmail account with `[account.contacts]`
   pointing at `https://carddav.fastmail.com/dav/addressbooks/user/<email>/`.
2. Wait for first sync (watch for the spinner / status line).
3. Press `c` to open compose; type 2+ chars in To:; verify
   dropdown shows real cached addresses ranked by recency.
4. Open a message from a known sender; press `i`; verify the
   popover shows the carded contact rather than a fixture.

Capture screenshots and link them in the pass-end ADR.

- [ ] **Step 9: Commit**

```bash
git add internal/ui/app.go
git commit -m "Pass 9m.8: wire cache-backed contacts into compose + popover

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Phone validation upgrade

Replace the "any non-empty string" gate in `Form.validate` with
`phonenumbers.Parse`. Default region: "US". Empty rows still
validate (they're inert UI scaffolding).

**Files:**
- Modify: `internal/ui/contacts/form.go`
- Modify: `internal/ui/contacts/form_test.go`
- Modify: `go.mod`

- [ ] **Step 1: Add the dep**

Run: `go get github.com/nyaruka/phonenumbers@latest`

- [ ] **Step 2: Locate the existing validation**

Run: `grep -n "phone\|Phone" internal/ui/contacts/form.go | head`
Find the validation block in `Form.validate` (or whatever
function returns the save-blocking error).

- [ ] **Step 3: Write the failing test**

Append to `internal/ui/contacts/form_test.go`:

```go
func TestForm_PhoneValidation(t *testing.T) {
	cases := []struct {
		num     string
		wantErr bool
	}{
		{"", false},                  // empty is fine
		{"+15555550100", false},      // E.164
		{"(555) 555-0100", false},    // parseable as US
		{"not a phone", true},
		{"123", true},                // too short
	}
	for _, tc := range cases {
		t.Run(tc.num, func(t *testing.T) {
			err := validatePhoneNumber(tc.num)
			if tc.wantErr && err == nil {
				t.Errorf("want error for %q", tc.num)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.num, err)
			}
		})
	}
}
```

- [ ] **Step 4: Run, expect FAIL**

Run: `go test ./internal/ui/contacts/ -run TestForm_PhoneValidation`
Expected: FAIL — `validatePhoneNumber` undefined.

- [ ] **Step 5: Implement**

In `internal/ui/contacts/form.go`:

```go
import "github.com/nyaruka/phonenumbers"

const defaultPhoneRegion = "US"

func validatePhoneNumber(s string) error {
	if s == "" {
		return nil
	}
	num, err := phonenumbers.Parse(s, defaultPhoneRegion)
	if err != nil {
		return fmt.Errorf("phone: %w", err)
	}
	if !phonenumbers.IsValidNumber(num) {
		return fmt.Errorf("phone: not a valid number")
	}
	return nil
}
```

Wire into the existing `validate()` (or whichever method gates
save) where the old "non-empty string check" lived:

```go
for _, p := range f.phones {
	if err := validatePhoneNumber(p.Value()); err != nil {
		return err
	}
}
```

- [ ] **Step 6: Run all form tests**

Run: `go test ./internal/ui/contacts/...`
Expected: green.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/contacts/form.go internal/ui/contacts/form_test.go go.mod go.sum
git commit -m "Pass 9m.9: phone validation via phonenumbers

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Pass-end ritual

Run the consolidation ritual from the `poplar-pass` skill.

- [ ] **Step 1: `/simplify` over the diff**

Invoke the simplify skill against the full `master..HEAD` diff for
this pass. Apply genuine wins; commit if any.

- [ ] **Step 2: Idiomatic-bubbletea check**

Only the App-side wiring touched `internal/ui/`. Run §10 of
`docs/poplar/bubbletea-conventions.md` against the diff. The
sync-tick path needs:
- [ ] `tea.Tick` used for the timer (not `time.AfterFunc`).
- [ ] Sync work in `tea.Cmd`, not in `Update`.
- [ ] Errors via `uicore.ErrorMsg`, not panics or logs.

- [ ] **Step 3: Write the ADR**

`docs/poplar/decisions/0175-carddav-ingest.md` (or next number):

```markdown
---
title: CardDAV ingest and contacts ranking
status: accepted
date: 2026-05-NN
---

## Context

(The fixture-backed contacts pool from Pass 9.1 needs a real
backend before v1. CardDAV is the obvious fit — Fastmail and most
self-hosted servers expose it; emersion/go-webdav is in the
library family per ADR-0... .)

## Decision

(Schema v8, internal/contacts/ package, sync-collection + CTAG
fallback, frequency × recency ranking, [[account.contacts]]
config block. Read-only this pass; write-back deferred to 9m.1.)

## Consequences

(Compose autocomplete and i-popover now reflect the user's real
address book. message_recipients projection unlocks future
ranking refinements without schema changes. Pass 9m.1 picks up
the form save → CardDAV PUT round trip.)
```

- [ ] **Step 4: Update `docs/poplar/invariants.md`**

Add to architecture: `internal/contacts/` package + ClientConfig.
Add to schema: v8 (five new tables). Add to config: `[[account.contacts]]`.
Replace the Pass 9.1 fixture-pool sentence with a cache-backed
description.

- [ ] **Step 5: Update `docs/poplar/decisions/INDEX.md`**

Add a row pointing the relevant binding facts to ADR-0175.

- [ ] **Step 6: Update `docs/poplar/STATUS.md`**

- Mark Pass 9m `done`.
- Insert Pass 9m.1 as the new current pass with a starter prompt:

```markdown
### Next starter prompt (Pass 9m.1)

> **Goal.** Round-trip the contact-edit Form: form save → cache
> upsert → outbox PUT → CardDAV server. Round-trip the form
> discard via the existing outbox DiscardOp.
>
> **Scope.** Extend `cache.OpKind` with `KindContactPut` and
> `KindContactDelete`; outbox payload carries vCard bytes for
> these kinds. Drainer dispatches via the CardDAV client added
> in 9m. Form's "Save to" cycler now affects the destination
> address book. Phone validation already in place.
>
> **Settled.** Storage shape (vCard blob + projection columns) is
> 9m's; no schema migration. Default-addressbook config pin
> already implemented. Conflict matrix (auth/not-found/transient)
> mirrors mail outbox semantics.
>
> **Still open — brainstorm:** vCard regeneration on edit
> (fully rebuild from projection vs. patch the stored blob);
> ETag round-trip across local edits; deletion UI in the Form.
>
> **Approach.** Brainstorm, write
> `docs/superpowers/plans/YYYY-MM-DD-carddav-writeback.md`,
> implement. Standard pass-end ritual.
```

- [ ] **Step 7: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-07-carddav-ingest.md \
       docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-07-carddav-ingest-design.md \
       docs/superpowers/archive/specs/
```

- [ ] **Step 8: `make check`**

Run: `make check`
Expected: green (vet + test + voice + fmt-check).

- [ ] **Step 9: Commit, push, install**

```bash
git add -A
git commit -m "Pass 9m: CardDAV ingest + autocomplete

$(cat <<'EOF'
Read-only contacts cache backed by CardDAV via
emersion/go-webdav. Schema v8 adds addressbooks/contacts/
contact_emails/contact_phones/message_recipients. Compose
autocomplete and i-popover now hit the cache; fixtures retired
from App. Form write-back deferred to Pass 9m.1.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
git push
make install
```
