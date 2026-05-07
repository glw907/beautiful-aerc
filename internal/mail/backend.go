package mail

import (
	"context"
	"errors"
	"time"
)

// ErrAuth is the sentinel for backend authentication failure (4xx
// from JMAP/HTTP, BAD/NO with auth context from IMAP, or expired
// OAuth token). The cache drainer matches via errors.Is and promotes
// the outbox row to conflict with kind "auth-failure". Backends MUST
// wrap auth failures with %w so the chain unwraps to ErrAuth.
var ErrAuth = errors.New("mail: authentication failed")

// ErrNotFound signals the message no longer exists on the server.
// The cache drainer treats it as idempotent success per spec §D.4.
var ErrNotFound = errors.New("mail: not found")

// ErrUnsupported signals that the backend does not implement the
// operation. Callers must gate on capability (e.g. IsJMAP) before
// queuing. Not routed through the drainer conflict matrix.
var ErrUnsupported = errors.New("mail: operation unsupported by backend")

// SearchCriteria defines message search parameters.
type SearchCriteria struct {
	Header map[string][]string
	Body   []string
	Text   []string
}

// Envelope is the SMTP-style envelope passed to Backend.Send. From
// is the bounce address (RFC 5321 MAIL FROM). Rcpts is the recipient
// list (RFC 5321 RCPT TO) and includes Bcc addresses that the MIME
// body omits.
type Envelope struct {
	From  string
	Rcpts []string
}

// Backend is the interface that mail protocol adapters implement.
// Every method blocks until the operation completes.
type Backend interface {
	// AccountName is the user-facing display label.
	AccountName() string
	// AccountEmail is the user's email address. Empty before Connect
	// resolves it for backends that auto-discover (e.g. JMAP session).
	AccountEmail() string

	Connect(ctx context.Context) error
	Disconnect() error

	ListFolders() ([]Folder, error)
	OpenFolder(name string) error

	// QueryFolder returns up to limit message UIDs from name starting
	// at offset (newest-first), plus the total message count. The
	// total enables the UI to show "showing N of M" and to stop
	// dispatching load-more once exhausted.
	QueryFolder(name string, offset, limit int) (uids []UID, total int, err error)

	FetchHeaders(uids []UID) ([]MessageInfo, error)
	FetchBody(uid UID) ([]byte, error)

	// Attachments returns metadata for non-body parts of uid.
	Attachments(uid UID) ([]Attachment, error)

	// FetchAttachment returns decoded bytes for partID on uid.
	// partID must come from a prior Attachments call on the same
	// backend instance.
	FetchAttachment(uid UID, partID string) ([]byte, error)

	Move(uids []UID, dest string) error
	// Destroy permanently deletes uids from the currently-selected
	// folder, bypassing Trash. Irreversible. Empty input is a no-op.
	Destroy(uids []UID) error
	Flag(uids []UID, flag Flag, set bool) error

	// Send transmits mime to the recipients in env. Backends that
	// collapse send + Sent-copy into one operation (JMAP) do so
	// atomically. IMAP+SMTP only transmits. The caller issues a
	// separate Append for the Sent copy.
	Send(env Envelope, mime []byte) error

	// Append writes mime to folder with the given flags. Used by
	// the cache outbox to deposit the Sent copy on IMAP, and to
	// save manual drafts. Empty flags is allowed. The caller sets
	// \Seen on Sent copies.
	Append(folder string, mime []byte, flags Flag) error

	// PushDraft writes mime to folder as a draft, destroying the prior
	// server image identified by prevUID in the same operation when
	// prevUID is non-empty. Returns the new server UID. JMAP batches
	// import + destroy into one request. IMAP is not atomic and may
	// orphan the prior image. Returns ErrUnsupported for backends that
	// do not implement server-side draft persistence.
	PushDraft(folder string, mime []byte, prevUID UID) (UID, error)

	// IsJMAP reports whether the backend uses JMAP rather than IMAP
	// submission.
	IsJMAP() bool

	Updates() <-chan Update
}

// MessageInfo holds message header information for list display.
//
// ThreadID groups messages that belong to the same conversation. A
// non-threaded message is a thread of size 1 with ThreadID == UID and
// InReplyTo == "". InReplyTo points at the parent message's UID and
// is empty for thread roots. The UI layer derives depth and box-
// drawing prefixes from the tree shape. Depth is not carried on the
// wire. Doing so would duplicate what the prefix walk already produces
// and risk drift if a backend miscounted.
type MessageInfo struct {
	UID     UID
	Subject string
	From    string
	// To, Cc, Bcc are flat display strings ("Name1, Name2, ...") in
	// the same shape as From. The viewer renders each as a single
	// header row when non-empty.
	To  string
	Cc  string
	Bcc string
	// Date is the pre-rendered display string the UI shows verbatim.
	// SentAt is the authoritative instant for sorting. Workers fill
	// both, and UI sort comparisons use SentAt (falling back to Date
	// lex when SentAt is zero, for legacy fixtures).
	Date   string
	SentAt time.Time
	Flags  Flag
	Size   uint32

	ThreadID  UID
	InReplyTo UID
}
