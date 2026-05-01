// SPDX-License-Identifier: MIT

package mail

import (
	"context"
	"io"
	"time"
)

// SearchCriteria defines message search parameters.
type SearchCriteria struct {
	Header map[string][]string
	Body   []string
	Text   []string
}

// Backend is the interface that mail protocol adapters implement.
// Each method blocks until the operation completes. Bubbletea's
// tea.Cmd model handles async naturally by running blocking calls
// in commands that return messages on completion.
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
	FetchBody(uid UID) (io.Reader, error)

	Search(criteria SearchCriteria) ([]UID, error)

	Move(uids []UID, dest string) error
	Copy(uids []UID, dest string) error
	Delete(uids []UID) error
	Flag(uids []UID, flag Flag, set bool) error
	MarkRead(uids []UID) error
	MarkUnread(uids []UID) error
	MarkAnswered(uids []UID) error

	Send(from string, rcpts []string, body io.Reader) error

	Updates() <-chan Update
}

// MessageInfo holds message header information for list display.
//
// ThreadID groups messages that belong to the same conversation. A
// non-threaded message is a thread of size 1 with ThreadID == UID and
// InReplyTo == "". InReplyTo points at the parent message's UID and
// is empty for thread roots. The UI layer derives depth and box-
// drawing prefixes from the tree shape — depth is not carried on the
// wire because doing so would duplicate information the prefix walk
// already produces and risk drift if a backend miscounted.
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
	// SentAt is the authoritative instant for sorting; workers fill
	// both, and UI sort comparisons use SentAt (falling back to Date
	// lex when SentAt is zero, for legacy fixtures).
	Date   string
	SentAt time.Time
	Flags  Flag
	Size   uint32

	ThreadID  UID
	InReplyTo UID
}
