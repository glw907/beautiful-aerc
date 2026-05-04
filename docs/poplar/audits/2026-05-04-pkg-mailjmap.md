# Human-voice audit — `internal/mailjmap/`

**Tally:** C1=11 (T1=4, T2=5, T5=2), C4=2 (T3/T31=1, T7=1), C6=3 (T9=2, T24=1), C7=6 (T10b=2, T11=3, T13=1)

Total: 22 findings.

---

## C1 — Comment rot

### jmap.go:514 / jmap.go:575 / push.go:279 / jmap.go:737 — C1 (T2)

```go
// translateEmail converts a JMAP *email.Email into mail.MessageInfo.
func translateEmail(e *email.Email) mail.MessageInfo {

// translateKeywords maps JMAP keyword strings to mail.Flag bits.
func translateKeywords(kw map[string]bool) mail.Flag {

// idsToUIDs converts a []jmap.ID to []mail.UID.
func idsToUIDs(ids []jmap.ID) []mail.UID {

// keywordForFlag maps a mail.Flag to its JMAP keyword string.
func keywordForFlag(flag mail.Flag) (string, error) {
```

Four unexported translation helpers; name + signature say everything.

### jmap.go:536–538 — C1 (T2)

```go
// formatFromList formats a list of JMAP addresses into a display string.
// Multiple senders are joined with ", ". Each address uses the display
// name if present, otherwise falls back to the email address.
```

Three sentences; the latter two restate the body.

### jmap.go:193–238 — C1 (T1)

```go
// --- Phase 1: resolve password (no lock) ---
// --- Phase 2: authenticate (no lock) ---
// --- Phase 3: fetch folders (no lock) ---
// --- Phase 4: seed email state (no lock) ---
// --- Phase 5: build local values (no lock) ---
// --- Phase 6: install state under lock, then spawn goroutine ---
```

Six banner labels inside `Connect`. Each restates the call beneath. Only the lock annotation is non-trivial; one line above the function suffices.

### push.go:242, 249 / jmap.go:314 — C1 (T1)

```go
// Emit UpdateFolderInfo for each changed mailbox ID.
affected := make([]jmap.ID, 0, ...)
// Build reverse map: jmap id → folder display name.
idToName := make(map[string]string, len(b.folders))
// Build raw mail.Folder slice to run through the classifier.
raw := make([]mail.Folder, 0, len(gr.List))
```

Each comment restates the next two lines. Variable names + immediate use already convey intent.

### jmap.go:727–729 — C1 (T5)

```go
// Send satisfies mail.Backend. Compose is planned for Pass 9.
func (b *Backend) Send(_ string, _ []string, _ io.Reader) error {
    return errors.New("send not implemented in pass 3 — see pass 9")
}
```

"Pass 9" / "pass 3" — pass labels in both godoc and error string. Belongs in a TODO or commit message.

### fake_test.go:30–32 — C1 (T5)

```go
// fakeResponse constructs a *jmap.Response whose Responses slice
// contains the given invocations. Tasks 10–14 use this to inject
// canned method responses into fakeClient.
```

"Tasks 10–14" is dev-plan reference, not contract.

---

## C4 — Uniform verbosity

### jmap.go + attachments.go (15 methods) — C4 (T3 / T31)

```go
// AccountName satisfies mail.Backend. The cfg.Name fallback only surfaces during the pre-Connect window …
// AccountEmail satisfies mail.Backend.
// Updates satisfies mail.Backend. Returns a nil channel before Connect succeeds.
// ListFolders satisfies mail.Backend. It returns the cached folder map …
// Flag satisfies mail.Backend. It sets or clears a JMAP keyword for each uid.
// Send satisfies mail.Backend. Compose is planned for Pass 9.
```

15 exported methods open with identical `"X satisfies mail.Backend."` boilerplate regardless of body length (1 line vs 60 lines).

### push.go (unexported function bank) — C4 (T31)

`pushLoop`, `runEventSource`, `dispatchEmailChanges`, `dispatchMailboxChanges`, `emit`, `idsToUIDs` all carry 1–3 sentence godocs in identical shape. `idsToUIDs` (3-line trivial converter) gets same weight as `runEventSource` (EventSource lifecycle).

### jmap.go:129–131 — C4 (T7)

```go
// Updates satisfies mail.Backend. Returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }
```

One-line getter; two-sentence godoc matching the weight given to substantive methods.

---

## C6 — Test boilerplate

### jmap_test.go:526–703 — C6 (T24)

```go
// TestHandleStateChange_Dedup verifies that calling handleStateChange
// twice with the same state only triggers the dispatcher once.

// TestHandleStateChange_StatePreservedOnError verifies that b.states
// is not advanced when the dispatcher returns an error.

// TestEmit_BufferFullDrop verifies that emit does not block or panic
// when the updates channel is full.

// TestDispatchEmailChanges_EmitsCorrectUpdates verifies that
// dispatchEmailChanges emits NewMail, FlagsChanged, and Expunge.

// TestDispatchMailboxChanges_EmitsFolderInfo verifies that
// dispatchMailboxChanges emits UpdateFolderInfo for each affected mailbox.
```

Seven tests with identical "TestX verifies that …" docstring opener. Test name already encodes the assertion.

### jmap_test.go:84, 117 — C6 (T9)

```go
{name: "unknown folder returns error", openName: "Nonexistent", wantErr: true},
{name: "happy path returns UIDs and total", …},
```

Predicate-form `name:` fields; the sibling `"known folder"` in the same table shows the correct shape.

---

## C7 — Error phrasing

### jmap.go:715, 806 — C7 (T10b)

```go
// checkEmailSetDestroyed:
return fmt.Errorf("no Email/set response")
// checkEmailSetUpdated:
return fmt.Errorf("no Email/set response")
```

Sibling functions, identical sentinel string. Should differentiate by verb: `"no Email/set response for destroy"` vs `"… for update"`.

### push.go:156, 167 / changes.go:47, 57 — C7 (T10b)

```go
// dispatchEmailChanges (push.go):
return fmt.Errorf("email/changes: %w", err)
return fmt.Errorf("email/changes: no response")

// Changes (changes.go):
return mail.ChangeSet{}, since, fmt.Errorf("email/changes: %w", err)
return mail.ChangeSet{}, since, errors.New("email/changes: no response")
```

Cross-file chorus on the same JMAP op. Push and cache-sync paths deserve distinct prefixes.

### jmap.go:658, 687, 782 — C7 (T11)

```go
// Move:    return fmt.Errorf("move: %w", err)         // both transport and checker errors
// Destroy: return fmt.Errorf("destroy: %w", err)      // same
// setKeyword: return fmt.Errorf("set keyword %s: %w", keyword, err)  // same
```

Adjacent transport and response-checker errors use identical template in three functions; failures are genuinely distinct (transport vs server-reported `NotUpdated`/`NotDestroyed`).

### jmap.go:196, 205, 212, 218, 224 — C7 (T13)

```go
return fmt.Errorf("connect: %w", err)
return fmt.Errorf("connect: authenticate: %w", err)
return fmt.Errorf("connect: list folders: %w", err)
return fmt.Errorf("connect: seed email state: %w", err)
return fmt.Errorf("connect: init body cache: %w", err)
```

No caller of `Connect` calls `errors.Is`/`errors.As`. `%v` is correct.
