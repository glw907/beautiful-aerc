# CardDAV write-back Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Round-trip the contact-edit Form: form save → cache upsert →
outbox PUT → CardDAV server, plus `D`-key delete via the same path.
Lossless vCard edits (preserve fields poplar doesn't model).

**Architecture:** Two new outbox kinds (`KindContactPut`, `KindContactDelete`)
glued to the existing drainer via a `contacts.Writer` seam on `cache.Account`.
Saves patch the stored vCard blob in-place to keep unknown fields. Conflicts
mirror mail-outbox semantics (If-Match → 412 → revert + conflicted state).

**Tech Stack:** `github.com/emersion/go-vcard`, `github.com/emersion/go-webdav`,
modernc.org/sqlite, bubbles/bubbletea, the existing cache drainer.

**Spec:** `docs/superpowers/specs/2026-05-07-carddav-writeback-design.md`.

---

## File Map

- Create:
  - `internal/contacts/writeback.go` — `BuildVCard`, `PatchVCard`.
  - `internal/contacts/writeback_test.go` — round-trip cases.
- Modify:
  - `internal/contacts/client.go` — add sentinels + `PutAddressObject`,
    `DeleteAddressObject`.
  - `internal/contacts/client_test.go` (create if absent) — fake HTTP server.
  - `internal/contacts/types.go` — `Writer` interface (one method per op).
  - `internal/cache/account.go` — `ContactsWriter contacts.Writer` field on
    `Account`.
  - `internal/cache/ops.go` — `KindContactPut`, `KindContactDelete`,
    `ContactPutArgs`, `ContactDeleteArgs`, `revertOptimisticTx` no-op for
    the new kinds.
  - `internal/cache/contacts.go` — `QueueContactPut`, `QueueContactDelete`,
    `reconcileRecipientsTx` helper.
  - `internal/cache/contacts_test.go` — queue-side tests.
  - `internal/cache/drainer.go` — dispatch arms for the new kinds, sentinel
    routing.
  - `internal/cache/conflicts_test.go` — fake writer 412 → revert.
  - `internal/cache/recover_test.go` (or wherever `recoverExecuting` is
    tested) — extend the non-idempotent set.
  - `internal/ui/contacts/keys.go` — `D` binding.
  - `internal/ui/contacts/msgs.go` — `OpenContactDeleteConfirmMsg`,
    `ContactDeleteMsg`.
  - `internal/ui/contacts/form.go` — wire `D` (gated on existing UID).
  - `internal/ui/contacts/form_test.go` — gating + emit tests.
  - `internal/ui/app.go` — handle `ContactSaveMsg` (queue Put),
    `OpenContactDeleteConfirmMsg` (open confirm), `ContactDeleteMsg`
    (queue Delete), confirm cascade ordering for contact-delete.

---

## Task 1: vCard build/patch primitives

Pure functions, no I/O. Foundation for every other task.

**Files:**
- Create: `internal/contacts/writeback.go`
- Create: `internal/contacts/writeback_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/contacts/writeback_test.go
package contacts

import (
	"strings"
	"testing"
	"time"
)

func TestBuildVCard_PersonMinimal(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	c := Contact{
		Kind:   KindPerson,
		Given:  "Ada",
		Family: "Lovelace",
		Name:   "Ada Lovelace",
		Emails: []Email{{Address: "ada@example.org", Label: "work"}},
	}
	got, err := BuildVCard(c, "uid-1", now)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{
		"BEGIN:VCARD", "VERSION:4.0", "UID:uid-1",
		"FN:Ada Lovelace", "N:Lovelace;Ada;;;",
		"EMAIL;PREF=1;TYPE=work:ada@example.org",
		"REV:2026-05-07T12:00:00Z", "END:VCARD",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestBuildVCard_OrgKind(t *testing.T) {
	c := Contact{Kind: KindOrg, Name: "ACME", Org: "ACME"}
	got, err := BuildVCard(c, "uid-org", time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "KIND:org") {
		t.Errorf("KindOrg missing KIND:org line:\n%s", got)
	}
}

func TestPatchVCard_PreservesUnknownFields(t *testing.T) {
	stored := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\n" +
		"FN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\n" +
		"EMAIL;PREF=1:ada@example.org\r\n" +
		"BDAY:18151210\r\nX-CUSTOM:keep-me\r\n" +
		"END:VCARD\r\n")
	c := Contact{
		Kind:   KindPerson,
		Given:  "Ada",
		Family: "Lovelace",
		Name:   "Ada Lovelace",
		Note:   "added note",
		Emails: []Email{{Address: "ada@example.org", Label: ""}},
	}
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	got, err := PatchVCard(stored, c, now)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{"BDAY:18151210", "X-CUSTOM:keep-me", "NOTE:added note"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q after patch:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "REV:2026-05-07T12:00:00Z") {
		t.Errorf("REV not bumped:\n%s", s)
	}
}

func TestPatchVCard_AddRemoveEmailsKeepLabels(t *testing.T) {
	stored := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\n" +
		"FN:Ada\r\nN:;Ada;;;\r\n" +
		"EMAIL;PREF=1;TYPE=_$!<Work>!$_:ada@old.example\r\n" +
		"EMAIL;TYPE=home:ada@home.example\r\n" +
		"END:VCARD\r\n")
	c := Contact{
		Kind:   KindPerson,
		Given:  "Ada",
		Name:   "Ada",
		Emails: []Email{
			{Address: "ada@home.example", Label: "home"}, // promoted to PREF
			{Address: "ada@new.example", Label: "work"},  // added
		},
	}
	got, err := PatchVCard(stored, c, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, "ada@old.example") {
		t.Errorf("removed email still present:\n%s", s)
	}
	if !strings.Contains(s, "EMAIL;PREF=1;TYPE=home:ada@home.example") {
		t.Errorf("retained row missing or PREF not promoted:\n%s", s)
	}
	if !strings.Contains(s, "EMAIL;TYPE=work:ada@new.example") {
		t.Errorf("added row missing canonical TYPE:\n%s", s)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/contacts/ -run 'BuildVCard|PatchVCard' -v`
Expected: FAIL with `undefined: BuildVCard` / `PatchVCard`.

- [ ] **Step 3: Implement `BuildVCard` and `PatchVCard`**

```go
// internal/contacts/writeback.go
package contacts

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
)

// BuildVCard encodes a fresh card from c. Used for new contacts that
// have no stored blob to patch.
func BuildVCard(c Contact, uid string, now time.Time) ([]byte, error) {
	card := vcard.Card{}
	card.SetValue(vcard.FieldVersion, "4.0")
	card.SetValue(vcard.FieldUID, uid)
	applyOwnedFields(card, c, now)
	if c.Kind == KindOrg {
		card.SetValue(vcard.FieldKind, "org")
	}
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, fmt.Errorf("encode vcard: %w", err)
	}
	return buf.Bytes(), nil
}

// PatchVCard decodes stored, mutates only the fields poplar models,
// and re-encodes. Unknown fields (BDAY, ADR, X-*, PHOTO, …) survive.
func PatchVCard(stored []byte, c Contact, now time.Time) ([]byte, error) {
	dec := vcard.NewDecoder(bytes.NewReader(stored))
	card, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode stored vcard: %w", err)
	}
	applyOwnedFields(card, c, now)
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, fmt.Errorf("re-encode vcard: %w", err)
	}
	return buf.Bytes(), nil
}

// applyOwnedFields mutates card in place: rewrites FN, N, ORG, TITLE,
// NOTE, EMAIL, TEL, REV. Other keys are untouched.
func applyOwnedFields(card vcard.Card, c Contact, now time.Time) {
	if c.Name != "" {
		card.SetValue(vcard.FieldFormattedName, c.Name)
	}
	if c.Family != "" || c.Given != "" {
		card.SetName(&vcard.Name{FamilyName: c.Family, GivenName: c.Given})
	} else {
		delete(card, vcard.FieldName)
	}
	setOrDelete(card, vcard.FieldOrganization, c.Org)
	setOrDelete(card, vcard.FieldTitle, c.Title)
	setOrDelete(card, vcard.FieldNote, c.Note)

	mergeRows(card, vcard.FieldEmail, emailValues(c.Emails), emailTypes(c.Emails))
	mergeRows(card, vcard.FieldTelephone, phoneValues(c.Phones), phoneTypes(c.Phones))

	card.SetValue(vcard.FieldRevision, now.UTC().Format(time.RFC3339))
}

func setOrDelete(card vcard.Card, key, val string) {
	if val == "" {
		delete(card, key)
		return
	}
	card.SetValue(key, val)
}

// mergeRows replaces card[key] with one row per value, preserving
// existing TYPE params for retained rows (matched by case-insensitive
// value equality) and assigning newType for added rows. Index 0 gets
// PREF=1; others have PREF cleared.
func mergeRows(card vcard.Card, key string, values, newTypes []string) {
	old := card[key]
	indexOld := func(v string) int {
		for i, f := range old {
			if strings.EqualFold(f.Value, v) {
				return i
			}
		}
		return -1
	}
	out := make([]*vcard.Field, 0, len(values))
	for i, v := range values {
		var f *vcard.Field
		if idx := indexOld(v); idx >= 0 {
			f = old[idx]
			f.Params.Del(vcard.ParamPreferred)
		} else {
			f = &vcard.Field{Value: v, Params: vcard.Params{}}
			if newTypes[i] != "" {
				f.Params.Set(vcard.ParamType, newTypes[i])
			}
		}
		if i == 0 {
			f.Params.Set(vcard.ParamPreferred, "1")
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		delete(card, key)
		return
	}
	card[key] = out
}

func emailValues(es []Email) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Address
	}
	return out
}
func emailTypes(es []Email) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = strings.ToLower(e.Label)
	}
	return out
}
func phoneValues(ps []Phone) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.E164
	}
	return out
}
func phoneTypes(ps []Phone) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = strings.ToLower(p.Label)
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/contacts/ -v`
Expected: PASS for the new tests; existing tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/contacts/writeback.go internal/contacts/writeback_test.go
git commit -m "Pass 9m.1: vCard build + patch primitives" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: CardDAV write methods on Client

Add raw PUT/DELETE wrappers with `If-Match` and the typed sentinels
the drainer routes on.

**Files:**
- Modify: `internal/contacts/client.go`
- Create: `internal/contacts/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/contacts/client_test.go
package contacts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "u", "p", false)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPutAddressObject_Success(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %q want PUT", r.Method)
		}
		if got := r.Header.Get("If-Match"); got != `"old"` {
			t.Errorf("If-Match = %q want %q", got, `"old"`)
		}
		w.Header().Set("ETag", `"new"`)
		w.WriteHeader(http.StatusCreated)
	}))
	href, etag, err := c.PutAddressObject(context.Background(),
		"/addressbooks/u/default/u1.vcf", `"old"`,
		[]byte("BEGIN:VCARD\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(href, "/u1.vcf") || etag != `"new"` {
		t.Errorf("got href=%q etag=%q", href, etag)
	}
}

func TestPutAddressObject_PreconditionFailed(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	_, _, err := c.PutAddressObject(context.Background(), "/x.vcf", `"e"`, []byte("x"))
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("err = %v want ErrPreconditionFailed", err)
	}
}

func TestDeleteAddressObject_NotFound(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	err := c.DeleteAddressObject(context.Background(), "/x.vcf", `"e"`)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v want ErrNotFound", err)
	}
}

func TestDeleteAddressObject_Auth(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	err := c.DeleteAddressObject(context.Background(), "/x.vcf", "")
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err = %v want ErrAuth", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/contacts/ -run 'PutAddressObject|DeleteAddressObject' -v`
Expected: FAIL — methods + sentinels do not exist.

- [ ] **Step 3: Implement the methods + sentinels**

Append to `internal/contacts/client.go`:

```go
// near the top imports, add:
//   "errors"
//   "net/url"
//   "io"

// Sentinels routed on by the drainer's conflict matrix.
var (
	ErrAuth               = errors.New("contacts: auth")
	ErrNotFound           = errors.New("contacts: not found")
	ErrPreconditionFailed = errors.New("contacts: precondition failed")
)

// PutAddressObject writes body to href with optional If-Match. Returns
// the server's chosen href (may differ from input) and the new ETag.
// Maps 401/403 → ErrAuth, 404 → ErrNotFound, 412 → ErrPreconditionFailed.
func (c *Client) PutAddressObject(
	ctx context.Context, href, ifMatch string, body []byte,
) (newHref, newETag string, err error) {
	req, err := c.newReq(ctx, "PUT", href, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "text/vcard; charset=utf-8")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("put %s: %w", href, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if e := mapStatus(resp.StatusCode); e != nil {
		return "", "", fmt.Errorf("put %s: %w", href, e)
	}
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("put %s: status %d", href, resp.StatusCode)
	}
	newHref = href
	if loc := resp.Header.Get("Location"); loc != "" {
		newHref = loc
	}
	return newHref, resp.Header.Get("ETag"), nil
}

// DeleteAddressObject removes href with optional If-Match. Maps the
// same status codes as PutAddressObject. Caller treats ErrNotFound as
// idempotent success.
func (c *Client) DeleteAddressObject(ctx context.Context, href, ifMatch string) error {
	req, err := c.newReq(ctx, "DELETE", href, nil)
	if err != nil {
		return err
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete %s: %w", href, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if e := mapStatus(resp.StatusCode); e != nil {
		return fmt.Errorf("delete %s: %w", href, e)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("delete %s: status %d", href, resp.StatusCode)
	}
	return nil
}

// newReq resolves href against the configured base URL and builds a
// context-bound request.
func (c *Client) newReq(ctx context.Context, method, href string, body io.Reader) (*http.Request, error) {
	target := href
	if u, err := url.Parse(href); err == nil && !u.IsAbs() {
		base, _ := url.Parse(c.base)
		target = base.ResolveReference(u).String()
	}
	return http.NewRequestWithContext(ctx, method, target, body)
}

func mapStatus(code int) error {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuth
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusPreconditionFailed:
		return ErrPreconditionFailed
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/contacts/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/contacts/client.go internal/contacts/client_test.go
git commit -m "Pass 9m.1: CardDAV PUT/DELETE with If-Match" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Writer interface + op kinds + decoder

Introduce the `contacts.Writer` seam, the new op kinds, the args
types, and extend `decodeArgs` / `revertOptimisticTx` /
`recoverExecuting`. No drainer dispatch yet.

**Files:**
- Modify: `internal/contacts/types.go`
- Modify: `internal/cache/account.go`
- Modify: `internal/cache/ops.go`
- Modify: `internal/cache/drainer.go` (decoder + recover only)

- [ ] **Step 1: Write the failing tests**

Append to `internal/cache/conflicts_test.go`:

```go
func TestDecodeArgs_ContactKinds(t *testing.T) {
	put := ContactPutArgs{BookHref: "/b/", Href: "/b/c.vcf", IfMatch: `"e1"`}
	bs, _ := json.Marshal(put)
	got, err := decodeArgs(string(KindContactPut), string(bs))
	if err != nil {
		t.Fatal(err)
	}
	if g, ok := got.(ContactPutArgs); !ok || g != put {
		t.Errorf("got %#v want %#v", got, put)
	}

	del := ContactDeleteArgs{Href: "/b/c.vcf", IfMatch: `"e1"`}
	bs, _ = json.Marshal(del)
	got, err = decodeArgs(string(KindContactDelete), string(bs))
	if err != nil {
		t.Fatal(err)
	}
	if g, ok := got.(ContactDeleteArgs); !ok || g != del {
		t.Errorf("got %#v want %#v", got, del)
	}
}

func TestRevertOptimisticTx_ContactKindsAreNoop(t *testing.T) {
	// revertOptimisticTx must accept the new kinds without error so
	// DiscardOp works on conflicted contact ops.
	tx := (*sql.Tx)(nil) // not used; the new kinds short-circuit
	if err := revertOptimisticTx(tx, 0, ContactPutArgs{}); err != nil {
		t.Errorf("ContactPutArgs revert: %v", err)
	}
	if err := revertOptimisticTx(tx, 0, ContactDeleteArgs{}); err != nil {
		t.Errorf("ContactDeleteArgs revert: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cache/ -run 'DecodeArgs_Contact|RevertOptimisticTx_Contact' -v`
Expected: FAIL — `KindContactPut` undefined, etc.

- [ ] **Step 3: Add the Writer seam**

Append to `internal/contacts/types.go`:

```go
// Writer is the CardDAV write surface the cache drainer dispatches
// against. *Client satisfies it; tests substitute fakes.
type Writer interface {
	PutAddressObject(ctx context.Context, href, ifMatch string, body []byte) (newHref, newETag string, err error)
	DeleteAddressObject(ctx context.Context, href, ifMatch string) error
	Multiget(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error)
}
```

(Add the imports `"context"` and `"github.com/emersion/go-webdav/carddav"`
to `types.go`.)

- [ ] **Step 4: Add the field on Account**

Edit `internal/cache/account.go`. Add to the `Account` struct (after
`ChangeTracker`):

```go
	ContactsWriter contacts.Writer
```

Add the import `"github.com/glw907/poplar/internal/contacts"` if not
already present (it is — used in `cache/contacts.go`, but `account.go`
needs its own import).

- [ ] **Step 5: Add op kinds + args**

Edit `internal/cache/account.go` — add to the `OpKind` const block:

```go
	KindContactPut    OpKind = "contact-put"
	KindContactDelete OpKind = "contact-delete"
```

Edit `internal/cache/ops.go` — add to the args block + `opKind`
methods:

```go
type ContactPutArgs struct {
	BookHref string
	Href     string // "" for new
	IfMatch  string // "" for new
}
type ContactDeleteArgs struct {
	Href    string
	IfMatch string
}

func (ContactPutArgs) opKind() OpKind    { return KindContactPut }
func (ContactDeleteArgs) opKind() OpKind { return KindContactDelete }
```

Extend `revertOptimisticTx` (same file) — add the no-op case alongside
SendArgs/AppendArgs/PushDraftArgs:

```go
	case SendArgs, AppendArgs, PushDraftArgs, ContactPutArgs, ContactDeleteArgs:
		return nil
```

- [ ] **Step 6: Extend decodeArgs**

Edit `internal/cache/drainer.go` — add cases in `decodeArgs`:

```go
	case KindContactPut:
		var v ContactPutArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
	case KindContactDelete:
		var v ContactDeleteArgs
		if err := json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		return v, nil
```

- [ ] **Step 7: Extend recoverExecuting**

Edit `internal/cache/drainer.go` — `recoverExecuting` currently
restores `Move/Flag/Destroy` to pending and conflicts everything else.
Contact ops are idempotent under If-Match (server rejects stale
attempts), so add them to the idempotent set:

```go
	if _, err := a.db.Exec(
		`UPDATE outbox SET status = ? WHERE status = ? AND kind IN (?,?,?,?,?)`,
		OpPending, OpExecuting,
		KindMove, KindFlag, KindDestroy, KindContactPut, KindContactDelete); err != nil {
		return err
	}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/cache/ -v`
Expected: PASS for the new decoder/revert tests; nothing else
regressed.

- [ ] **Step 9: Commit**

```bash
git add internal/contacts/types.go internal/cache/account.go \
        internal/cache/ops.go internal/cache/drainer.go \
        internal/cache/conflicts_test.go
git commit -m "Pass 9m.1: contact op kinds + Writer seam" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: QueueContactPut + QueueContactDelete

Optimistic local apply + outbox insert + recipient-projection
reconciliation, all in one tx.

**Files:**
- Modify: `internal/cache/contacts.go`
- Modify: `internal/cache/contacts_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/cache/contacts_test.go`:

```go
func TestQueueContactPut_ReconcilesEmails(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	// Seed: one contact with one email, one historical message from
	// that address so a recipient projection row exists.
	mustExec(t, a, `INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at) VALUES ('/b/', 'Default', '', '', '', 1, 0)`)
	mustExec(t, a, `INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		VALUES ('u1', '/b/', '/b/u1.vcf', '"e1"', ?, '', 'Ada', 'Lovelace', 'Ada', '', '', '', 0)`,
		[]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\nFN:Ada\r\nN:Lovelace;Ada;;;\r\nEMAIL;PREF=1:old@x.example\r\nEND:VCARD\r\n"))
	mustExec(t, a, `INSERT INTO contact_emails(contact_uid, address, label, pref) VALUES ('u1','old@x.example','',1)`)
	// Need a folder + message to back the projection inserts.
	mustExec(t, a, `INSERT INTO folders(id, name) VALUES (1, 'INBOX')`)
	mustExec(t, a, `INSERT INTO messages(id, protocol_id, sent_at, from_address, from_name, to_addresses, cc_addresses, subject, flags, ui_flags, ui_hide) VALUES (10, 'm10', 100, 'new@x.example', 'Ada', '', '', 's', 0, 0, 0)`)
	mustExec(t, a, `INSERT INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (10,'from','new@x.example','Ada',100)`)
	mustExec(t, a, `INSERT INTO message_recipients(message_uid, role, address, name, sent_at) VALUES (10,'from','old@x.example','Ada',100)`)

	c := contacts.Contact{
		Kind: contacts.KindPerson, Given: "Ada", Family: "Lovelace", Name: "Ada Lovelace",
		Emails: []contacts.Email{{Address: "new@x.example"}},
	}
	body := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:u1\r\nEMAIL;PREF=1:new@x.example\r\nEND:VCARD\r\n")
	if err := a.QueueContactPut(ctx, "/b/", "u1", "/b/u1.vcf", `"e1"`, c, body); err != nil {
		t.Fatal(err)
	}

	// Outbox row exists.
	var rowKind string
	if err := a.db.QueryRow(`SELECT kind FROM outbox`).Scan(&rowKind); err != nil {
		t.Fatal(err)
	}
	if rowKind != string(KindContactPut) {
		t.Errorf("kind=%q want contact-put", rowKind)
	}

	// contact_emails reconciled.
	var addrs []string
	rows, _ := a.db.Query(`SELECT address FROM contact_emails WHERE contact_uid='u1'`)
	defer rows.Close()
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		addrs = append(addrs, s)
	}
	if len(addrs) != 1 || addrs[0] != "new@x.example" {
		t.Errorf("contact_emails=%v want [new@x.example]", addrs)
	}

	// message_recipients projection: removed addr untouched (still
	// rooted in messages), but `contact_uid` cross-walk would change
	// only via contact_emails (NOCASE join). We verify the pivot table
	// instead — no spurious row added/removed in message_recipients.
	var nrec int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM message_recipients WHERE message_uid=10`).Scan(&nrec); err != nil {
		t.Fatal(err)
	}
	if nrec != 2 {
		t.Errorf("message_recipients rows=%d want 2", nrec)
	}
}

func TestQueueContactDelete_RemovesRowAndRecipientPivots(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	ctx := context.Background()

	mustExec(t, a, `INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at) VALUES ('/b/', 'Default', '', '', '', 1, 0)`)
	mustExec(t, a, `INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		VALUES ('u1', '/b/', '/b/u1.vcf', '"e1"', x'', '', 'Ada', '', 'Ada', '', '', '', 0)`)
	mustExec(t, a, `INSERT INTO contact_emails(contact_uid, address, label, pref) VALUES ('u1','a@x.example','',1)`)

	if err := a.QueueContactDelete(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM contacts WHERE uid='u1'`).Scan(&n)
	if n != 0 {
		t.Errorf("contacts row not deleted")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM contact_emails WHERE contact_uid='u1'`).Scan(&n)
	if n != 0 {
		t.Errorf("contact_emails not cascaded")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE kind=?`, string(KindContactDelete)).Scan(&n)
	if n != 1 {
		t.Errorf("outbox row missing for delete")
	}
}
```

(Use the existing `newTestAccount` / `mustExec` helpers from
`cache_test.go` if present; if their signatures differ, adapt to
match. Run `grep -n 'func newTestAccount\|func mustExec' internal/cache/*.go`
to confirm.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cache/ -run 'QueueContactPut|QueueContactDelete' -v`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement the queue methods**

Append to `internal/cache/contacts.go`:

```go
// QueueContactPut writes vcardBytes to the cache and enqueues a
// CardDAV PUT. ifMatch is the etag pinned for optimistic concurrency
// (empty for new contacts). The caller already produced vcardBytes
// via contacts.PatchVCard or contacts.BuildVCard.
func (a *Account) QueueContactPut(
	ctx context.Context,
	bookHref string,
	uid string,
	href string,
	ifMatch string,
	c contacts.Contact,
	vcardBytes []byte,
) error {
	args := ContactPutArgs{BookHref: bookHref, Href: href, IfMatch: ifMatch}
	body, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("encode args: %w", err)
	}
	now := time.Now()
	err = a.tx(ctx, func(tx *sql.Tx) error {
		// Diff old emails for projection reconciliation.
		oldEmails, err := loadEmailsTx(ctx, tx, uid)
		if err != nil {
			return err
		}
		stored := contacts.Stored{
			Parsed: contacts.Parsed{
				UID:     uid,
				Rev:     now.UTC().Format(time.RFC3339),
				Raw:     vcardBytes,
				Contact: c,
			},
			Href: href,
			ETag: ifMatch, // pin to old etag until server confirms
		}
		if err := upsertContactTx(ctx, tx, bookHref, stored, now.Unix()); err != nil {
			return err
		}
		if err := reconcileRecipientsTx(ctx, tx, uid, oldEmails, c.Emails); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO outbox (folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
            VALUES (NULL, NULL, ?, ?, ?, ?, ?, 0, NULL)`,
			string(KindContactPut), string(body), vcardBytes, now.UnixNano(), OpPending)
		return err
	})
	if err != nil {
		return err
	}
	a.signalDrainer()
	return nil
}

// QueueContactDelete tombstones the local row and enqueues a CardDAV
// DELETE. The contact's href and etag are read inside the same tx.
func (a *Account) QueueContactDelete(ctx context.Context, uid string) error {
	now := time.Now()
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var href, etag string
		if err := tx.QueryRowContext(ctx,
			`SELECT href, etag FROM contacts WHERE uid = ?`, uid).Scan(&href, &etag); err != nil {
			return fmt.Errorf("lookup contact %s: %w", uid, err)
		}
		args := ContactDeleteArgs{Href: href, IfMatch: etag}
		body, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("encode args: %w", err)
		}
		// Cascades drop contact_emails / contact_phones via FK.
		if _, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE uid = ?`, uid); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
            INSERT INTO outbox (folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
            VALUES (NULL, NULL, ?, ?, NULL, ?, ?, 0, NULL)`,
			string(KindContactDelete), string(body), now.UnixNano(), OpPending)
		return err
	})
	if err != nil {
		return err
	}
	a.signalDrainer()
	return nil
}

// loadEmailsTx returns the stored emails for uid in pref order. Used
// by reconcileRecipientsTx to diff old-vs-new addresses.
func loadEmailsTx(ctx context.Context, tx *sql.Tx, uid string) ([]contacts.Email, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT address, label FROM contact_emails WHERE contact_uid = ? ORDER BY pref ASC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []contacts.Email
	for rows.Next() {
		var e contacts.Email
		if err := rows.Scan(&e.Address, &e.Label); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// reconcileRecipientsTx is a no-op stub today: contact_emails is the
// only address-keyed pivot, and upsertContactTx already replaces it.
// message_recipients rows are keyed by (message_uid, role, address);
// changing a contact's emails does not invalidate any historical row
// (the address still appeared in that historical message). Future
// passes that add a contact_uid foreign-key column to
// message_recipients will populate it here.
func reconcileRecipientsTx(_ context.Context, _ *sql.Tx, _ string, _ []contacts.Email, _ []contacts.Email) error {
	return nil
}
```

> Note on the recipient reconciliation: re-reading the schema, the
> projection table `message_recipients` does not carry a `contact_uid`
> column — the contact↔address join goes through `contact_emails` at
> read time (see `suggestQuery` and `LookupContact`). So replacing the
> `contact_emails` rows (which `upsertContactTx` already does)
> *is* the reconciliation. The stub above documents that and gives
> future passes a hook. The spec's "drop/insert recipient rows"
> language assumed a `contact_uid` column the schema lacks; the v8
> backfill in schema.go confirms the join shape. The behavior the
> spec asked for (no stale contact↔address mapping after edit) holds
> for free under the existing schema.

(Add `_ "github.com/glw907/poplar/internal/cache"` if needed; you
already import `internal/contacts` and stdlib `database/sql`,
`encoding/json`, `time`. Add `encoding/json` and `time` if absent.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cache/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/contacts.go internal/cache/contacts_test.go
git commit -m "Pass 9m.1: queue contact PUT + DELETE through outbox" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Drainer dispatch arms

Wire the new kinds to `Account.ContactsWriter` and to the existing
conflict matrix.

**Files:**
- Modify: `internal/cache/drainer.go`
- Modify: `internal/cache/conflicts_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/cache/conflicts_test.go`:

```go
type fakeContactsWriter struct {
	putErr    error
	delErr    error
	puts      int
	deletes   int
	multiget  func(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error)
	newHref   string
	newETag   string
}

func (f *fakeContactsWriter) PutAddressObject(ctx context.Context, href, ifMatch string, body []byte) (string, string, error) {
	f.puts++
	if f.putErr != nil {
		return "", "", f.putErr
	}
	if f.newHref != "" {
		return f.newHref, f.newETag, nil
	}
	return href, f.newETag, nil
}
func (f *fakeContactsWriter) DeleteAddressObject(ctx context.Context, href, ifMatch string) error {
	f.deletes++
	return f.delErr
}
func (f *fakeContactsWriter) Multiget(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error) {
	if f.multiget != nil {
		return f.multiget(ctx, bookHref, hrefs)
	}
	return nil, nil
}

func TestDrainer_ContactPut_Success(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	w := &fakeContactsWriter{newETag: `"new"`}
	a.ContactsWriter = w
	ctx := context.Background()

	seedContact(t, a, "u1", `"old"`)
	args, _ := json.Marshal(ContactPutArgs{BookHref: "/b/", Href: "/b/u1.vcf", IfMatch: `"old"`})
	mustExec(t, a, `INSERT INTO outbox(folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
		VALUES (NULL, NULL, ?, ?, x'', 0, ?, 0, NULL)`, string(KindContactPut), string(args), OpPending)

	a.drainOnce(ctx, defaultDrainerConfig())

	if w.puts != 1 {
		t.Fatalf("puts=%d want 1", w.puts)
	}
	var status, etag string
	_ = a.db.QueryRow(`SELECT status FROM outbox`).Scan(&status)
	_ = a.db.QueryRow(`SELECT etag FROM contacts WHERE uid='u1'`).Scan(&etag)
	if status != string(OpDone) {
		t.Errorf("status=%q want done", status)
	}
	if etag != `"new"` {
		t.Errorf("etag=%q want new", etag)
	}
}

func TestDrainer_ContactPut_PreconditionConflict(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	w := &fakeContactsWriter{putErr: contacts.ErrPreconditionFailed}
	a.ContactsWriter = w
	ctx := context.Background()

	seedContact(t, a, "u1", `"stale"`)
	args, _ := json.Marshal(ContactPutArgs{BookHref: "/b/", Href: "/b/u1.vcf", IfMatch: `"stale"`})
	mustExec(t, a, `INSERT INTO outbox(folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
		VALUES (NULL, NULL, ?, ?, x'', 0, ?, 0, NULL)`, string(KindContactPut), string(args), OpPending)

	a.drainOnce(ctx, defaultDrainerConfig())

	var status string
	_ = a.db.QueryRow(`SELECT status FROM outbox`).Scan(&status)
	if status != string(OpConflict) {
		t.Errorf("status=%q want conflict", status)
	}
}

func TestDrainer_ContactDelete_NotFoundIsSuccess(t *testing.T) {
	a := newTestAccount(t)
	defer a.Close()
	a.ContactsWriter = &fakeContactsWriter{delErr: contacts.ErrNotFound}
	ctx := context.Background()

	args, _ := json.Marshal(ContactDeleteArgs{Href: "/b/u1.vcf", IfMatch: `"e"`})
	mustExec(t, a, `INSERT INTO outbox(folder, message, kind, args, payload, enqueued_at, status, attempts, next_eligible_at)
		VALUES (NULL, NULL, ?, ?, NULL, 0, ?, 0, NULL)`, string(KindContactDelete), string(args), OpPending)

	a.drainOnce(ctx, defaultDrainerConfig())

	var status string
	_ = a.db.QueryRow(`SELECT status FROM outbox`).Scan(&status)
	if status != string(OpDone) {
		t.Errorf("status=%q want done", status)
	}
}
```

`seedContact` is a small helper (add at the bottom of the test file
if absent):

```go
func seedContact(t *testing.T, a *Account, uid, etag string) {
	t.Helper()
	mustExec(t, a, `INSERT INTO addressbooks(href, display_name, description, sync_token, ctag, supports_sync, last_synced_at) VALUES ('/b/', 'Default', '', '', '', 1, 0)`)
	mustExec(t, a, `INSERT INTO contacts(uid, addressbook_href, href, etag, vcard, rev, fn, family, given, org, title, note, last_synced_at)
		VALUES (?, '/b/', '/b/'||?||'.vcf', ?, x'', '', '', '', '', '', '', '', 0)`,
		uid, uid, etag)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cache/ -run 'Drainer_Contact' -v`
Expected: FAIL — dispatch returns `unknown args` for the new kinds.

- [ ] **Step 3: Add the dispatch arms**

Edit `internal/cache/drainer.go` — extend `dispatch`:

```go
	case ContactPutArgs:
		if a.ContactsWriter == nil {
			return errors.New("contacts writer not wired")
		}
		newHref, newETag, err := a.ContactsWriter.PutAddressObject(
			context.Background(), v.Href, v.IfMatch, row.Payload)
		if err != nil {
			return err
		}
		// Stash the post-success values in args so finalizeSuccess can
		// write them back. Easiest path: do the cache write here.
		_, dbErr := a.db.Exec(
			`UPDATE contacts SET href = ?, etag = ? WHERE href = ?`,
			newHref, newETag, v.Href)
		return dbErr
	case ContactDeleteArgs:
		if a.ContactsWriter == nil {
			return errors.New("contacts writer not wired")
		}
		return a.ContactsWriter.DeleteAddressObject(
			context.Background(), v.Href, v.IfMatch)
```

Extend `executeOne` — add sentinel routing alongside `mail.ErrAuth` /
`mail.ErrNotFound`:

```go
	case errors.Is(dispatchErr, contacts.ErrAuth):
		_ = a.finishOp(row.ID, OpConflict, encodeErr("auth-failure", dispatchErr), 0)
		a.publish(row, OpConflict, dispatchErr)
	case errors.Is(dispatchErr, contacts.ErrPreconditionFailed):
		_ = a.finishOp(row.ID, OpConflict, encodeErr("precondition-failed", dispatchErr), 0)
		a.publish(row, OpConflict, dispatchErr)
	case errors.Is(dispatchErr, contacts.ErrNotFound):
		_ = a.finalizeSuccess(ctx, row, args)
		a.publish(row, OpDone, nil)
```

(Place these *before* the existing `mail.ErrAuth` case so order
matches the import — or use `||` chaining if a single arm makes
sense. Either works; the test pins behavior.)

Add the import `"github.com/glw907/poplar/internal/contacts"` to
`drainer.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cache/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cache/drainer.go internal/cache/conflicts_test.go
git commit -m "Pass 9m.1: drainer dispatch + conflict routing for contacts" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Form `D` key + delete confirm message

Add `D` to the Form keymap, gate on existing UID, emit
`OpenContactDeleteConfirmMsg`.

**Files:**
- Modify: `internal/ui/contacts/keys.go`
- Modify: `internal/ui/contacts/msgs.go`
- Modify: `internal/ui/contacts/form.go`
- Modify: `internal/ui/contacts/form_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/contacts/form_test.go`:

```go
func TestForm_D_InertOnNewContact(t *testing.T) {
	f := NewForm(NewStyles(theme.Default()), Contact{Kind: KindPerson}, false, []string{"local"})
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if cmd != nil {
		t.Fatal("D on new contact must be inert")
	}
}

func TestForm_D_OpensConfirmOnExisting(t *testing.T) {
	c := Contact{Kind: KindPerson, Given: "Ada", Name: "Ada"}
	c.Emails = []Email{{Address: "ada@x.example"}}
	// existing-contact marker: NewForm doesn't take a UID arg today,
	// so we use a sentinel field. The test imports the same symbol
	// the production code reads. See the implementation step.
	f := NewForm(NewStyles(theme.Default()), c, false, []string{"local"})
	f = f.WithExistingUID("u-1")
	_, cmd := f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if cmd == nil {
		t.Fatal("D on existing contact must emit OpenContactDeleteConfirmMsg")
	}
	msg := cmd()
	open, ok := msg.(OpenContactDeleteConfirmMsg)
	if !ok {
		t.Fatalf("got %T want OpenContactDeleteConfirmMsg", msg)
	}
	if open.UID != "u-1" || open.DisplayName != "Ada" {
		t.Errorf("got %+v", open)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui/contacts/ -run 'Form_D_' -v`
Expected: FAIL — `WithExistingUID`, `OpenContactDeleteConfirmMsg` undefined.

- [ ] **Step 3: Add the message types**

Edit `internal/ui/contacts/msgs.go` — append:

```go
// OpenContactDeleteConfirmMsg asks App to open the deletion confirm
// modal for an existing contact. Form emits this; App handles the
// confirm cascade and emits ContactDeleteMsg on Yes.
type OpenContactDeleteConfirmMsg struct {
	UID         string
	DisplayName string
}

// ContactDeleteMsg fires after the user confirms deletion. App routes
// it to cache.QueueContactDelete and dismisses the form.
type ContactDeleteMsg struct{ UID string }
```

- [ ] **Step 4: Add the key binding**

Edit `internal/ui/contacts/keys.go`:

```go
	D key.Binding
```

In the literal:

```go
	D: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete contact")),
```

- [ ] **Step 5: Wire `D` and the existing-UID seam in Form**

Edit `internal/ui/contacts/form.go`. Add the field to `Form`:

```go
	existingUID string
```

Add a setter (callers — App, tests — populate it after `NewForm`):

```go
// WithExistingUID marks the form as editing an existing contact.
// Empty UID means "new contact" and disables the Delete key.
func (f Form) WithExistingUID(uid string) Form {
	f.existingUID = uid
	return f
}
```

In `Update`, before the focused-widget switch (after the
`Tab/Shift+Tab` arm), intercept `D`:

```go
	if k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] == 'D' {
		if f.existingUID == "" {
			return f, nil
		}
		// Only intercept when no input has focus eating the keystroke;
		// the focused-widget switch below handles the input case.
		if !f.focusedIsTextInput() {
			return f, func() tea.Msg {
				return OpenContactDeleteConfirmMsg{
					UID:         f.existingUID,
					DisplayName: f.currentContact().Name,
				}
			}
		}
	}
```

Add `focusedIsTextInput()` (returns true when the focused widget
accepts text — name/email/phone/title/note inputs). If the form
already has an analog (`focusedWidget().kind`), reuse that:

```go
func (f Form) focusedIsTextInput() bool {
	w := f.focusedWidget()
	return w.kind == widInput || w.kind == widNote
}
```

(Adjust the `wid*` constants to whatever exists in the file.)

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/ui/contacts/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/contacts/keys.go internal/ui/contacts/msgs.go \
        internal/ui/contacts/form.go internal/ui/contacts/form_test.go
git commit -m "Pass 9m.1: Form D-key + delete confirm message" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: App wiring — save, delete, confirm cascade

`ContactSaveMsg` becomes the gateway to `QueueContactPut`;
`OpenContactDeleteConfirmMsg` opens a confirm; `ContactDeleteMsg`
calls `QueueContactDelete`. The confirm cascade gains a contact-delete
arm in front of compose-save (matching the priority order from
ui-invariants).

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/contacts/form.go` (only to populate `existingUID`
  on the construction site — App calls `WithExistingUID` after
  `NewForm`).

- [ ] **Step 1: Read the existing handler and confirm cascade**

Skim `internal/ui/app.go` lines 285–420 (already in context). The
relevant landmarks:

- `case contacts.OpenFormMsg:` (line 291) — extend to call
  `WithExistingUID(msg.Initial.UID)` once Initial carries UID, OR
  — since `Contact` has no UID field today — pass it via a new
  field on `OpenFormMsg`.

Choose: add `UID string` to `OpenFormMsg`. Initial source for
`OpenFormMsg` is the popover's "edit" path; the popover knows the
UID via `cache.LookupContact` (UID returned via a new return value
or stashed on the popover row). Cleaner: extend `LookupContact` to
return the UID. Check call sites:

```bash
grep -n 'LookupContact' internal/ui/ -r
```

(Today `LookupContact` discards the UID; the test's `WithExistingUID`
seam is meant for the App to populate after the lookup.)

- [ ] **Step 2: Extend LookupContact to return UID**

Edit `internal/cache/contacts.go`:

```go
// LookupContact returns the cached contact for an email address.
// Returns (Contact, uid, ok). ok=false means no match.
func (a *Account) LookupContact(ctx context.Context, address string) (contacts.Contact, string, bool) {
	const q = `
SELECT c.uid, c.fn, c.family, c.given, c.org, c.title, c.note
  FROM contact_emails ce
  JOIN contacts c ON c.uid = ce.contact_uid
 WHERE LOWER(ce.address) = LOWER(?)
 LIMIT 1
`
	var uid string
	var c contacts.Contact
	err := a.db.QueryRowContext(ctx, q, address).Scan(
		&uid, &c.Name, &c.Family, &c.Given, &c.Org, &c.Title, &c.Note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return contacts.Contact{}, "", false
	}
	if err != nil {
		return contacts.Contact{}, "", false
	}
	if c.Family == "" && c.Given == "" && c.Org != "" {
		c.Kind = contacts.KindOrg
	}
	c.Emails = a.loadEmails(ctx, uid)
	c.Phones = a.loadPhones(ctx, uid)
	return c, uid, true
}
```

Update existing call sites in `internal/ui/`:

```bash
grep -rn 'LookupContact' internal/
```

Each call site that previously did `c, ok := acct.LookupContact(...)`
becomes `c, uid, ok := acct.LookupContact(...)`. The popover should
stash `uid` and pass it on `OpenFormMsg`.

- [ ] **Step 3: Add UID to OpenFormMsg**

Edit `internal/ui/contacts/msgs.go`:

```go
type OpenFormMsg struct {
	Initial     Contact
	UID         string // empty for new contacts
	FromPopover bool
}
```

Update the popover emit site (find via grep) to fill `UID`. Update
`form_test.go` callers if they construct `OpenFormMsg` directly.

- [ ] **Step 4: Wire the App handlers**

Edit `internal/ui/app.go`. Replace the existing
`case contacts.OpenFormMsg:` body with one that threads UID:

```go
	case contacts.OpenFormMsg:
		m.popover = nil
		saveTo := []string{"Local file"}
		if email := m.acct.AccountEmail(); email != "" {
			saveTo = append(saveTo, email)
		}
		f := contacts.NewForm(m.contactsStyles, msg.Initial, msg.FromPopover, saveTo).
			WithExistingUID(msg.UID)
		w, h := m.formSize(msg.FromPopover)
		f = f.SetSize(w, h)
		m.form = &f
		return m, nil
```

Replace the `case contacts.ContactSaveMsg:` body with the queue path:

```go
	case contacts.ContactSaveMsg:
		uid := ""
		if m.form != nil {
			uid = m.form.ExistingUID() // add this getter — see below
		}
		f := m.form
		m.form = nil
		return m, queueContactPutCmd(m.acct.Cache(), uid, msg.Contact, msg.SaveTo, f)
```

(Define `queueContactPutCmd` in `internal/ui/cmds.go` — see step 5.)

Add a new arm for `OpenContactDeleteConfirmMsg`:

```go
	case contacts.OpenContactDeleteConfirmMsg:
		m.pendingContactDelete = msg.UID
		m.confirm = m.confirm.Open(ConfirmRequest{
			Title: "Delete contact?",
			Body:  msg.DisplayName + " will be removed from this address book.",
		})
		return m, nil
```

Add the field on `App`:

```go
	pendingContactDelete string
```

Extend `ConfirmModalYesMsg`:

```go
		switch {
		case m.pendingFormDiscard:
			m.pendingFormDiscard = false
			m.form = nil
			return m, nil
		case m.pendingContactDelete != "":
			uid := m.pendingContactDelete
			m.pendingContactDelete = ""
			m.form = nil
			return m, queueContactDeleteCmd(m.acct.Cache(), uid)
		case m.pendingComposeSave:
			// ... unchanged
		case m.pendingEmpty.folder != "":
			// ... unchanged
		}
```

Extend `ConfirmModalNoMsg` and `ConfirmModalClosedMsg` to clear
`pendingContactDelete`:

```go
	if m.pendingContactDelete != "" {
		m.pendingContactDelete = ""
		// keep the form mounted so the user can edit further
		return m, nil
	}
```

- [ ] **Step 5: Add the cmds**

Edit `internal/ui/cmds.go`. Append:

```go
// queueContactPutCmd patches (or builds) the vCard for c, then enqueues
// a CardDAV PUT through the cache outbox.
func queueContactPutCmd(
	c *cache.Account,
	uid string,
	contact corecontacts.Contact,
	saveTo string,
	f *contacts.Form, // nil-safe; only used to recover ETag on edits
) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		bookHref, err := resolveBookHref(ctx, c, saveTo)
		if err != nil {
			return uicore.ErrorMsg{Op: "save contact", Err: err}
		}
		var (
			vcardBytes []byte
			href       string
			ifMatch    string
		)
		if uid == "" {
			uid = newContactUID()
			vcardBytes, err = corecontacts.BuildVCard(contact, uid, time.Now())
			if err != nil {
				return uicore.ErrorMsg{Op: "save contact", Err: err}
			}
		} else {
			stored, e, h, err := loadStoredVCard(ctx, c, uid)
			if err != nil {
				return uicore.ErrorMsg{Op: "save contact", Err: err}
			}
			href, ifMatch = h, e
			vcardBytes, err = corecontacts.PatchVCard(stored, contact, time.Now())
			if err != nil {
				return uicore.ErrorMsg{Op: "save contact", Err: err}
			}
		}
		if err := c.QueueContactPut(ctx, bookHref, uid, href, ifMatch, contact, vcardBytes); err != nil {
			return uicore.ErrorMsg{Op: "save contact", Err: err}
		}
		return nil
	}
}

// queueContactDeleteCmd queues a CardDAV DELETE for uid.
func queueContactDeleteCmd(c *cache.Account, uid string) tea.Cmd {
	return func() tea.Msg {
		if err := c.QueueContactDelete(context.Background(), uid); err != nil {
			return uicore.ErrorMsg{Op: "delete contact", Err: err}
		}
		return nil
	}
}

// resolveBookHref maps the form's "Save to" string to a CardDAV book
// href. The first cached book is the default for v1; multi-book
// destination selection is post-1.0.
func resolveBookHref(ctx context.Context, c *cache.Account, _ string) (string, error) {
	books, err := c.Books(ctx)
	if err != nil {
		return "", err
	}
	for href := range books {
		return href, nil
	}
	return "", errors.New("no address book configured")
}

// loadStoredVCard fetches the stored vCard blob, etag, and href for uid.
func loadStoredVCard(ctx context.Context, c *cache.Account, uid string) (raw []byte, etag, href string, err error) {
	row := c.DB().QueryRowContext(ctx,
		`SELECT vcard, etag, href FROM contacts WHERE uid = ?`, uid)
	err = row.Scan(&raw, &etag, &href)
	return
}

// newContactUID returns a fresh RFC 4122 UUID for a new contact.
func newContactUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
```

(Add imports `crypto/rand`, `errors`, `time`,
`github.com/glw907/poplar/internal/contacts/...` aliased as the file
already does — `corecontacts` for the domain package, `contacts` for
the UI subpackage. Add `c.DB()` if not exposed: a one-line accessor
on `cache.Account`:

```go
// DB returns the underlying *sql.DB. Read-only callers in internal/ui
// use this for ad-hoc queries kept out of the typed surface.
func (a *Account) DB() *sql.DB { return a.db }
```

…or, cleaner, push `loadStoredVCard` into `cache/contacts.go`. Pick
the cleaner path: add a `LoadStoredVCard(ctx, uid) (raw, etag, href, error)`
method on `cache.Account`.)

Add the `ExistingUID()` getter on Form (`internal/ui/contacts/form.go`):

```go
func (f Form) ExistingUID() string { return f.existingUID }
```

- [ ] **Step 6: Build + check**

Run: `make check`
Expected: PASS. If anything fails, fix and re-run.

- [ ] **Step 7: Live tmux smoke test**

```bash
make install
```

Then in a tmux pane (see `.claude/docs/tmux-testing.md`): launch
`poplar`, open Contacts mode, edit an existing contact, save —
verify the local view updates and the outbox shows the queued op
(`Q`). Watch the drainer succeed against the live Fastmail
account; then re-open the contact and confirm the change persisted.

For delete: edit, press `D`, accept the confirm, watch outbox drain.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/app.go internal/ui/cmds.go internal/ui/contacts/form.go \
        internal/ui/contacts/msgs.go internal/cache/contacts.go
git commit -m "Pass 9m.1: wire form save + delete to outbox" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: ContactsWriter wiring at startup

App constructs `*contacts.Client` (already does for sync) and
attaches it to `Account.ContactsWriter`. Without this, the drainer
hits the "contacts writer not wired" guard.

**Files:**
- Modify: `internal/ui/app.go` (or wherever `acct.SyncContacts` is
  set up — `internal/ui/cmds.go:439` per earlier grep).
- Modify: `internal/cache/contacts.go` — keep `SyncContacts` working
  with the same long-lived client.

- [ ] **Step 1: Build the client once at startup**

Find the App init that creates `contactsCfg` (around `app.go:141`).
Right after the cfg is built, construct the client and attach it:

```go
	if app.contactsCfg != nil {
		cl, err := corecontacts.NewClient(
			app.contactsCfg.URL,
			app.contactsCfg.Username,
			app.contactsCfg.Password,
			app.contactsCfg.InsecureTLS,
		)
		if err != nil {
			// Surface as a startup banner; sync + writeback both disabled.
			app.startupErr = fmt.Errorf("contacts client: %w", err)
		} else {
			app.acct.ContactsWriter = cl
			app.contactsClient = cl // reused by SyncContacts to avoid rebuilding
		}
	}
```

(`startupErr` already exists or is added the same way the existing
config-load errors are surfaced. If not, fall back to logging via
the error banner Cmd.)

- [ ] **Step 2: Make SyncContacts use the cached client**

Edit `internal/cache/contacts.go`:

```go
// SyncContacts runs one CardDAV sync pass using ContactsWriter when
// available. cfg is retained for the legacy code path that builds a
// client per call; new callers wire ContactsWriter at Open.
func (a *Account) SyncContacts(ctx context.Context, cfg *contacts.ClientConfig) error {
	if a.ContactsWriter != nil {
		if cl, ok := a.ContactsWriter.(*contacts.Client); ok {
			return contacts.Sync(ctx, cl, a)
		}
	}
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

- [ ] **Step 3: Verify**

```bash
make check
```

Expected: PASS.

- [ ] **Step 4: Live tmux smoke test (writer wiring)**

Re-run the Task 7 smoke test. Edit + save against the live Fastmail
account; outbox should drain to OpDone (not stick at OpFailed with
"contacts writer not wired").

- [ ] **Step 5: Commit**

```bash
git add internal/ui/app.go internal/cache/contacts.go
git commit -m "Pass 9m.1: wire ContactsWriter at startup" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Pass-end ritual

Standard `poplar-pass` close-out.

- [ ] **Step 1: Run /simplify**

Invoke the `simplify` skill. Apply genuine wins, skip churn.

- [ ] **Step 2: Idiomatic-bubbletea check**

The pass touched `internal/ui/contacts/form.go` and `internal/ui/app.go`.
Walk the `bubbletea-conventions.md §10` checklist against those diffs.
Capture an 80×24 + 120×40 tmux session showing the Form, the delete
confirm, and the outbox after a queued contact op.

- [ ] **Step 3: Write ADR**

Single ADR (the Put + Delete + patch decisions are tightly coupled).
File: `docs/poplar/decisions/0176-carddav-writeback.md`. Cover:

- Two new outbox kinds; conflict matrix shape (412 → conflict, 404
  → idempotent success, 401/403 → auth-failure).
- Patch-not-rebuild policy and the email/phone matching rule
  (address equality, then E.164 equality, canonical TYPE labels for
  added rows, PREF re-derived from index 0).
- `contacts.Writer` seam on `cache.Account`.
- Recipient-projection note: schema already routes contact↔address
  through `contact_emails`, which `upsertContactTx` rewrites — no
  separate `message_recipients` work needed today.

- [ ] **Step 4: Update invariants.md**

In `docs/poplar/invariants.md`:

- Extend the "Catkin" / "Compose" / "Contacts" / outbox sections
  describing op kinds: add `KindContactPut` + `KindContactDelete`
  to the `OpKind` enumeration; add `ContactPutArgs` + `ContactDeleteArgs`
  to the `OpArgs` sealed sum.
- Update the contacts surface paragraph (the one mentioning
  `LookupContact` and the i-popover): note that `LookupContact`
  now returns `(Contact, uid, ok)`.
- Update the Form paragraph: `D` is wired (drop "inert until 9.3"),
  emits `OpenContactDeleteConfirmMsg`, gated on existing UID.
- Add one sentence to the cache invariants: `Account.ContactsWriter`
  is the CardDAV write seam; drainer dispatches contact ops against
  it; sentinels in `internal/contacts` route through `errors.Is`.

Update `docs/poplar/decisions/INDEX.md` with a row for ADR-0176.

- [ ] **Step 5: Update STATUS.md**

Mark Pass 9m.1 done. Replace the starter prompt with the next pass
(Pass 9n — signatures + identities, per the existing pass table).

- [ ] **Step 6: Archive plan + spec**

```bash
git mv docs/superpowers/plans/2026-05-07-carddav-writeback.md \
       docs/superpowers/archive/plans/
git mv docs/superpowers/specs/2026-05-07-carddav-writeback-design.md \
       docs/superpowers/archive/specs/
```

- [ ] **Step 7: make check + commit + push + install**

```bash
make check
git add -A
git commit -m "Pass 9m.1: ADR-0176, invariants update, archive plan + spec" -m "Co-Authored-By: Claude <noreply@anthropic.com>"
git push
make install
```

---

## Self-Review

**Spec coverage check:**

- BuildVCard / PatchVCard policy → Task 1 ✓
- Client PUT/DELETE + sentinels → Task 2 ✓
- Op kinds + Writer seam → Task 3 ✓
- QueueContactPut / QueueContactDelete → Task 4 ✓ (recipient
  reconciliation collapses to a no-op given current schema; Task 4
  documents this with an in-code note and the ADR captures the
  shape)
- Drainer dispatch + conflict matrix → Task 5 ✓
- Form D-key + delete confirm → Task 6 ✓
- App wiring (save, delete, confirm cascade) → Task 7 ✓
- ContactsWriter wired at startup → Task 8 ✓
- Pass-end ritual → Task 9 ✓

**Type consistency:**

- `ContactPutArgs{BookHref, Href, IfMatch}` used in Tasks 3, 4, 5,
  7 — same shape throughout.
- `ContactDeleteArgs{Href, IfMatch}` used in Tasks 3, 4, 5 — same
  shape.
- `Writer` interface (Tasks 3, 5, 8) — `PutAddressObject(ctx,
  href, ifMatch, body) (newHref, newETag, error)` matches the
  client implementation in Task 2.
- `LookupContact` signature changes from `(Contact, bool)` to
  `(Contact, string, bool)` in Task 7; all call sites updated in
  the same task.

**Placeholder scan:** every step has concrete code or an exact
command. The recipient-reconciliation reduction to a no-op is
explicitly justified in the Task 4 implementation note rather than
left as TBD.

**Scope check:** 9 tasks, one ADR, fits the 8–12-task budget.
