package mail

import (
	"context"
	"errors"
	"time"
)

// ErrAuth is the sentinel for backend authentication failure (HTTP
// 4xx from JMAP, IMAP BAD/NO with auth context, expired OAuth token).
// The cache drainer matches via errors.Is and promotes the outbox row
// to a conflict with kind "auth-failure".
var ErrAuth = errors.New("mail: authentication failed")

// ErrNotFound signals the message no longer exists on the server. The
// drainer treats it as idempotent success.
var ErrNotFound = errors.New("mail: not found")

// ErrUnsupported signals the backend does not implement the operation.
// Callers gate on capability (e.g. IsJMAP) before queuing. Not routed
// through the drainer conflict matrix.
var ErrUnsupported = errors.New("mail: operation unsupported by backend")

// SearchCriteria defines message search parameters.
type SearchCriteria struct {
	Header map[string][]string
	Body   []string
	Text   []string
}

// Envelope is the RFC 5321 envelope passed to Backend.Send. From is
// the bounce address (MAIL FROM). Rcpts is the recipient list (RCPT
// TO) and includes Bcc addresses that the MIME body omits.
type Envelope struct {
	From  string
	Rcpts []string
}

// Backend is the interface that mail protocol adapters implement.
// Every method blocks until the operation completes.
type Backend interface {
	AccountName() string
	// AccountEmail is empty before Connect for backends that auto-
	// discover the address (e.g. JMAP session).
	AccountEmail() string

	Connect(ctx context.Context) error
	Disconnect() error

	ListFolders() ([]Folder, error)
	OpenFolder(name string) error

	// QueryFolder returns up to limit UIDs from name starting at offset
	// (newest-first), plus the total count so the UI can show "N of M"
	// and stop dispatching load-more.
	QueryFolder(name string, offset, limit int) (uids []UID, total int, err error)

	FetchHeaders(uids []UID) ([]MessageInfo, error)
	FetchBody(uid UID) ([]byte, error)

	Attachments(uid UID) ([]Attachment, error)
	// FetchAttachment returns decoded bytes for partID on uid. partID
	// must come from a prior Attachments call on the same Backend.
	FetchAttachment(uid UID, partID string) ([]byte, error)

	Move(uids []UID, dest string) error
	// Destroy permanently deletes uids from the selected folder,
	// bypassing Trash. Irreversible. Empty input is a no-op.
	Destroy(uids []UID) error
	Flag(uids []UID, flag Flag, set bool) error

	// Send transmits mime to env's recipients. JMAP collapses send
	// and Sent-copy atomically. IMAP+SMTP only transmits, and the
	// caller issues a separate Append for the Sent copy.
	Send(env Envelope, mime []byte) error

	// Append writes mime to folder with the given flags. The cache
	// outbox uses it for the Sent copy on IMAP and for manual drafts.
	// Callers set \Seen on Sent copies.
	Append(folder string, mime []byte, flags Flag) error

	// PushDraft writes mime to folder as a draft and, when prevUID is
	// non-empty, destroys the prior server image in the same operation.
	// Returns the new server UID. JMAP batches import + destroy in one
	// request. IMAP is not atomic and may orphan the prior image.
	// Backends without server-side draft persistence return
	// ErrUnsupported.
	PushDraft(folder string, mime []byte, prevUID UID) (UID, error)

	IsJMAP() bool

	Updates() <-chan Update
}

// MessageInfo holds message header information for list display.
//
// ThreadID groups messages in a conversation. A non-threaded message
// is a thread of size 1 with ThreadID == UID and InReplyTo == "".
// InReplyTo points at the parent UID and is empty for thread roots.
// Depth is not carried on the wire. The UI walks the tree to derive
// depth and box-drawing prefixes.
type MessageInfo struct {
	UID     UID
	Subject string
	From    string
	// To, Cc, Bcc are flat display strings ("Name1, Name2, ...") shaped
	// like From. The viewer renders each as a header row when non-empty.
	To     string
	Cc     string
	Bcc    string
	SentAt time.Time
	Flags  Flag
	Size   uint32

	ThreadID  UID
	InReplyTo UID
}
