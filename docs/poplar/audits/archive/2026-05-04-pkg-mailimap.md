# Human-voice audit — `internal/mailimap/`

**Tally:** C1=4 (T1=2, T2=1, T5=1), C4=5 (T3/T31=2, T7=2, T17=1), C6=3 (T9=2, T24=1), C7=5 (T10=1, T10b=2, T11=1, T13=1), C8=1 (T19=1)

Total: 18 findings.

---

## C1 — Comment rot

### realclient.go:43–44 — C1 (T1)

```go
// imapUID converts imap.UID (uint32) to mail.UID (decimal string).
func imapUID(u imap.UID) mail.UID {
```

Prose translation of the signature. Adds nothing.

### realclient.go:48–49 — C1 (T1)

```go
// mailUIDsToSet converts a slice of mail.UID (decimal string) to an imap.UIDSet.
func mailUIDsToSet(uids []mail.UID) imap.UIDSet {
```

Same pattern. Function name + signature say everything.

### realclient.go:105–112 — C1 (T2)

```go
// attrsToStrings converts a slice of imap.MailboxAttr to plain strings.
func attrsToStrings(attrs []imap.MailboxAttr) []string {
```

Zero-information godoc on unexported trivial converter.

### client.go:17–19 — C1 (T5)

```go
// Method signatures will be fleshed out as each task lands. Each
// method should return errors with the wrapped IMAP server response
// when applicable so the error banner can surface useful detail.
```

PR-description voice describing development workflow. Belongs in a commit message.

---

## C4 — Uniform verbosity

### realclient.go:364, 509, 579 — C4 (T3 / T31)

Three functions with structurally identical three-error sequences (Collect fails → no messages → section absent), and one-sentence docstrings of identical shape:

```go
// FetchBody fetches the complete RFC 822 body for uid and returns a reader.
func (r *realClient) FetchBody(uid mail.UID) (io.ReadCloser, error) {
    msgs, err := r.c.Fetch(…).Collect()
    if err != nil { return nil, fmt.Errorf("uid fetch body: %w", err) }
    if len(msgs) == 0 { return nil, fmt.Errorf("uid fetch body: no message for uid %s", uid) }
    raw := msgs[0].FindBodySection(…)
    if raw == nil { return nil, fmt.Errorf("uid fetch body: no BODY[] section for uid %s", uid) }

// FetchBodyStructure issues UID FETCH BODYSTRUCTURE for one UID …
func (r *realClient) FetchBodyStructure(uid mail.UID) (BodyStructure, error) {
    …
    if err != nil { return BodyStructure{}, fmt.Errorf("uid fetch bodystructure: %w", err) }
    if len(msgs) == 0 { return BodyStructure{}, fmt.Errorf("uid fetch bodystructure: no message for uid %s", uid) }
    if msgs[0].BodyStructure == nil { return BodyStructure{}, fmt.Errorf("uid fetch bodystructure: no BODYSTRUCTURE for uid %s", uid) }

// FetchBodyPart fetches the raw bytes of a single MIME part …
func (r *realClient) FetchBodyPart(uid mail.UID, section string) ([]byte, error) {
    …
    if err != nil { return nil, fmt.Errorf("fetch body part %q: %w", section, err) }
    if len(msgs) == 0 { return nil, fmt.Errorf("fetch body part %q: no message for uid %s", section, uid) }
    raw := msgs[0].FindBodySection(fetchSec)
    if raw == nil { return nil, fmt.Errorf("fetch body part %q: section not found for uid %s", section, uid) }
```

Three-guard body structure mechanically copied; three docstrings of identical one-sentence shape. (Three-guard pattern is partly go-imap-driven; the docstring rhythm is not.)

### imap.go:55–76 — C4 (T7)

```go
// AccountName satisfies mail.Backend.
func (b *Backend) AccountName() string { … }

// AccountEmail satisfies mail.Backend.
func (b *Backend) AccountEmail() string { … }

// Updates satisfies mail.Backend. Returns a nil channel before
// Connect succeeds.
func (b *Backend) Updates() <-chan mail.Update { return b.updates }
```

Trivial getters get same comment weight as substantive `Connect`. Metronomic density.

### client.go:21–65 — C4 (T7)

Every method in the `imapClient` interface (twelve methods) gets a 1–2 sentence comment in identical "X runs/issues/returns Y" shape. `UIDExpunge` (one line), `Store` (one line), and `FetchBodyStructure` (two lines) all receive same visual weight.

### idle_test.go:49–293, attachments_test.go:86–163 — C4 (T17 / T31)

Six tests in `idle_test.go` and four in `attachments_test.go` have docstrings in identical "TestX verifies/confirms/ensures that Y" sentence form:

```go
// TestIdleEmitsConnectedOnStart verifies that idleLoop emits
// ConnConnected after it selects the folder and enters IDLE.
// TestIdleFolderSwitch verifies that a folder-switch signal causes
// the idle goroutine to stop the current IDLE and re-IDLE on the new folder.
// TestIdlePollFallback verifies that when IDLE capability is absent
// the goroutine falls back to polling and emits UpdateFolderInfo.
```

Per-test docstrings restating the test name in sentence form.

---

## C6 — Test boilerplate

### idle_test.go:49,86,164,229,255,291 — C6 (T9)

"TestX verifies that Y" docstrings encode the assertion in sentence form. Test function names already carry the documentation.

### attachments_test.go:86,107,148,161 — C6 (T9)

```go
// TestAttachmentsSkipsBodyOnly confirms that when the top-level is a
// standalone text/plain (no multipart wrapper), it is also skipped.

// TestAttachmentsMissingUID confirms an error is returned for an unknown UID.
```

"confirms that" sentence-form assertion docstrings.

### messages_test.go and across all test files — C6 (T24)

```go
if total != 0 { t.Errorf("total = %d, want 0", total) }
…
if len(uids) != 0 { t.Errorf("uids = %v, want empty", uids) }
```

The `got = X, want Y` template appears ~35 times across six test files with no variation in phrasing.

---

## C7 — Error phrasing

### realclient.go:373–381 — C7 (T10b)

```go
return nil, fmt.Errorf("uid fetch body: %w", err)
return nil, fmt.Errorf("uid fetch body: no message for uid %s", uid)
return nil, fmt.Errorf("uid fetch body: no BODY[] section for uid %s", uid)
```

Three adjacent sites with identical prefix; the same pattern repeats verbatim in `FetchBodyStructure` and `FetchBodyPart`. Cross-function error chorus.

### actions.go:37,40 / 101,104 — C7 (T11)

```go
if err := cmd.Store(uids, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
    return fmt.Errorf("store deleted: %w", classifyErr(err))
}
if err := cmd.UIDExpunge(uids); err != nil {
    return fmt.Errorf("uid expunge: %w", classifyErr(err))
}
```

The `"store deleted" / "uid expunge"` pair appears identically in MOVE fallback (37/40) and in `Destroy` (101/104) — copy-pasted between two functions.

### auth.go:158–160 — C7 (T10)

```go
return "", fmt.Errorf("password-cmd failed: %s", stderr)
…
return "", fmt.Errorf("password-cmd failed: %w", err)
```

"failed:" prefix anti-pattern; both adjacent sites use it.

### realclient.go:396, auth.go:117, messages.go:95 — C7 (T13)

```go
return fmt.Errorf("store: %w", err)
return fmt.Errorf("authenticate (%s): %w", role, err)
return nil, fmt.Errorf("fetch headers: %w", classifyErr(err))
```

`%w` reflexively used across internal adapter methods. `classifyErr` already promotes sentinels; the second `%w` adds unwrap surface with no caller benefit (no `errors.Is`/`errors.As` on these results).

---

## C8 — Structural symmetry

### internal/mailimap/ — C8 (T19)

Twelve files split by role: `imap.go`, `auth.go`, `client.go`, `realclient.go`, `messages.go`, `folders.go`, `actions.go`, `attachments.go`, `changes.go`, `idle.go`, `errors.go`, `tlsHint.go`. `actions.go` groups Move/Destroy/Flag/Send purely because they are "write" operations. `changes.go` (77 lines, one method + two helpers) could live in `messages.go` without loss. `errors.go` (44 lines) and `tlsHint.go` (26 lines) are skeleton files. The split reflexively mirrors `mailjmap`'s layout rather than emerging from this package's actual coupling.

---

## Calibration notes

- **C2 absent:** `len(uids) == 0` and `ch == nil` guards are at exported-API boundaries — correct.
- **C3 absent:** `imapClient` has two concrete impls (`realClient`, `fakeClient`) — explicit DI seam.
- **C5 absent:** no `Get`-prefix, no package-doubled names, no docstring-length exports.
