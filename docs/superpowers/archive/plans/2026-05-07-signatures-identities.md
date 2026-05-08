# Pass 9n — Per-account identities and signatures

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land per-account `[[account.identity]]` config with ordered per-identity signatures, expose identity + signature cycling in compose (`Space/← /→ /h/l` for identity in focusFrom; `g` in focusFrom or `Ctrl+G` anywhere for signature), and thread the chosen identity through both JMAP and IMAP send paths.

**Architecture:** New `Identity` and `Signature` value types in `internal/config`, decoded from `[[account.identity]]` and `[[account.identity.signature]]` TOML blocks. `internal/compose` gains `Draft.Identity int` and `Draft.Signature int` (-1 for none); `AssembleMIME` takes the identities slice and applies the chosen identity to the From header and the chosen signature's `Text` to the body. `internal/ui/compose` adds a `focusFrom` state, From-row chip, and the cycler keys. `internal/mailjmap` swaps its single-identity cache for an email-keyed map. The reader already dims RFC 3676 signature blocks (`internal/content` + `theme.Signature`); no change there.

**Tech Stack:** Go 1.26, `pelletier/go-toml/v2`, `bubbletea`, `bubbles/textinput`, `emersion/go-message/mail`, `git.sr.ht/~rockorager/go-jmap`.

---

## File structure

**Created:**
- (none — all changes land in existing files)

**Modified:**
- `internal/config/accounts.go` — `Identity`/`Signature` types, decode wiring, legacy `from` synthesis.
- `internal/config/accounts_test.go` — decode tests.
- `internal/compose/draft.go` — `Identity` and `Signature` fields on `Draft`.
- `internal/compose/assemble.go` — `AssembleMIME` signature; sig append.
- `internal/compose/assemble_test.go` — table tests for identity + sig append.
- `internal/compose/seed.go` — `SeedReply`/`SeedReplyAll`/`SeedForward` initialize the new fields.
- `internal/mailjmap/jmap.go` — `identityIDs map[string]jmap.ID`; `resolveIdentityID` keyed by email.
- `internal/mailjmap/jmap_test.go` — multi-identity cache test.
- `internal/ui/compose/model.go` — `focusFrom` state, identities slice, From-row chip, cycler keys.
- `internal/ui/compose/model_test.go` — focus + cycler + chip tests.
- `internal/ui/compose/styles.go` — chip and cycler-glyph styles.
- `internal/ui/footer.go` — compose footer hint groups (`Ctrl+G sig` always; `Space/←→ identity` when focusFrom).
- `internal/ui/app.go` — pass `[]config.Identity` into `uicompose.New`/`Open`.

**No changes needed:**
- `internal/content/parse.go` and `internal/content/render.go` already detect the RFC 3676 sentinel and render `Signature` blocks via the dim `theme.Signature` style.

---

## Task 1 — Config: Identity and Signature types + TOML decode

**Files:**
- Modify: `internal/config/accounts.go`
- Test: `internal/config/accounts_test.go`

- [ ] **Step 1.1: Add the value types and TOML entry types**

In `internal/config/accounts.go`, add the types and the decode entries near the existing `accountEntry`:

```go
// Identity is one of an account's sending identities. Name is the
// display name (rendered in From), Email is the address (must match
// a server-side identity for JMAP submission), Signatures is the
// ordered list the user cycles through in compose.
type Identity struct {
	Name       string
	Email      string
	Signatures []Signature
}

// Signature is one of an identity's named signatures. Text is the
// resolved body (with RFC 3676 "-- \n" sentinel guaranteed). Name
// labels it in the compose chip and footer.
type Signature struct {
	Name string
	Text string
}

type identityEntry struct {
	Name       string           `toml:"name"`
	Email      string           `toml:"email"`
	Signatures []signatureEntry `toml:"signature"`
}

type signatureEntry struct {
	Name string `toml:"name"`
	Text string `toml:"text"`
	File string `toml:"file"`
}
```

Then add the field on `accountEntry`:

```go
	Identities []identityEntry   `toml:"identity"`
```

And the field on `AccountConfig`:

```go
	// Identities is the ordered list of [[account.identity]] blocks.
	// Always length >= 1: when no blocks are configured, the legacy
	// top-level From synthesizes a single identity with no signatures.
	Identities []Identity
```

- [ ] **Step 1.2: Write the failing test for empty-block decode**

In `internal/config/accounts_test.go`, add:

```go
func TestParseAccountsIdentitiesDecode(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		want    []Identity
		wantErr string
	}{
		{
			name: "two identities ordered, with mixed text and file signatures",
			toml: `
[[account]]
name = "fastmail"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "Geoff Wright"
email = "geoff@907.life"

[[account.identity.signature]]
name = "default"
text = "Geoff Wright\nhttps://907.life"

[[account.identity]]
name = "Geoff @ ASC"
email = "geoff.wright@aksailingclub.org"

[[account.identity.signature]]
name = "casual"
text = "Geoff"
`,
			want: []Identity{
				{
					Name:  "Geoff Wright",
					Email: "geoff@907.life",
					Signatures: []Signature{
						{Name: "default", Text: "-- \nGeoff Wright\nhttps://907.life"},
					},
				},
				{
					Name:  "Geoff @ ASC",
					Email: "geoff.wright@aksailingclub.org",
					Signatures: []Signature{
						{Name: "casual", Text: "-- \nGeoff"},
					},
				},
			},
		},
		{
			name: "identity with zero signatures decodes",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"
`,
			want: []Identity{{Name: "G", Email: "g@x"}},
		},
		{
			name: "text and file mutually exclusive",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "x"
text = "y"
file = "/tmp/z"
`,
			wantErr: `signature "x": text and file are mutually exclusive`,
		},
		{
			name: "signature missing both text and file",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "x"
`,
			wantErr: `signature "x": text or file is required`,
		},
		{
			name: "duplicate signature name within identity",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "dup"
text = "a"

[[account.identity.signature]]
name = "dup"
text = "b"
`,
			wantErr: `duplicate signature name "dup"`,
		},
		{
			name: "preserves existing -- \\n sentinel",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "x"
text = "-- \nalready sentineled"
`,
			want: []Identity{{
				Name:  "G",
				Email: "g@x",
				Signatures: []Signature{
					{Name: "x", Text: "-- \nalready sentineled"},
				},
			}},
		},
		{
			name: "invalid identity email",
			toml: `
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "not-an-address"
`,
			wantErr: `identity "G": email`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAccountsFromBytes([]byte(tt.toml))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("got nil error, want substring %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAccountsFromBytes: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("got %d accounts, want 1", len(got))
			}
			if !reflect.DeepEqual(got[0].Identities, tt.want) {
				t.Errorf("identities mismatch\ngot:  %#v\nwant: %#v", got[0].Identities, tt.want)
			}
		})
	}
}
```

(If `reflect` and `strings` aren't already imported in the test file, add them.)

- [ ] **Step 1.3: Run the test — it must fail at compile time first, then with decode failures**

Run: `go test ./internal/config/ -run TestParseAccountsIdentitiesDecode -v`

Expected: compile error for `Identities` field references (after Step 1.1, the field exists but no decode yet, so cases will fail with empty results).

- [ ] **Step 1.4: Implement decode in `toAccountConfig`**

In `internal/config/accounts.go`, after the existing identity-block (around the `if e.From != ""` line), add the decode:

```go
	identities, err := decodeIdentities(e.Name, e.Identities)
	if err != nil {
		return nil, err
	}
	if len(identities) == 0 && acct.From != nil {
		identities = []Identity{{
			Name:  acct.From.Name,
			Email: acct.From.Address,
		}}
	}
	if len(identities) == 0 && acct.From == nil {
		// Both empty — preserve existing "from required" error path. JMAP
		// auto-discovers email so we don't fail here for it; ParseAccounts
		// has no other gate today, so leave Identities empty and rely on
		// downstream require checks if any. (Compose initialization will
		// not fire when len(Identities) == 0.)
	}
	acct.Identities = identities
	if len(acct.Identities) > 0 {
		// First identity is the default; sync legacy From for any
		// consumer still reading it.
		first := acct.Identities[0]
		acct.From = &mail.Address{Name: first.Name, Address: first.Email}
	}
```

Then add `decodeIdentities`:

```go
func decodeIdentities(account string, entries []identityEntry) ([]Identity, error) {
	out := make([]Identity, 0, len(entries))
	for _, ie := range entries {
		if ie.Name == "" {
			return nil, fmt.Errorf("account %q: identity: name is required", account)
		}
		if _, err := mail.ParseAddress(ie.Email); err != nil {
			return nil, fmt.Errorf("account %q: identity %q: email: %w", account, ie.Name, err)
		}
		sigs, err := decodeSignatures(ie.Name, ie.Signatures)
		if err != nil {
			return nil, fmt.Errorf("account %q: %w", account, err)
		}
		out = append(out, Identity{
			Name:       ie.Name,
			Email:      ie.Email,
			Signatures: sigs,
		})
	}
	return out, nil
}

func decodeSignatures(identity string, entries []signatureEntry) ([]Signature, error) {
	seen := make(map[string]bool, len(entries))
	out := make([]Signature, 0, len(entries))
	for _, se := range entries {
		if se.Name == "" {
			return nil, fmt.Errorf("identity %q: signature: name is required", identity)
		}
		if seen[se.Name] {
			return nil, fmt.Errorf("identity %q: duplicate signature name %q", identity, se.Name)
		}
		seen[se.Name] = true
		if se.Text != "" && se.File != "" {
			return nil, fmt.Errorf("identity %q: signature %q: text and file are mutually exclusive", identity, se.Name)
		}
		if se.Text == "" && se.File == "" {
			return nil, fmt.Errorf("identity %q: signature %q: text or file is required", identity, se.Name)
		}
		text := se.Text
		if se.File != "" {
			path, err := ExpandHome(se.File)
			if err != nil {
				return nil, fmt.Errorf("identity %q: signature %q: file: %w", identity, se.Name, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("identity %q: signature %q: file: %w", identity, se.Name, err)
			}
			text = string(data)
		}
		text = injectSentinel(text)
		out = append(out, Signature{Name: se.Name, Text: text})
	}
	return out, nil
}

// injectSentinel prepends RFC 3676's "-- \n" if t doesn't already start
// with it. Idempotent.
func injectSentinel(t string) string {
	if strings.HasPrefix(t, "-- \n") {
		return t
	}
	return "-- \n" + t
}
```

- [ ] **Step 1.5: Run the test**

Run: `go test ./internal/config/ -run TestParseAccountsIdentitiesDecode -v`

Expected: PASS for all subtests.

- [ ] **Step 1.6: Add file-resolution test**

```go
func TestParseAccountsSignatureFile(t *testing.T) {
	dir := t.TempDir()
	sigPath := filepath.Join(dir, "sig.md")
	if err := os.WriteFile(sigPath, []byte("Geoff\nhttps://907.life"), 0o600); err != nil {
		t.Fatalf("write sig: %v", err)
	}
	toml := fmt.Sprintf(`
[[account]]
name = "a"
provider = "fastmail"
password = "x"

[[account.identity]]
name = "G"
email = "g@x"

[[account.identity.signature]]
name = "default"
file = %q
`, sigPath)
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	want := "-- \nGeoff\nhttps://907.life"
	if got[0].Identities[0].Signatures[0].Text != want {
		t.Errorf("text = %q, want %q", got[0].Identities[0].Signatures[0].Text, want)
	}
}
```

Run: `go test ./internal/config/ -run TestParseAccountsSignatureFile -v` — Expected: PASS.

- [ ] **Step 1.7: Add legacy-from synthesis test**

```go
func TestParseAccountsLegacyFromSynthesis(t *testing.T) {
	toml := `
[[account]]
name = "a"
provider = "fastmail"
password = "x"
from = "Geoff <geoff@907.life>"
`
	got, err := ParseAccountsFromBytes([]byte(toml))
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	want := []Identity{{Name: "Geoff", Email: "geoff@907.life"}}
	if !reflect.DeepEqual(got[0].Identities, want) {
		t.Errorf("identities = %#v, want %#v", got[0].Identities, want)
	}
}
```

Run: `go test ./internal/config/ -run TestParseAccountsLegacyFromSynthesis -v` — Expected: PASS.

- [ ] **Step 1.8: Commit**

```bash
git add internal/config/accounts.go internal/config/accounts_test.go
git commit -m "Pass 9n: config Identity + Signature decode

[[account.identity]] arrays with [[account.identity.signature]]
sub-blocks. File resolution at config-load with RFC 3676 sentinel
injection. Legacy top-level from synthesizes identity #1 when no
blocks are configured. First-in-order is the default."
```

---

## Task 2 — Compose domain: Draft fields + AssembleMIME signature

**Files:**
- Modify: `internal/compose/draft.go`
- Modify: `internal/compose/assemble.go`
- Modify: `internal/compose/seed.go`
- Test: `internal/compose/assemble_test.go`

- [ ] **Step 2.1: Update `Draft`**

In `internal/compose/draft.go`, add the two fields:

```go
type Draft struct {
	From    gomail.Address
	To      []gomail.Address
	Cc      []gomail.Address
	Bcc     []gomail.Address
	Subject string
	Body    string

	InReplyTo  string
	References []string

	Attachments []string

	// Identity is the index into the account's Identities slice that
	// the user picked at compose time. The compose UI mirrors the
	// chosen identity into From; AssembleMIME reads From directly.
	Identity int

	// Signature is the index into Identities[Identity].Signatures, or
	// -1 for "no signature." -1 is the legitimate state when the
	// identity has zero signatures, or when the user cycled past the
	// last sig.
	Signature int
}
```

- [ ] **Step 2.2: Add the AssembleMIME signature change failing test**

In `internal/compose/assemble_test.go`, add a test (alongside existing tests). Define a small local `idents` slice rather than importing `internal/config` (compose stays a leaf package on the domain side; the UI layer does the conversion):

```go
func TestAssembleMIMEAppendsSignature(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)

	idents := []Identity{
		{
			Name:  "Geoff",
			Email: "geoff@907.life",
			Signatures: []Signature{
				{Name: "default", Text: "-- \nGeoff Wright"},
			},
		},
	}

	tests := []struct {
		name      string
		sig       int
		wantBody  string
		wantNoSig bool
	}{
		{
			name:     "sig included",
			sig:      0,
			wantBody: "Hello.\n\n-- \nGeoff Wright",
		},
		{
			name:      "sig omitted (-1)",
			sig:       -1,
			wantBody:  "Hello.",
			wantNoSig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Draft{
				From:      gomail.Address{Name: "Geoff", Address: "geoff@907.life"},
				To:        []gomail.Address{{Address: "x@y"}},
				Subject:   "hi",
				Body:      "Hello.",
				Identity:  0,
				Signature: tt.sig,
			}
			got, err := AssembleMIME(d, idents, now)
			if err != nil {
				t.Fatalf("AssembleMIME: %v", err)
			}
			if !bytes.Contains(got, []byte(tt.wantBody)) {
				t.Errorf("body missing %q in:\n%s", tt.wantBody, got)
			}
			if tt.wantNoSig && bytes.Contains(got, []byte("-- \n")) {
				t.Errorf("did not expect sentinel in:\n%s", got)
			}
		})
	}
}
```

Add a local-types declaration block at the top of `assemble.go`:

```go
// Identity mirrors the compose-relevant slice of config.Identity.
// Defined here so the compose package stays free of the config
// dependency. The UI layer fills these from []config.Identity.
type Identity struct {
	Name       string
	Email      string
	Signatures []Signature
}

// Signature mirrors config.Signature.
type Signature struct {
	Name string
	Text string
}
```

- [ ] **Step 2.3: Run the test, expect a compile error**

Run: `go test ./internal/compose/ -run TestAssembleMIMEAppendsSignature -v`

Expected: compile error — `AssembleMIME` doesn't take `idents` yet.

- [ ] **Step 2.4: Update `AssembleMIME`**

In `internal/compose/assemble.go`, change the function signature and append the signature:

```go
func AssembleMIME(d Draft, identities []Identity, now time.Time) ([]byte, error) {
	from := d.From
	if from.Address == "" {
		return nil, fmt.Errorf("compose: From address required")
	}

	body := d.Body
	if d.Signature >= 0 && d.Identity >= 0 && d.Identity < len(identities) {
		ident := identities[d.Identity]
		if d.Signature < len(ident.Signatures) {
			body = body + "\n\n" + ident.Signatures[d.Signature].Text
		}
	}

	htmlBody, err := filter.MarkdownToHTML([]byte(body))
	if err != nil {
		return nil, fmt.Errorf("compose: render html: %w", err)
	}

	// ...rest unchanged, but pass `body` to writeAlternative instead of d.Body...
```

Update the `writeAlternative` call to pass `body` rather than `d.Body`.

- [ ] **Step 2.5: Update existing call sites**

The callers of `AssembleMIME` are in `internal/cache/outbox*.go` (drainer dispatch) and `internal/ui/app.go` (compose send path). Find them and update:

```bash
grep -rn "AssembleMIME" --include='*.go'
```

For each call site that doesn't have access to the identities slice, pass `nil` and set `d.Signature = -1`. The minimal fix at this task is `nil, -1`; the proper threading lands in Task 9.

```go
mime, err := compose.AssembleMIME(d, nil, time.Now())
```

- [ ] **Step 2.6: Update `seed.go`**

In `internal/compose/seed.go`, find each of `SeedReply`, `SeedReplyAll`, `SeedForward`. Initialize the new fields:

```go
d.Identity = 0
d.Signature = 0  // caller will reset to -1 if the identity has no sigs
```

(If you don't have access to identities here, leave `Signature = 0`; the UI layer corrects it on Open.)

- [ ] **Step 2.7: Run the targeted test then the package**

```bash
go test ./internal/compose/ -run TestAssembleMIMEAppendsSignature -v
go test ./internal/compose/ -v
```

Both should PASS. If existing assemble tests break because of the new signature, update their call sites to pass `nil, time.Time` and set `Signature: -1` on the test drafts.

- [ ] **Step 2.8: Commit**

```bash
git add internal/compose/draft.go internal/compose/assemble.go internal/compose/seed.go internal/compose/assemble_test.go internal/cache/ internal/ui/app.go
git commit -m "Pass 9n: compose Draft.Identity/Signature + AssembleMIME

Draft carries identity + signature indices. AssembleMIME takes
the identities slice and appends the chosen signature's text to
the body before goldmark rendering. Signature == -1 means no sig."
```

---

## Task 3 — JMAP: identity cache keyed by email

**Files:**
- Modify: `internal/mailjmap/jmap.go`
- Test: `internal/mailjmap/jmap_test.go`

- [ ] **Step 3.1: Write the failing test**

In `internal/mailjmap/jmap_test.go`, after `TestSendUsesIdentityID` (or similar):

```go
func TestSendCachesIdentityIDByEmail(t *testing.T) {
	calls := 0
	identityResp := &jmap.Invocation{
		Name: "Identity/get",
		Args: &identity.GetResponse{
			List: []*identity.Identity{
				{ID: "id-1", Email: "alice@example.com"},
				{ID: "id-2", Email: "bob@example.com"},
			},
		},
	}
	// (rest of fake setup mirrors TestSendUsesIdentityID — see that test
	// for the boilerplate)
	b := newTestBackend(t, func(req *jmap.Request) (*jmap.Response, error) {
		calls++
		if hasInvocation(req, "Identity/get") {
			return fakeResponse(identityResp), nil
		}
		return fakeSendResponse(), nil
	})

	if err := b.Send(mail.Envelope{From: "alice@example.com", Rcpts: []string{"x@y"}}, []byte("Subject: hi\r\n\r\nbody")); err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := b.Send(mail.Envelope{From: "bob@example.com", Rcpts: []string{"x@y"}}, []byte("Subject: hi\r\n\r\nbody")); err != nil {
		t.Fatalf("second send: %v", err)
	}

	// Identity/get is issued once on the first miss and reuses cache
	// for both addresses (the response listed both). Expect:
	//   1. first Send: 1 Identity/get + 1 send batch = 2 round trips
	//   2. second Send: 0 Identity/get + 1 send batch = 1 round trip
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (1 identity probe + 2 sends)", calls)
	}
}
```

(`hasInvocation`, `fakeResponse`, `fakeSendResponse`, `newTestBackend` — model on the existing `TestSendUsesIdentityID` shape and extract any helpers if needed. Read `jmap_test.go:932-995` for the canonical pattern.)

- [ ] **Step 3.2: Run, expect FAIL**

Run: `go test ./internal/mailjmap/ -run TestSendCachesIdentityIDByEmail -v`

Expected: FAIL — the existing single-identity cache returns the first probe's ID for both calls.

- [ ] **Step 3.3: Replace the single-ID cache with a per-email map**

In `internal/mailjmap/jmap.go`, change the `Backend` struct:

```go
	// identityIDs caches Identity/get results by lowercased email.
	// Populated lazily on first Send for an unseen From address.
	identityIDs map[string]jmap.ID
```

Remove the `identityID jmap.ID` field. In the constructor, initialize the map:

```go
	b.identityIDs = make(map[string]jmap.ID)
```

Update `resolveIdentityID` to take the email and key by it:

```go
func (b *Backend) resolveIdentityID(accountID jmap.ID, email string) (jmap.ID, error) {
	b.mu.Lock()
	if id, ok := b.identityIDs[strings.ToLower(email)]; ok {
		b.mu.Unlock()
		return id, nil
	}
	b.mu.Unlock()

	// Probe Identity/get and populate the cache for every identity
	// returned (not just the requested one) so subsequent sends from
	// the same account hit the cache.
	req := &jmap.Request{}
	callID := req.Invoke(&identity.Get{Account: accountID})
	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("identity/get: %w", err)
	}
	for _, inv := range resp.MethodResponses {
		if inv.CallID != callID {
			continue
		}
		gr, ok := inv.Args.(*identity.GetResponse)
		if !ok {
			continue
		}
		b.mu.Lock()
		for _, id := range gr.List {
			b.identityIDs[strings.ToLower(id.Email)] = id.ID
		}
		got, ok := b.identityIDs[strings.ToLower(email)]
		b.mu.Unlock()
		if ok {
			return got, nil
		}
		return "", fmt.Errorf("identity/get: no identity for %q", email)
	}
	return "", fmt.Errorf("identity/get: empty response")
}
```

Update the caller in `Send`:

```go
	identityID, err := b.resolveIdentityID(accountID, env.From)
```

- [ ] **Step 3.4: Run the test**

Run: `go test ./internal/mailjmap/ -run TestSendCachesIdentityIDByEmail -v`

Expected: PASS.

- [ ] **Step 3.5: Run the full mailjmap suite**

Run: `go test ./internal/mailjmap/ -v`

Expected: PASS. The existing `TestSendUsesIdentityID` may need its expected `calls` count adjusted if the response listed only one identity (the new code still issues exactly one probe per cache miss).

- [ ] **Step 3.6: Commit**

```bash
git add internal/mailjmap/jmap.go internal/mailjmap/jmap_test.go
git commit -m "Pass 9n: JMAP identity cache keyed by email

Backend.identityID becomes identityIDs map[string]jmap.ID. One
Identity/get probe populates the cache for every identity the
server returns. Subsequent sends from the same account hit the
cache regardless of which identity is in From."
```

---

## Task 4 — UI compose: focusFrom focus state

**Files:**
- Modify: `internal/ui/compose/model.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 4.1: Write the failing test**

In `internal/ui/compose/model_test.go`:

```go
func TestComposeFocusOrderIncludesFrom(t *testing.T) {
	c := newTestModel(t)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "G", Email: "g@x", Signatures: []mailcompose.Signature{{Name: "s", Text: "-- \nG"}}},
	})
	c.SetSize(80, 24)

	// Tab from initial focus (To) should walk through all five fields
	// plus From in order: To → Cc → Bcc → Subject → Body → From → To.
	got := []int{c.Focus()}
	for i := 0; i < 6; i++ {
		c.Update(tea.KeyMsg{Type: tea.KeyTab})
		got = append(got, c.Focus())
	}
	want := []int{focusTo, focusCc, focusBcc, focusSubject, focusBody, focusFrom, focusTo}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("focus order = %v, want %v", got, want)
	}
}
```

`newTestModel`, `Focus()`, and `SetIdentities` may need to be added if missing. Add `Focus() int { return c.focus }` as an unexported test accessor in `model.go` if existing tests don't expose it. Add `SetIdentities`:

```go
// SetIdentities is the parent's wire-up after construction. The slice
// must be length >= 1 (callers synthesize from legacy From if config
// has no [[account.identity]] blocks). The first identity is the
// initial selection; the first signature within it (or -1) is the
// initial signature index.
func (c *Model) SetIdentities(ids []mailcompose.Identity) {
	c.identities = ids
	c.identity = 0
	if len(ids) > 0 && len(ids[0].Signatures) > 0 {
		c.signature = 0
	} else {
		c.signature = -1
	}
}
```

- [ ] **Step 4.2: Run, expect FAIL**

Run: `go test ./internal/ui/compose/ -run TestComposeFocusOrderIncludesFrom -v`

Expected: compile error (no `focusFrom` const, no `identities` field, no `SetIdentities`).

- [ ] **Step 4.3: Add `focusFrom` and the identity state**

In `internal/ui/compose/model.go`, change the focus enum so `focusFrom` exists *after* body in the cycle (Tab walks `To→Cc→Bcc→Subject→Body→From→To`; placing it at the end keeps `Esc` semantics — body↔subject — unchanged):

```go
const (
	focusTo = iota
	focusCc
	focusBcc
	focusSubject
	focusBody
	focusFrom
)
```

Add fields to `Model`:

```go
	identities []mailcompose.Identity
	identity   int // index into identities
	signature  int // index into identities[identity].Signatures, or -1
```

(Top of file: `compose "github.com/glw907/poplar/internal/compose"` should already be aliased as `mailcompose`. Use the existing alias.)

Update `advanceFocus` so the cycle includes `focusFrom`:

```go
func (c *Model) advanceFocus(d int) {
	const n = focusFrom + 1
	c.setFocus((c.focus + d + n) % n)
}
```

Update `setFocus` to handle Focus()/Blur() on text inputs and the new focusFrom state:

```go
func (c *Model) setFocus(f int) {
	c.to.Blur()
	c.cc.Blur()
	c.bcc.Blur()
	c.subject.Blur()
	c.focus = f
	switch f {
	case focusTo:
		c.to.Focus()
	case focusCc:
		c.cc.Focus()
	case focusBcc:
		c.bcc.Focus()
	case focusSubject:
		c.subject.Focus()
	case focusBody, focusFrom:
		// non-textinput; nothing to focus on bubbles side.
	}
}
```

(If `setFocus` doesn't exist with this exact shape, refactor the existing focus-setting logic into one.)

Add the test accessor:

```go
// Focus returns the current focus index. Test-only.
func (c *Model) Focus() int { return c.focus }
```

- [ ] **Step 4.4: Run the test**

Run: `go test ./internal/ui/compose/ -run TestComposeFocusOrderIncludesFrom -v`

Expected: PASS.

- [ ] **Step 4.5: Verify no other compose tests broke**

Run: `go test ./internal/ui/compose/ -v`

Expected: PASS for all. If `TestComposeEscToggle` or similar relies on focus ordering, audit and update.

- [ ] **Step 4.6: Commit**

```bash
git add internal/ui/compose/model.go internal/ui/compose/model_test.go
git commit -m "Pass 9n: compose adds focusFrom focus state

Tab cycle walks To→Cc→Bcc→Subject→Body→From→To. SetIdentities
wires the per-message identity + signature state from the parent."
```

---

## Task 5 — UI compose: From-row chip + cycler glyph rendering

**Files:**
- Modify: `internal/ui/compose/model.go`
- Modify: `internal/ui/compose/styles.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 5.1: Add chip and glyph styles**

In `internal/ui/compose/styles.go`, find the existing `Styles` struct constructor (`NewStyles`). Add fields:

```go
type Styles struct {
	// ...existing fields...
	FromChip     lipgloss.Style
	CyclerGlyph  lipgloss.Style
}
```

In `NewStyles`:

```go
	s.FromChip = lipgloss.NewStyle().Foreground(t.MutedFg).Background(t.BgBase)
	s.CyclerGlyph = lipgloss.NewStyle().Foreground(t.MutedFg).Background(t.BgBase)
```

(If the theme palette uses `FgDim` rather than `MutedFg`, use that — match `attribution`/`signature` styling.)

- [ ] **Step 5.2: Write the failing test**

```go
func TestComposeFromRowRendersChip(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(80, 24)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "Geoff Wright", Email: "geoff@907.life",
			Signatures: []mailcompose.Signature{
				{Name: "default", Text: "-- \nG"},
				{Name: "casual", Text: "-- \ng"},
			}},
	})

	view := stripANSI(c.View())

	// Initial chip: "Geoff Wright <geoff@907.life> · sig: default"
	if !strings.Contains(view, "Geoff Wright <geoff@907.life>") {
		t.Errorf("From row missing identity:\n%s", view)
	}
	if !strings.Contains(view, "· sig: default") {
		t.Errorf("From row missing chip:\n%s", view)
	}

	// Empty-signatures identity → no chip.
	c.SetIdentities([]mailcompose.Identity{
		{Name: "G", Email: "g@x"},
	})
	view = stripANSI(c.View())
	if strings.Contains(view, "· sig:") {
		t.Errorf("expected no chip for zero-sig identity, got:\n%s", view)
	}

	// Signature == -1 → "no sig" chip when sigs exist.
	c.SetIdentities([]mailcompose.Identity{
		{Name: "G", Email: "g@x", Signatures: []mailcompose.Signature{{Name: "x", Text: "-- \nx"}}},
	})
	c.SetSignature(-1)
	view = stripANSI(c.View())
	if !strings.Contains(view, "· no sig") {
		t.Errorf("expected '· no sig' chip, got:\n%s", view)
	}
}
```

`stripANSI` is a helper present in the package's test file (or add one based on `ansi.Strip`). Add `SetSignature(int)` for tests:

```go
func (c *Model) SetSignature(idx int) { c.signature = idx }
```

- [ ] **Step 5.3: Run, expect FAIL**

Run: `go test ./internal/ui/compose/ -run TestComposeFromRowRendersChip -v`

Expected: FAIL — the From row currently renders only `c.from`.

- [ ] **Step 5.4: Implement chip rendering**

In `model.go`, find where the From row is rendered (around line 226: `rows = append(rows, c.headerRow("From:", c.from))`). Replace the static `c.from` with a computed value:

```go
	rows = append(rows, c.headerRow("From:", c.fromValue()))
```

Add the helper:

```go
// fromValue renders the From: cell: identity address plus the sig
// chip (or "no sig" / absent) plus the cycler glyph when focused.
func (c *Model) fromValue() string {
	if len(c.identities) == 0 {
		return c.from
	}
	id := c.identities[c.identity]
	addr := id.Name + " <" + id.Email + ">"
	chip := ""
	switch {
	case len(id.Signatures) == 0:
		// no chip
	case c.signature < 0:
		chip = c.styles.FromChip.Render(" · no sig")
	case c.signature < len(id.Signatures):
		chip = c.styles.FromChip.Render(" · sig: " + id.Signatures[c.signature].Name)
	}
	glyph := ""
	if c.focus == focusFrom && (len(c.identities) > 1 || len(id.Signatures) > 0) {
		glyph = c.styles.CyclerGlyph.Render(" ‹ ›")
	}
	return addr + chip + glyph
}
```

- [ ] **Step 5.5: Run the test**

Run: `go test ./internal/ui/compose/ -run TestComposeFromRowRendersChip -v`

Expected: PASS.

- [ ] **Step 5.6: Commit**

```bash
git add internal/ui/compose/model.go internal/ui/compose/styles.go internal/ui/compose/model_test.go
git commit -m "Pass 9n: compose From row renders identity + sig chip

From: cell renders 'Name <email> · sig: <name>' with the chip
suppressed when the identity has zero signatures and replaced by
'· no sig' when Signature == -1. Cycler glyph appears on focus."
```

---

## Task 6 — UI compose: identity cycling keys

**Files:**
- Modify: `internal/ui/compose/model.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 6.1: Failing test**

```go
func TestComposeIdentityCycle(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(80, 24)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "A", Email: "a@x", Signatures: []mailcompose.Signature{{Name: "s1", Text: "-- \n1"}}},
		{Name: "B", Email: "b@x"}, // no sigs
		{Name: "C", Email: "c@x", Signatures: []mailcompose.Signature{{Name: "s3", Text: "-- \n3"}}},
	})
	// Tab to focusFrom (last in cycle).
	for c.Focus() != focusFrom {
		c.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Space → next identity.
	c.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.Identity() != 1 {
		t.Errorf("after Space: identity = %d, want 1", c.Identity())
	}
	// B has no sigs → Signature must reset to -1.
	if c.Signature() != -1 {
		t.Errorf("after Space onto B: signature = %d, want -1", c.Signature())
	}

	// Space again → C; Signature resets to 0 (C has sigs).
	c.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.Identity() != 2 || c.Signature() != 0 {
		t.Errorf("after second Space: identity = %d, signature = %d, want 2, 0", c.Identity(), c.Signature())
	}

	// Wrap forward.
	c.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.Identity() != 0 {
		t.Errorf("after wrap: identity = %d, want 0", c.Identity())
	}

	// Left arrow → previous (wrap to 2).
	c.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if c.Identity() != 2 {
		t.Errorf("after Left wrap: identity = %d, want 2", c.Identity())
	}
}
```

Add test accessors:

```go
func (c *Model) Identity() int  { return c.identity }
func (c *Model) Signature() int { return c.signature }
```

- [ ] **Step 6.2: Run, expect FAIL**

Run: `go test ./internal/ui/compose/ -run TestComposeIdentityCycle -v` — Expected: FAIL.

- [ ] **Step 6.3: Add the cycler in `Update`**

In `Update`, find the focused-key dispatch (around line 397, the `switch c.focus { ... }` block). Add a new case before the existing ones:

```go
	case focusFrom:
		if k, ok := msg.(tea.KeyMsg); ok {
			switch k.Type {
			case tea.KeySpace, tea.KeyRight:
				c.cycleIdentity(+1)
				return c, nil
			case tea.KeyLeft:
				c.cycleIdentity(-1)
				return c, nil
			case tea.KeyRunes:
				switch string(k.Runes) {
				case "l":
					c.cycleIdentity(+1)
					return c, nil
				case "h":
					c.cycleIdentity(-1)
					return c, nil
				}
			}
		}
```

Add `cycleIdentity`:

```go
func (c *Model) cycleIdentity(d int) {
	n := len(c.identities)
	if n <= 1 {
		return
	}
	c.identity = (c.identity + d + n) % n
	if len(c.identities[c.identity].Signatures) > 0 {
		c.signature = 0
	} else {
		c.signature = -1
	}
}
```

- [ ] **Step 6.4: Run the test**

Run: `go test ./internal/ui/compose/ -run TestComposeIdentityCycle -v` — Expected: PASS.

- [ ] **Step 6.5: Commit**

```bash
git add internal/ui/compose/model.go internal/ui/compose/model_test.go
git commit -m "Pass 9n: compose identity cycler in focusFrom

Space/→/l next, ←/h previous; Signature resets to 0 (or -1 if the
new identity has no sigs). Inert when len(identities) <= 1."
```

---

## Task 7 — UI compose: signature cycling keys (g and Ctrl+G)

**Files:**
- Modify: `internal/ui/compose/model.go`
- Test: `internal/ui/compose/model_test.go`

- [ ] **Step 7.1: Failing test**

```go
func TestComposeSignatureCycle(t *testing.T) {
	c := newTestModel(t)
	c.SetSize(80, 24)
	c.SetIdentities([]mailcompose.Identity{
		{Name: "A", Email: "a@x", Signatures: []mailcompose.Signature{
			{Name: "s1", Text: "-- \n1"},
			{Name: "s2", Text: "-- \n2"},
		}},
	})

	// Body has focus initially after Tab walk; navigate to Body.
	for c.Focus() != focusBody {
		c.Update(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Ctrl+G in Body should cycle: 0 → 1 → -1 → 0.
	steps := []struct {
		want int
	}{{1}, {-1}, {0}}
	for _, st := range steps {
		c.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
		if c.Signature() != st.want {
			t.Errorf("after Ctrl+G: signature = %d, want %d", c.Signature(), st.want)
		}
	}

	// Tab to focusFrom; bare 'g' should cycle.
	for c.Focus() != focusFrom {
		c.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if c.Signature() != 1 {
		t.Errorf("after 'g' in focusFrom: signature = %d, want 1", c.Signature())
	}

	// Inert when identity has zero sigs.
	c.SetIdentities([]mailcompose.Identity{{Name: "B", Email: "b@x"}})
	c.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if c.Signature() != -1 {
		t.Errorf("expected signature stays -1, got %d", c.Signature())
	}
}
```

- [ ] **Step 7.2: Run, expect FAIL**

Run: `go test ./internal/ui/compose/ -run TestComposeSignatureCycle -v` — Expected: FAIL.

- [ ] **Step 7.3: Implement Ctrl+G handler at the top-level switch**

In `Update`, in the existing `switch msg.Type` block (around line 371) that handles `KeyCtrlX`/`KeyCtrlC`/`KeyTab`/`KeyShiftTab`/`KeyEsc`, add:

```go
		case tea.KeyCtrlG:
			c.cycleSignature()
			return c, nil
```

Then in the `case focusFrom:` block from Task 6, add bare-letter `g`:

```go
				case "g":
					c.cycleSignature()
					return c, nil
```

Add `cycleSignature`:

```go
func (c *Model) cycleSignature() {
	if len(c.identities) == 0 {
		return
	}
	sigs := c.identities[c.identity].Signatures
	if len(sigs) == 0 {
		return
	}
	switch {
	case c.signature < 0:
		c.signature = 0
	case c.signature == len(sigs)-1:
		c.signature = -1
	default:
		c.signature++
	}
}
```

- [ ] **Step 7.4: Run the test**

Run: `go test ./internal/ui/compose/ -run TestComposeSignatureCycle -v` — Expected: PASS.

- [ ] **Step 7.5: Run the full compose-UI suite**

Run: `go test ./internal/ui/compose/ -v`

Expected: PASS. Watch for two regressions: (a) `Ctrl+G` previously typed nothing but is now handled — make sure Catkin doesn't bind it (per catkin-invariants, Catkin uses `Ctrl+B/I/K/L/Q/Space`; `Ctrl+G` is free); (b) `g` typed in body should still type into the editor — verify it does (the dispatch falls through to `c.editor` because the bare-letter handler is gated on `case focusFrom`).

- [ ] **Step 7.6: Commit**

```bash
git add internal/ui/compose/model.go internal/ui/compose/model_test.go
git commit -m "Pass 9n: compose signature cycler

Ctrl+G cycles signatures from any compose focus state including
focusFrom; bare 'g' is the focusFrom-only convenience. Cycle order
is 0 → 1 → … → N-1 → -1 (none) → 0. Inert when the identity has
no signatures."
```

---

## Task 8 — UI compose: footer hints

**Files:**
- Modify: `internal/ui/footer.go`
- Modify: `internal/ui/app.go` (footer dispatcher)
- Test: `internal/ui/footer_test.go`

- [ ] **Step 8.1: Identify the compose footer surface**

Read `internal/ui/footer.go` end-to-end and the dispatcher in `internal/ui/app.go` that picks the footer group by context. There is no `composeFooterGroups` today; the compose surface uses the account footer or a synthetic one. Find the call site by searching:

```bash
grep -rn "FooterContext\|footerGroups\|composeOpen" internal/ui/ --include='*.go'
```

If the compose-open path falls back to `accountFooterGroups`, add a new context. If a `compose: true` branch already exists, augment it.

- [ ] **Step 8.2: Add `composeFooterGroups`**

In `footer.go`:

```go
// composeFooterGroups returns the compose-mode footer hint groups.
// hasSig and isFocusFrom select which sig and identity hints render.
func composeFooterGroups(hasSig, isFocusFrom bool) [][]footerHint {
	core := []footerHint{
		hint("Ctrl+X", "send", 0),
		hint("Ctrl+C", "cancel", 0),
		hint("Tab", "field", 4),
	}
	if hasSig {
		core = append(core, hint("Ctrl+G", "sig", 5))
	}
	groups := [][]footerHint{core}
	if isFocusFrom {
		groups = append(groups, []footerHint{
			hint("Space/←→", "identity", 6),
			hint("Ctrl+G", "sig", 6),
		})
	}
	return groups
}
```

- [ ] **Step 8.3: Failing test**

In `internal/ui/footer_test.go`:

```go
func TestComposeFooterIncludesSigHint(t *testing.T) {
	g := composeFooterGroups(true, false)
	if !containsHint(g, "Ctrl+G", "sig") {
		t.Errorf("compose footer missing Ctrl+G sig:\n%v", g)
	}
}

func TestComposeFooterOmitsSigWhenNoSigs(t *testing.T) {
	g := composeFooterGroups(false, false)
	if containsHint(g, "Ctrl+G", "sig") {
		t.Errorf("compose footer included Ctrl+G sig with no sigs:\n%v", g)
	}
}

func TestComposeFooterAddsFromHintsOnFocusFrom(t *testing.T) {
	g := composeFooterGroups(true, true)
	if !containsHint(g, "Space/←→", "identity") {
		t.Errorf("focusFrom footer missing identity hint:\n%v", g)
	}
}

// containsHint scans groups for a (key, desc) pair.
func containsHint(groups [][]footerHint, key, desc string) bool {
	for _, g := range groups {
		for _, h := range g {
			if h.key == key && h.desc == desc {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 8.4: Run the test**

Run: `go test ./internal/ui/ -run TestComposeFooter -v` — Expected: PASS (after Step 8.2 lands).

- [ ] **Step 8.5: Wire into the dispatcher**

In `internal/ui/footer.go` `View()` method (around line 144), add a branch in the `groups = ...` selection so compose mode picks the new groups. Pass `m.compose != nil` along with `m.compose.HasSignatures()` and `m.compose.Focus() == focusFrom` from app state:

In `internal/ui/compose/model.go`, add accessors:

```go
func (c *Model) HasSignatures() bool {
	return len(c.identities) > 0 && len(c.identities[c.identity].Signatures) > 0
}

func (c *Model) IsFocusFrom() bool { return c.focus == focusFrom }
```

In `app.go` find where `FooterContext` is set or where `View()` calls `footer.View(...)`. Update the dispatch so when `m.compose != nil`, use `composeFooterGroups(m.compose.HasSignatures(), m.compose.IsFocusFrom())`.

- [ ] **Step 8.6: Verify in tmux**

```bash
make install
# In a fresh terminal, run poplar against the test fastmail account
# (FASTMAIL_API_TOKEN exported), open compose with `c`, observe:
# - Ctrl+G sig appears in footer
# - Tab to From, Space/←→ identity appears
# - Cycle keys update the chip
poplar
```

(See `.claude/docs/tmux-testing.md` if you need to script this for capture.)

- [ ] **Step 8.7: Commit**

```bash
git add internal/ui/footer.go internal/ui/footer_test.go internal/ui/compose/model.go internal/ui/app.go
git commit -m "Pass 9n: compose footer hints for sig + identity

composeFooterGroups gates 'Ctrl+G sig' on the active identity
having signatures, and surfaces 'Space/←→ identity' when From
has focus. The chord is shown even on focusFrom (bare 'g' is the
quiet variant)."
```

---

## Task 9 — App wiring: thread identities into compose

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/compose/model.go` (constructor signatures if needed)
- Modify: any cache outbox dispatch that calls `AssembleMIME`

- [ ] **Step 9.1: Convert `[]config.Identity` to `[]mailcompose.Identity`**

`internal/compose` defines its own `Identity` and `Signature` (Task 2). Add an adapter — either a one-off conversion in `app.go` or a helper in `internal/ui/compose/`:

In `internal/ui/compose/model.go` (or a new `bind.go`):

```go
// IdentitiesFromConfig converts the config-side identity slice into
// the compose-side mirror. Pure value copy.
func IdentitiesFromConfig(in []config.Identity) []mailcompose.Identity {
	out := make([]mailcompose.Identity, len(in))
	for i, ci := range in {
		sigs := make([]mailcompose.Signature, len(ci.Signatures))
		for j, cs := range ci.Signatures {
			sigs[j] = mailcompose.Signature{Name: cs.Name, Text: cs.Text}
		}
		out[i] = mailcompose.Identity{Name: ci.Name, Email: ci.Email, Signatures: sigs}
	}
	return out
}
```

- [ ] **Step 9.2: Wire `SetIdentities` at compose-open**

In `internal/ui/app.go`, find every `uicompose.New(...)` and `uicompose.Open(...)` call. After construction, call:

```go
m.compose.SetIdentities(uicompose.IdentitiesFromConfig(m.acct.Identities()))
```

This requires `m.acct` to expose `Identities() []config.Identity`. If `m.acct` is `*cache.Account`, add an `Identities() []config.Identity` accessor — store the slice on construction in `cache.NewAccount` (or wherever the config flows in).

If `m.acct` already has access to its `AccountConfig`, just pull `cfg.Identities`.

- [ ] **Step 9.3: Wire identities into the outbox AssembleMIME call**

In the outbox drainer dispatch path (find via `grep -rn "AssembleMIME" --include='*.go'`), the assembled MIME currently happens at queue time, not at dispatch (per ADR-0160 / spec §Outbox). So `AssembleMIME` is called at compose-send. Confirm by inspecting `internal/ui/app.go` for the send dispatch and `internal/cache/` for any direct call.

When you find the call, replace the `nil, -1` Task 2 placeholder:

```go
mime, err := compose.AssembleMIME(d, uicompose.IdentitiesFromConfig(m.acct.Identities()), time.Now())
```

(Or pass the local `mailcompose.Identity` slice if the caller has already converted.)

- [ ] **Step 9.4: Run the full suite**

```bash
make check
```

Expected: PASS. If `voice-check.sh` flags new comments, audit and fix.

- [ ] **Step 9.5: Live verification (tmux, 80×24 + 120×40)**

Per `docs/poplar/bubbletea-conventions.md` §10 Review checklist:

```bash
make install
# 80x24 capture
tmux new-session -d -s 9n80 -x 80 -y 24 'poplar'
sleep 1
tmux send-keys -t 9n80 c
sleep 1
tmux capture-pane -t 9n80 -p > /tmp/9n-compose-80x24.txt

# 120x40 capture
tmux new-session -d -s 9n120 -x 120 -y 40 'poplar'
sleep 1
tmux send-keys -t 9n120 c
sleep 1
tmux capture-pane -t 9n120 -p > /tmp/9n-compose-120x40.txt
```

Inspect both: From row renders identity + chip, Tab walks through focusFrom, footer drops `Ctrl+G sig` first under width pressure.

- [ ] **Step 9.6: Add a temporary `[[account.identity]]` block to the test config**

To exercise the multi-identity path against a live Fastmail account, add to `~/.config/poplar/config.toml`:

```toml
[[account.identity]]
name = "Geoff Wright"
email = "geoff@907.life"

[[account.identity.signature]]
name = "default"
text = "Geoff Wright"
```

Verify in compose: chip reads `· sig: default`, Tab to From, `Space` is inert (only one identity), `g` cycles to `· no sig` and back.

- [ ] **Step 9.7: Commit**

```bash
git add internal/ui/app.go internal/ui/compose/ internal/cache/
git commit -m "Pass 9n: wire identities into compose

App pulls AccountConfig.Identities and threads it into compose
on Open/New. AssembleMIME at send time uses the same slice. End-
to-end: Tab → From → cycle identity / sig → Ctrl+X sends with the
chosen identity in From and the chosen signature appended."
```

---

## Pass-end consolidation

After Task 9 lands, invoke the `poplar-pass` skill for the consolidation ritual:

- `/simplify` on the diff.
- §10 idiomatic-bubbletea review checklist (the live tmux captures from Task 9.5 cover the size verification).
- Write **ADR-0177** at `docs/poplar/decisions/0177-identities-and-signatures.md` per the spec's *ADR* section.
- Update `docs/poplar/invariants.md` per the spec's *Invariants delta* section: replace the JMAP single-identity cache fact with the per-email map; add the `[[account.identity]]` decode rules to *Config & theming*; the reader sentinel-dim fact is already correct (`internal/content` + `theme.Signature` predates this pass).
- Update `docs/poplar/decisions/INDEX.md` with an ADR-0177 row.
- Update `STATUS.md`: mark 9n done; replace the starter prompt with Pass 9o (Claude Tidy implementation).
- `git mv` this plan and the spec into `docs/superpowers/archive/plans/` and `docs/superpowers/archive/specs/`.
- `make check` → commit → push → `make install`.
