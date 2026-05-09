# List-Unsubscribe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface RFC 8058 one-click unsubscribe in the viewer for messages carrying `List-Unsubscribe` and `List-Unsubscribe-Post: List-Unsubscribe=One-Click`. Fall back to mailto-into-compose when no one-click endpoint exists; route plain http: through the existing `URLOpener` seam.

**Architecture:** Pure header parser in `internal/content/`; parse runs in the existing body-fetch Cmd; parsed result rides back on `reader.BodyLoadedMsg` via a new `Unsub` field. Reader's `U` key emits `OpenUnsubscribeConfirmMsg`; App opens `ConfirmModal` and on Yes routes by precedence (one-click POST → mailto compose seed → plain http via URLOpener). Success surfaces as a one-shot `Unsubscribed from <host>` notice in the chrome banner row.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, stdlib `net/http` + `net/textproto` + `net/url` + `net/mail`. No new third-party deps.

**Spec:** `docs/superpowers/specs/2026-05-09-list-unsubscribe-design.md`

**Pass-end ritual:** Standard. Invoke `poplar-pass` skill at the end (ADR-0185, invariants edits, plan/spec archive, `make check`, commit/push/install).

---

### Plan deviation note (read first)

The spec proposed a sibling `UnsubscribeLoadedMsg` batched with `BodyLoadedMsg`. Implementing that requires returning two Msgs from one Cmd, which bubbletea doesn't support cleanly. The plan extends `reader.BodyLoadedMsg` with an `Unsub content.Unsubscribe` field instead — equivalent semantics, smaller diff (3 production callers + 3 test callers updated), no new Msg type. The spec note about "keeping shape stable for downstream consumers" overstated the cost; downstream surface is small.

---

### Task 1: Header parser — value type and stub

**Files:**
- Create: `internal/content/listunsubscribe.go`

- [ ] **Step 1: Write the value type and stub**

```go
package content

import (
	"net/textproto"
)

// Unsubscribe carries the parsed result of RFC 2369 List-Unsubscribe
// and RFC 8058 List-Unsubscribe-Post headers. Available reports
// whether at least one actionable form is present.
type Unsubscribe struct {
	// OneClick is the https URL eligible for an RFC 8058 one-click
	// POST. Set only when List-Unsubscribe-Post advertises
	// List-Unsubscribe=One-Click and at least one https URL is
	// present in List-Unsubscribe. Empty otherwise.
	OneClick string

	// Mailto is the first mailto: URL from List-Unsubscribe, "" when
	// none is present.
	Mailto string

	// HTTP is the first http(s) URL from List-Unsubscribe when not
	// promoted to OneClick. Used as the URLOpener fallback.
	HTTP string
}

// Available reports whether any actionable form is set.
func (u Unsubscribe) Available() bool {
	return u.OneClick != "" || u.Mailto != "" || u.HTTP != ""
}

// ParseListUnsubscribe parses RFC 2369 List-Unsubscribe and RFC 8058
// List-Unsubscribe-Post headers from h and returns the resolved
// Unsubscribe value. Tolerates whitespace, missing brackets, and
// mixed schemes. Only https URLs are eligible for OneClick promotion
// (RFC 8058 §3); http URLs always land in HTTP.
func ParseListUnsubscribe(h textproto.MIMEHeader) Unsubscribe {
	return Unsubscribe{}
}
```

- [ ] **Step 2: Commit the stub**

```bash
git add internal/content/listunsubscribe.go
git commit -m "Pass 11: List-Unsubscribe value type stub"
```

---

### Task 2: Header parser — failing tests

**Files:**
- Create: `internal/content/listunsubscribe_test.go`

- [ ] **Step 1: Write the table-driven test**

```go
package content

import (
	"net/textproto"
	"testing"
)

func TestParseListUnsubscribe(t *testing.T) {
	cases := []struct {
		name string
		hdrs map[string][]string
		want Unsubscribe
	}{
		{
			name: "absent",
			hdrs: map[string][]string{},
			want: Unsubscribe{},
		},
		{
			name: "mailto only",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"<mailto:unsub@list.example>"},
			},
			want: Unsubscribe{Mailto: "mailto:unsub@list.example"},
		},
		{
			name: "https without post header",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"<https://list.example/unsub?id=42>"},
			},
			want: Unsubscribe{HTTP: "https://list.example/unsub?id=42"},
		},
		{
			name: "https with one-click post",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{OneClick: "https://list.example/u?id=42"},
		},
		{
			name: "https + mailto with one-click post (RFC 8058 canonical)",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>, <mailto:u@list.example?subject=unsub>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{
				OneClick: "https://list.example/u?id=42",
				Mailto:   "mailto:u@list.example?subject=unsub",
			},
		},
		{
			name: "http (non-TLS) does not promote",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<http://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{HTTP: "http://list.example/u?id=42"},
		},
		{
			name: "post header with non-one-click value",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=Confirm"},
			},
			want: Unsubscribe{HTTP: "https://list.example/u?id=42"},
		},
		{
			name: "case-insensitive post key",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://list.example/u?id=42>"},
				"List-Unsubscribe-Post": {"list-unsubscribe=One-Click"},
			},
			want: Unsubscribe{OneClick: "https://list.example/u?id=42"},
		},
		{
			name: "missing brackets tolerated",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"mailto:u@list.example, https://list.example/u"},
			},
			want: Unsubscribe{
				Mailto: "mailto:u@list.example",
				HTTP:   "https://list.example/u",
			},
		},
		{
			name: "multiple https — first wins for HTTP",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"<https://a.example/u>, <https://b.example/u>"},
			},
			want: Unsubscribe{HTTP: "https://a.example/u"},
		},
		{
			name: "multiple https — first promoted to OneClick",
			hdrs: map[string][]string{
				"List-Unsubscribe":      {"<https://a.example/u>, <https://b.example/u>"},
				"List-Unsubscribe-Post": {"List-Unsubscribe=One-Click"},
			},
			want: Unsubscribe{OneClick: "https://a.example/u"},
		},
		{
			name: "garbage values ignored",
			hdrs: map[string][]string{
				"List-Unsubscribe": {"   <not-a-uri>, , <ftp://x.example/y>  "},
			},
			want: Unsubscribe{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := textproto.MIMEHeader{}
			for k, vs := range tc.hdrs {
				for _, v := range vs {
					h.Add(k, v)
				}
			}
			got := ParseListUnsubscribe(h)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestUnsubscribeAvailable(t *testing.T) {
	cases := []struct {
		name string
		u    Unsubscribe
		want bool
	}{
		{"empty", Unsubscribe{}, false},
		{"oneclick", Unsubscribe{OneClick: "https://x"}, true},
		{"mailto", Unsubscribe{Mailto: "mailto:x"}, true},
		{"http", Unsubscribe{HTTP: "https://x"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.u.Available(); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/content/ -run ListUnsubscribe -v
```

Expected: every `TestParseListUnsubscribe/*` case FAILs (parser returns zero value).

---

### Task 3: Header parser — implementation

**Files:**
- Modify: `internal/content/listunsubscribe.go`

- [ ] **Step 1: Replace the stub**

```go
package content

import (
	"net/textproto"
	"strings"
)

type Unsubscribe struct {
	OneClick string
	Mailto   string
	HTTP     string
}

func (u Unsubscribe) Available() bool {
	return u.OneClick != "" || u.Mailto != "" || u.HTTP != ""
}

// ParseListUnsubscribe parses RFC 2369 List-Unsubscribe and RFC 8058
// List-Unsubscribe-Post headers and returns the resolved value.
func ParseListUnsubscribe(h textproto.MIMEHeader) Unsubscribe {
	raw := h.Get("List-Unsubscribe")
	if raw == "" {
		return Unsubscribe{}
	}

	var mailto, httpURL string
	httpsURLs := make([]string, 0, 2)
	for _, entry := range splitListUnsubscribe(raw) {
		switch {
		case strings.HasPrefix(entry, "mailto:"):
			if mailto == "" {
				mailto = entry
			}
		case strings.HasPrefix(entry, "https://"):
			httpsURLs = append(httpsURLs, entry)
			if httpURL == "" {
				httpURL = entry
			}
		case strings.HasPrefix(entry, "http://"):
			if httpURL == "" {
				httpURL = entry
			}
		}
	}

	u := Unsubscribe{Mailto: mailto, HTTP: httpURL}

	if len(httpsURLs) > 0 && hasOneClickPost(h.Get("List-Unsubscribe-Post")) {
		u.OneClick = httpsURLs[0]
		// Promotion consumes the URL from HTTP so callers don't double-route.
		if u.HTTP == u.OneClick {
			u.HTTP = ""
		}
	}

	return u
}

// splitListUnsubscribe tokenizes a comma-separated List-Unsubscribe
// value into individual URIs, stripping enclosing angle brackets and
// surrounding whitespace. Tolerates missing brackets.
func splitListUnsubscribe(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "<")
		p = strings.TrimSuffix(p, ">")
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// hasOneClickPost reports whether the List-Unsubscribe-Post header
// value advertises RFC 8058 one-click. The key is matched
// case-insensitively per RFC 8058 §3; the value comparison is
// likewise case-insensitive.
func hasOneClickPost(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	eq := strings.IndexByte(v, '=')
	if eq < 0 {
		return false
	}
	key := strings.TrimSpace(v[:eq])
	val := strings.TrimSpace(v[eq+1:])
	return strings.EqualFold(key, "List-Unsubscribe") &&
		strings.EqualFold(val, "One-Click")
}
```

- [ ] **Step 2: Run tests to verify they pass**

```bash
go test ./internal/content/ -run ListUnsubscribe -v
```

Expected: all subtests PASS.

- [ ] **Step 3: Run full content package tests**

```bash
go test ./internal/content/
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/content/listunsubscribe.go internal/content/listunsubscribe_test.go
git commit -m "Pass 11: List-Unsubscribe header parser"
```

---

### Task 4: Extend BodyLoadedMsg, parse from raw header block

**Files:**
- Modify: `internal/ui/reader/msgs.go`
- Modify: `internal/ui/account/cmds.go`

- [ ] **Step 1: Add Unsub field to BodyLoadedMsg**

In `internal/ui/reader/msgs.go`, replace the existing `BodyLoadedMsg` declaration:

```go
// BodyLoadedMsg carries the parsed-block body and any List-Unsubscribe
// headers harvested from the raw RFC 5322 envelope. AccountTab drops
// mismatches against the viewer's current UID (user closed and
// reopened on a different UID before the Cmd resolved).
type BodyLoadedMsg struct {
	UID    mail.UID
	Blocks []content.Block
	Unsub  content.Unsubscribe
}
```

- [ ] **Step 2: Parse List-Unsubscribe in loadBodyCmd**

In `internal/ui/account/cmds.go`, inside `loadBodyCmd` (around line 174–232): parse the raw header block before `gomail.CreateReader` consumes it. Add this near the top of the goroutine, after the `c.FetchBody` call returns successfully:

```go
unsub := parseUnsubscribeFromRaw(buf)
```

Then update the message emission at the bottom of the goroutine (line 224):

```go
resultCh <- reader.BodyLoadedMsg{
	UID:    uid,
	Blocks: content.ParseBlocks(text),
	Unsub:  unsub,
}
```

- [ ] **Step 3: Add parseUnsubscribeFromRaw helper**

Append to `internal/ui/account/cmds.go` (alongside `isRFC822`):

```go
// parseUnsubscribeFromRaw extracts the RFC 5322 header block from buf
// and returns parsed List-Unsubscribe data. Non-RFC822 input (mock-
// backend pre-cleaned markdown) parses to a zero Unsubscribe.
func parseUnsubscribeFromRaw(buf []byte) content.Unsubscribe {
	if !isRFC822(buf) {
		return content.Unsubscribe{}
	}
	r := textproto.NewReader(bufio.NewReader(bytes.NewReader(buf)))
	h, err := r.ReadMIMEHeader()
	if err != nil {
		return content.Unsubscribe{}
	}
	return content.ParseListUnsubscribe(h)
}
```

Add the imports if not already present: `bufio`, `bytes`, `net/textproto`, and the `content` package (already imported).

- [ ] **Step 4: Update existing test callers**

Three test files reference `BodyLoadedMsg` literals: update them to include the new (zero) field implicitly. Go's struct literal with named fields handles this without changes — verify:

```bash
go vet ./...
go test ./internal/ui/...
```

Expected: PASS. (Existing tests use named-field literals like `reader.BodyLoadedMsg{UID: ..., Blocks: ...}` so the new field defaults to zero.)

- [ ] **Step 5: Commit**

```bash
git add internal/ui/reader/msgs.go internal/ui/account/cmds.go
git commit -m "Pass 11: parse List-Unsubscribe in body-fetch Cmd"
```

---

### Task 5: Reader U key + accessor

**Files:**
- Modify: `internal/ui/reader/model.go`
- Modify: `internal/ui/reader/msgs.go`

- [ ] **Step 1: Add msg type for the confirm trigger**

Append to `internal/ui/reader/msgs.go`:

```go
// OpenUnsubscribeConfirmMsg asks App to open ConfirmModal for the
// list-unsubscribe action on the viewer's current message.
type OpenUnsubscribeConfirmMsg struct {
	UID   mail.UID
	Unsub content.Unsubscribe
}
```

- [ ] **Step 2: Add unsub field, accessor, and key binding**

In `internal/ui/reader/model.go`:

In the `viewerKeys` struct (around line 33), add a `Unsubscribe` field:

```go
type viewerKeys struct {
	Close            key.Binding
	OpenPicker       key.Binding
	OpenAttachPicker key.Binding
	BodyTop          key.Binding
	BodyBottom       key.Binding
	Unsubscribe      key.Binding
	Links            [9]key.Binding
}
```

In `newViewerKeys` (around line 42), add the binding:

```go
vk := viewerKeys{
	Close:            key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q/esc", "close")),
	OpenPicker:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "links")),
	OpenAttachPicker: key.NewBinding(key.WithKeys("@"), key.WithHelp("@", "attachments")),
	BodyTop:          key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top of body")),
	BodyBottom:       key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom of body")),
	Unsubscribe:      key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "unsubscribe")),
}
```

In the `Model` struct (around line 61), add the field:

```go
	unsub        content.Unsubscribe
```

- [ ] **Step 3: Set unsub on body load**

Update `SetBody` (around line 133) to take the unsub value:

```go
// SetBody installs parsed blocks plus the harvested List-Unsubscribe
// data and transitions to ready. Callers must drop BodyLoadedMsg
// with a UID mismatch before invoking.
func (v Model) SetBody(blocks []content.Block, unsub content.Unsubscribe) Model {
	v.blocks = blocks
	v.unsub = unsub
	v.phase = PhaseReady
	v.layout()
	return v
}

// Unsubscribe returns the harvested List-Unsubscribe data for the
// current message. Zero when the viewer is closed or the message
// has no List-Unsubscribe headers.
func (v Model) Unsubscribe() content.Unsubscribe { return v.unsub }
```

Update `Open` (around line 112) to clear `unsub`:

```go
func (v Model) Open(msg mail.MessageInfo) Model {
	v.open = true
	v.phase = PhaseLoading
	v.msg = msg
	v.blocks = nil
	v.links = nil
	v.attachments = nil
	v.chipRow = ""
	v.chipHeight = 0
	v.panel = ""
	v.unsub = content.Unsubscribe{}
	return v
}
```

- [ ] **Step 4: Wire the U key**

In `handleKey` (around line 198), add a case before the link-digit loop:

```go
	case key.Matches(msg, v.keys.Unsubscribe):
		if !v.unsub.Available() {
			return v, nil
		}
		uid := v.msg.UID
		unsub := v.unsub
		return v, func() tea.Msg {
			return OpenUnsubscribeConfirmMsg{UID: uid, Unsub: unsub}
		}
```

- [ ] **Step 5: Update SetBody caller in account/model.go**

In `internal/ui/account/model.go`, update the `BodyLoadedMsg` handler (around line 203):

```go
	case reader.BodyLoadedMsg:
		if m.viewer.CurrentUID() == msg.UID {
			m.bodyFetchCancel = nil
			m.viewer = m.viewer.SetBody(msg.Blocks, msg.Unsub)
		}
		return m, nil
```

- [ ] **Step 6: Update test callers of SetBody**

`internal/ui/reader/model_test.go` plus any account-side tests that call `SetBody` need a second argument. Run:

```bash
go build ./... 2>&1
```

For each compile error, append `, content.Unsubscribe{}` to the call. Then:

```bash
go test ./internal/ui/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/reader/ internal/ui/account/model.go
git commit -m "Pass 11: reader U key + Unsubscribe accessor"
```

---

### Task 6: POST cmd + UnsubscribeDoneMsg

**Files:**
- Modify: `internal/ui/cmds.go`

- [ ] **Step 1: Add the cmd and msg**

Append to `internal/ui/cmds.go` (after the existing `launchURLCmd`):

```go
// UnsubscribeDoneMsg fires when the one-click POST resolves.
// Successful runs (2xx) carry an empty Err and a Host string the
// notice consumer renders. Failures surface as ErrorMsg before
// reaching this msg type.
type UnsubscribeDoneMsg struct {
	Host string
}

// unsubscribePostCmd issues an RFC 8058 one-click POST to url with
// body "List-Unsubscribe=One-Click" and a 10-second timeout. 2xx →
// UnsubscribeDoneMsg{Host: <url-host>}. Anything else (non-2xx,
// network, TLS, timeout) → ErrorMsg{Op: "unsubscribe"}.
func unsubscribePostCmd(rawURL string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(rawURL)
		if err != nil {
			return ErrorMsg{Op: "unsubscribe", Err: err}
		}
		body := strings.NewReader("List-Unsubscribe=One-Click")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
		if err != nil {
			return ErrorMsg{Op: "unsubscribe", Err: err}
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return ErrorMsg{Op: "unsubscribe", Err: err}
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ErrorMsg{
				Op:  "unsubscribe",
				Err: fmt.Errorf("server returned %s", resp.Status),
			}
		}
		return UnsubscribeDoneMsg{Host: u.Host}
	}
}
```

Add the missing imports to the file's import block: `net/http`, `net/url`. (`strings`, `fmt`, `context`, `time` are already present.)

- [ ] **Step 2: Add a unit test**

Append to `internal/ui/cmds_test.go` (create if needed; check whether it exists with `ls internal/ui/cmds_test.go` — file already exists per recon):

```go
func TestUnsubscribePostCmd(t *testing.T) {
	t.Run("2xx success", func(t *testing.T) {
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			got = string(body)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		msg := unsubscribePostCmd(srv.URL)()
		done, ok := msg.(UnsubscribeDoneMsg)
		if !ok {
			t.Fatalf("got %T, want UnsubscribeDoneMsg", msg)
		}
		if got != "List-Unsubscribe=One-Click" {
			t.Errorf("body = %q, want %q", got, "List-Unsubscribe=One-Click")
		}
		if done.Host == "" {
			t.Error("Host empty")
		}
	})

	t.Run("non-2xx surfaces ErrorMsg", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		msg := unsubscribePostCmd(srv.URL)()
		if _, ok := msg.(ErrorMsg); !ok {
			t.Fatalf("got %T, want ErrorMsg", msg)
		}
	})

	t.Run("network failure surfaces ErrorMsg", func(t *testing.T) {
		// Closed-port URL.
		msg := unsubscribePostCmd("http://127.0.0.1:1")()
		if _, ok := msg.(ErrorMsg); !ok {
			t.Fatalf("got %T, want ErrorMsg", msg)
		}
	})
}
```

Add the test imports as needed: `io`, `net/http`, `net/http/httptest`, `testing`.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/ui/ -run TestUnsubscribePost -v
```

Expected: all three subtests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/cmds.go internal/ui/cmds_test.go
git commit -m "Pass 11: unsubscribe POST cmd"
```

---

### Task 7: App routing — confirm modal + dispatch

**Files:**
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Add pending-unsubscribe state**

Find the App struct (around line 50). Add:

```go
	pendingUnsub *reader.OpenUnsubscribeConfirmMsg
```

The pointer-or-nil pattern matches existing `pendingEmpty` / `pendingComposeSave` style for confirm-modal rendezvous.

- [ ] **Step 2: Handle OpenUnsubscribeConfirmMsg**

Find the `case account.OpenConfirmEmptyMsg` block (around line 386). Add a sibling case immediately before it (so the unsubscribe handler precedes empty-folder in the switch order):

```go
	case reader.OpenUnsubscribeConfirmMsg:
		host := unsubscribeHost(msg.Unsub)
		stash := msg
		m.pendingUnsub = &stash
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Unsubscribe",
			Body:  "Send unsubscribe request to " + host + "?",
		})
		return m, nil
```

- [ ] **Step 3: Add unsubscribeHost helper**

Append to `internal/ui/app.go` (or a sibling file like `internal/ui/cmds.go`):

```go
// unsubscribeHost returns the user-visible host string for a
// confirm-modal prompt. Picks from the action that will fire (one-click
// → mailto → http) and falls back to a fixed label if every URL is
// malformed.
func unsubscribeHost(u content.Unsubscribe) string {
	switch {
	case u.OneClick != "":
		if p, err := url.Parse(u.OneClick); err == nil && p.Host != "" {
			return p.Host
		}
	case u.Mailto != "":
		if p, err := url.Parse(u.Mailto); err == nil && p.Opaque != "" {
			at := strings.IndexByte(p.Opaque, '?')
			if at < 0 {
				return p.Opaque
			}
			return p.Opaque[:at]
		}
	case u.HTTP != "":
		if p, err := url.Parse(u.HTTP); err == nil && p.Host != "" {
			return p.Host
		}
	}
	return "this list"
}
```

Add `"github.com/glw907/poplar/internal/content"` and `"net/url"` imports if not already present in the chosen file.

- [ ] **Step 4: Route the Yes branch**

In the `ConfirmModalYesMsg` switch (around line 395), add a clause **before** the `pendingEmpty` clause so unsubscribe takes precedence:

```go
		case m.pendingUnsub != nil:
			pu := *m.pendingUnsub
			m.pendingUnsub = nil
			return m, m.dispatchUnsubscribe(pu.Unsub)
```

- [ ] **Step 5: Clear pendingUnsub on No / Esc**

In `ConfirmModalNoMsg` (around line 430): add `m.pendingUnsub = nil` at the top.

In `ConfirmModalClosedMsg` (around line 451): add this clause before the final fallthrough that calls `m.confirm.Close()`:

```go
	if m.pendingUnsub != nil {
		m.pendingUnsub = nil
		m.confirm = m.confirm.Close()
		return m, nil
	}
```

- [ ] **Step 6: Implement dispatchUnsubscribe**

Append to `internal/ui/app.go` near `unsubscribeHost`:

```go
// dispatchUnsubscribe routes the confirmed unsubscribe to its action
// path: one-click POST → mailto compose seed → plain http via
// URLOpener. Precedence matches RFC 8058: https one-click is preferred,
// mailto is the fallback, plain http is last.
func (m App) dispatchUnsubscribe(u content.Unsubscribe) tea.Cmd {
	switch {
	case u.OneClick != "":
		return unsubscribePostCmd(u.OneClick)
	case u.Mailto != "":
		d, err := compose.SeedFromMailto(u.Mailto, m.acct.AccountEmail())
		if err != nil {
			return func() tea.Msg {
				return ErrorMsg{Op: "unsubscribe (mailto)", Err: err}
			}
		}
		return func() tea.Msg { return uicompose.SeededMsg{Draft: d} }
	case u.HTTP != "":
		return launchURLCmd(m.opener, u.HTTP)
	}
	return nil
}
```

Add imports to app.go if not present: `"github.com/glw907/poplar/internal/compose"` (the domain compose), `uicompose "github.com/glw907/poplar/internal/ui/compose"` (likely already present).

- [ ] **Step 7: Handle UnsubscribeDoneMsg (success notice)**

Add a case in App.Update (alongside other msg cases, near the existing toast handlers):

```go
	case UnsubscribeDoneMsg:
		m.lastNotice = "Unsubscribed from " + msg.Host
		return m, clearNoticeAfter(5 * time.Second)
```

The `lastNotice` field and `clearNoticeAfter` helper are introduced in Task 8 — leave this case in place; it will compile after Task 8.

- [ ] **Step 8: Commit (build will fail until Task 8 lands lastNotice; that's fine — bundle the commit at the end of T8)**

Skip commit for now. Move to Task 8.

---

### Task 8: Success notice in chrome banner row

The chrome banner row currently renders error then triage toast then collapses. Add a third tier — a one-shot success notice — between error (highest) and toast.

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/error_banner.go` (or wherever `chromeBannerRow` lives — verify)

- [ ] **Step 1: Locate the banner row**

```bash
grep -n "chromeBannerRow\|lastErr\b\|pendingAction\b" internal/ui/app.go internal/ui/error_banner.go
```

Note the file and function that composes the row. The plan assumes it's `App.chromeBannerRow` in `app.go` — adjust if it's elsewhere.

- [ ] **Step 2: Add lastNotice state**

In the App struct, add:

```go
	lastNotice         string
	lastNoticeDeadline time.Time
```

In the chrome row composition (the function that picks error vs toast), add the notice tier between error and toast:

```go
// Existing logic:
//   if lastErr present → render error
//   else if pendingAction toast present → render toast
//   else → collapsed
//
// New logic:
//   if lastErr present → render error
//   else if lastNotice present and not expired → render notice
//   else if pendingAction toast present → render toast
//   else → collapsed
```

The exact code lives where the existing branches are. Render the notice with the same row dimensions and a `theme.FgBright` (or the existing success-style if one exists) lipgloss style — check `internal/ui/styles.go` for an existing success/notice style; if absent, use `theme.AccentPrimary` to match toast convention.

- [ ] **Step 3: Add clearNoticeAfter**

Append to `internal/ui/cmds.go`:

```go
// noticeExpireMsg fires when a transient notice times out.
type noticeExpireMsg struct{ deadline time.Time }

// clearNoticeAfter schedules a noticeExpireMsg after d.
func clearNoticeAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return noticeExpireMsg{deadline: t.Add(d)}
	})
}
```

In App.Update, handle the expire (alongside `toastExpireMsg`):

```go
	case noticeExpireMsg:
		if !m.lastNoticeDeadline.IsZero() && !time.Now().Before(m.lastNoticeDeadline) {
			m.lastNotice = ""
			m.lastNoticeDeadline = time.Time{}
		}
		return m, nil
```

Update the `UnsubscribeDoneMsg` handler from Task 7 step 7 to set the deadline:

```go
	case UnsubscribeDoneMsg:
		m.lastNotice = "Unsubscribed from " + msg.Host
		m.lastNoticeDeadline = time.Now().Add(5 * time.Second)
		return m, clearNoticeAfter(5 * time.Second)
```

Also clear the notice when an error replaces it — find where `m.lastErr` gets set and add `m.lastNotice = ""` immediately after, matching the spec rule "error wins, then notice, else collapse".

- [ ] **Step 4: Build + test**

```bash
go build ./...
go test ./internal/ui/...
```

Expected: PASS.

- [ ] **Step 5: Commit Tasks 7 + 8 together**

```bash
git add internal/ui/app.go internal/ui/cmds.go internal/ui/error_banner.go
git commit -m "Pass 11: confirm-modal routing + success notice"
```

---

### Task 9: SeedFromMailto helper

**Files:**
- Create: `internal/compose/mailto.go`
- Create: `internal/compose/mailto_test.go`

- [ ] **Step 1: Write failing tests**

```go
package compose

import (
	"testing"

	gomail "github.com/emersion/go-message/mail"
)

func TestSeedFromMailto(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		fromEmail string
		want      Draft
		wantErr   bool
	}{
		{
			name:      "address only",
			raw:       "mailto:unsub@list.example",
			fromEmail: "me@example.org",
			want: Draft{
				From: gomail.Address{Address: "me@example.org"},
				To:   []gomail.Address{{Address: "unsub@list.example"}},
			},
		},
		{
			name:      "subject + body",
			raw:       "mailto:u@list.example?subject=unsub&body=please%20unsub%20me",
			fromEmail: "me@example.org",
			want: Draft{
				From:    gomail.Address{Address: "me@example.org"},
				To:      []gomail.Address{{Address: "u@list.example"}},
				Subject: "unsub",
				Body:    "please unsub me",
			},
		},
		{
			name:      "multiple addresses — first wins",
			raw:       "mailto:a@list.example,b@list.example",
			fromEmail: "me@example.org",
			want: Draft{
				From: gomail.Address{Address: "me@example.org"},
				To:   []gomail.Address{{Address: "a@list.example"}},
			},
		},
		{
			name:      "not a mailto",
			raw:       "https://list.example/u",
			fromEmail: "me@example.org",
			wantErr:   true,
		},
		{
			name:      "missing scheme prefix",
			raw:       "u@list.example?subject=x",
			fromEmail: "me@example.org",
			wantErr:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := SeedFromMailto(tc.raw, tc.fromEmail)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.From != tc.want.From {
				t.Errorf("From = %v, want %v", d.From, tc.want.From)
			}
			if len(d.To) != len(tc.want.To) || (len(d.To) > 0 && d.To[0] != tc.want.To[0]) {
				t.Errorf("To = %v, want %v", d.To, tc.want.To)
			}
			if d.Subject != tc.want.Subject {
				t.Errorf("Subject = %q, want %q", d.Subject, tc.want.Subject)
			}
			if d.Body != tc.want.Body {
				t.Errorf("Body = %q, want %q", d.Body, tc.want.Body)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test ./internal/compose/ -run SeedFromMailto -v
```

Expected: FAIL ("undefined: SeedFromMailto").

- [ ] **Step 3: Implement SeedFromMailto**

```go
package compose

import (
	"errors"
	"net/url"
	"strings"

	gomail "github.com/emersion/go-message/mail"
)

// SeedFromMailto parses a mailto: URL and returns a Draft seeded with
// the first recipient and any subject/body query parameters. Multiple
// addresses are tolerated (first wins, RFC 6068 reading). fromEmail
// populates the From field.
func SeedFromMailto(raw, fromEmail string) (Draft, error) {
	if !strings.HasPrefix(strings.ToLower(raw), "mailto:") {
		return Draft{}, errors.New("not a mailto URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Draft{}, err
	}
	if u.Opaque == "" {
		return Draft{}, errors.New("mailto URL missing recipient")
	}
	addrs := strings.Split(u.Opaque, ",")
	first := strings.TrimSpace(addrs[0])
	if first == "" {
		return Draft{}, errors.New("mailto URL missing recipient")
	}
	d := Draft{
		From: gomail.Address{Address: fromEmail},
		To:   []gomail.Address{{Address: first}},
	}
	q := u.Query()
	d.Subject = q.Get("subject")
	d.Body = q.Get("body")
	return d, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

```bash
go test ./internal/compose/ -run SeedFromMailto -v
```

Expected: all subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/compose/mailto.go internal/compose/mailto_test.go
git commit -m "Pass 11: compose.SeedFromMailto"
```

---

### Task 10: Conditional `U unsub` footer hint

**Files:**
- Modify: `internal/ui/footer.go`
- Modify: `internal/ui/app.go`

- [ ] **Step 1: Add a parameterized viewer footer builder**

In `internal/ui/footer.go`, add a sibling to `viewerFooterGroups`:

```go
// viewerFooterGroupsWithUnsub returns the viewer footer hint groups
// with a conditional `U unsub` hint inserted into the reply group at
// drop rank 6. Called only when the current message has actionable
// List-Unsubscribe data.
func viewerFooterGroupsWithUnsub() [][]footerHint {
	return [][]footerHint{
		triageHints,
		append(append([]footerHint(nil), replyHints...), hint("U", "unsub", 6)),
		{
			hint("Tab", "links", 0),
			hint("q", "close", 0),
			hint("?", "help", 0),
		},
	}
}
```

- [ ] **Step 2: Switch viewer footer based on viewer state**

Find the existing call in `internal/ui/app.go` (around line 1091) where `m.footer.View(m.width)` runs. Currently this branch handles ViewerContext via the unconditional `viewerFooterGroups`. Change to:

```go
		if m.viewerOpen() && m.acctTab.Viewer().Unsubscribe().Available() {
			foot = m.footer.ViewGroups(viewerFooterGroupsWithUnsub(), m.width)
		} else {
			foot = m.footer.View(m.width)
		}
```

The exact accessor name (`m.acctTab.Viewer()`, `m.viewerOpen()`) depends on the App's existing surface — verify with:

```bash
grep -n "acctTab\|viewerOpen\|m.viewer\b" internal/ui/app.go | head -10
```

Use whatever accessor returns the live viewer model. If none exists, add `Viewer() reader.Model` on AccountTab.

- [ ] **Step 3: Build + test**

```bash
go build ./...
go test ./internal/ui/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/ui/footer.go internal/ui/app.go
git commit -m "Pass 11: conditional U unsub footer hint"
```

---

### Task 11: Docs + ADR + invariants

**Files:**
- Modify: `docs/poplar/keybindings.md`
- Modify: `docs/poplar/invariants.md`
- Create: `docs/poplar/decisions/0185-list-unsubscribe.md`
- Modify: `docs/poplar/decisions/INDEX.md`
- Modify: `docs/poplar/STATUS.md`

- [ ] **Step 1: Add U row to keybindings.md**

Under the **Viewer** section table (around line 156), add a row:

```markdown
| `U` | Unsubscribe (when List-Unsubscribe header present) | V |
```

In the viewer footer block, document the conditional hint:

```markdown
`U unsub` is conditional — it appears at drop rank 6 only when the
current message carries `List-Unsubscribe` (RFC 2369) headers.
```

- [ ] **Step 2: Write ADR-0185**

Create `docs/poplar/decisions/0185-list-unsubscribe.md`:

```markdown
---
title: List-Unsubscribe — RFC 8058 one-click preferred, mailto fallback opens compose
status: accepted
date: 2026-05-09
---

## Context

RFC 2369 advertises an unsubscribe path via the `List-Unsubscribe`
header (mailto and/or http URLs). RFC 8058 layers a one-click POST
profile on top: when the sender opts in via
`List-Unsubscribe-Post: List-Unsubscribe=One-Click`, the client
posts `List-Unsubscribe=One-Click` to the https URL with no human
in the loop. Every major client in poplar's matrix (Thunderbird,
Apple Mail, Outlook, mutt, aerc, K-9, Geary, Evolution) surfaces
the affordance whenever the header is present and fires on click;
none remember prior unsubscribes.

## Decision

- Single key `U` in the viewer (modifier-free uppercase per
  ADR-0076; free in the keybinding map). Inert when no
  List-Unsubscribe headers are present.
- Confirmation prompt via `ConfirmModal`. POST is irreversible.
- Action precedence: https one-click POST > mailto into compose >
  plain http via the existing `URLOpener` seam.
- Mailto fallback opens poplar compose pre-filled (To/Subject/Body
  parsed from the mailto URL); we don't route mailto through
  `xdg-open` since poplar is the mail client.
- Plain http (no one-click promotion) routes through `URLOpener`
  with no POST — same path as `1`–`9` link launch.
- No client-side memory of prior unsubscribes. The unsub endpoint
  is the source of truth (idempotent in practice); a well-behaved
  list stops sending after a successful unsub. Universal across
  the matrix.
- Header parsing lives in `internal/content/listunsubscribe.go` as
  a pure function `ParseListUnsubscribe(textproto.MIMEHeader)
  Unsubscribe`. Parse runs in the existing body-fetch Cmd; result
  rides back on `reader.BodyLoadedMsg.Unsub`. `mail.MessageInfo`
  is not extended — the affordance is viewer-only.

## Consequences

- Viewer footer gains a conditional `U unsub` hint at drop rank 6.
- Confirm modal cascade picks up one new pending state
  (`pendingUnsub`); precedence is unsubscribe > empty > others.
- Success surfaces via a new `lastNotice` tier in the chrome
  banner row (between error and triage toast). 5-second visibility
  with auto-clear.
- Pre-1.0 revisit: if usage shows users want a "you've already
  done this" signal, add per-List-Id memory in a new schema
  table. Not in scope here.
```

- [ ] **Step 3: Update INDEX.md**

In `docs/poplar/decisions/INDEX.md`, add an entry under the appropriate theme (likely "Viewer" or "Outbound mail"):

```markdown
| 0185 | List-Unsubscribe (RFC 8058 one-click) |
```

- [ ] **Step 4: Update invariants.md**

In `docs/poplar/invariants.md`, under the viewer section (the `Viewer` subsection of the architecture facts), add a new bullet:

```markdown
- Viewer harvests `List-Unsubscribe` (RFC 2369) and
  `List-Unsubscribe-Post` (RFC 8058) headers at body-fetch time
  via `content.ParseListUnsubscribe`. The parsed
  `content.Unsubscribe` rides on `reader.BodyLoadedMsg.Unsub` and
  is exposed via `reader.Model.Unsubscribe()`. `U` opens a
  `ConfirmModal`; on Yes the App routes by precedence: https
  one-click POST (`unsubscribePostCmd`, 10s timeout, 2xx success)
  > mailto into compose (`compose.SeedFromMailto`) > plain http
  via `URLOpener`. No client-side memory of prior unsubscribes.
  Success surfaces as `App.lastNotice` ("Unsubscribed from
  <host>") in the chrome banner row tier between error and
  triage toast (5s auto-clear). ADR-0185.
```

- [ ] **Step 5: Update STATUS.md**

Mark Pass 11 done in the table; replace the starter prompt with Pass 12 (`.ics` viewer #37) per the existing queue.

- [ ] **Step 6: Commit**

```bash
git add docs/
git commit -m "Pass 11: ADR-0185 + invariants + keybindings + STATUS"
```

---

### Task 12: Live verification + tmux capture

- [ ] **Step 1: make check**

```bash
make check
```

Expected: green (fmt, vet, voice, test).

- [ ] **Step 2: make install**

```bash
make install
```

- [ ] **Step 3: Live test against Fastmail**

Launch poplar against the live Fastmail account (`geoff@907.life`), open a known list message in the viewer (newsletters, GitHub notifications, etc. typically carry both forms). Verify:

- `U` is inert on a non-list message (no footer hint either).
- `U` on a list message opens `Send unsubscribe request to <host>?`.
- Yes confirms; success notice appears in the banner row.
- Banner clears after 5 seconds.

- [ ] **Step 4: tmux captures**

Per `docs/poplar/wireframes.md` conventions, capture the confirm modal at 80×24 and 120×40. Save to `docs/poplar/wireframes/` with a descriptive filename.

- [ ] **Step 5: Pass-end ritual**

Invoke the `poplar-pass` skill for archival, commit/push/install. Plan archive:

```bash
git mv docs/superpowers/plans/2026-05-09-list-unsubscribe.md docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-09-list-unsubscribe-design.md docs/superpowers/archive/specs/
```

---

## Self-review notes

**Spec coverage:**
- Header parser ✓ (T1–T3)
- Wire path (BodyLoadedMsg extension) ✓ (T4)
- Reader U key + accessor ✓ (T5)
- POST cmd ✓ (T6)
- App routing + confirm modal ✓ (T7)
- Success notice ✓ (T8)
- Mailto fallback ✓ (T9)
- Footer ✓ (T10)
- Docs + ADR ✓ (T11)
- Live verify + ritual ✓ (T12)

**Type consistency check:** `Unsubscribe`, `OneClick`, `Mailto`, `HTTP`, `Available()`, `ParseListUnsubscribe`, `OpenUnsubscribeConfirmMsg`, `UnsubscribeDoneMsg`, `unsubscribePostCmd`, `unsubscribeHost`, `dispatchUnsubscribe`, `SeedFromMailto`, `viewerFooterGroupsWithUnsub`, `lastNotice`, `lastNoticeDeadline`, `noticeExpireMsg`, `clearNoticeAfter` — names consistent across tasks.

**Pass-size budget:** 12 tasks. Slightly over the 8–12 nominal but T11 (docs) and T12 (verify+ritual) are routine. Implementation core is T1–T10 (10 tasks). Within budget.

**Deviation from spec:** Task 4 extends `BodyLoadedMsg` rather than emitting a sibling msg — see "Plan deviation note" at top.
