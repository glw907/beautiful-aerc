package mailimap

import (
	"io"
	"strconv"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// imapClient is the subset of go-imap's client surface mailimap uses.
// *imapclient.Client satisfies it via a thin adapter. Tests substitute
// a fake.
type imapClient interface {
	Logout() error
	Capabilities() (map[string]bool, error)
	// List runs LIST. specialUse turns on LIST RETURN (SPECIAL-USE)
	// when supported.
	List(ref, pattern string, specialUse bool) ([]listEntry, error)
	Select(folder string, readOnly bool) (mail.Folder, error)
	Search(criteria mail.SearchCriteria) ([]mail.UID, error)
	// Fetch runs UID FETCH. resultFn fires once per message.
	Fetch(uids []mail.UID, items []string, resultFn func(uid mail.UID, items map[string]any)) error
	FetchBody(uid mail.UID) (io.ReadCloser, error)
	Store(uids []mail.UID, item string, value any) error
	Copy(uids []mail.UID, dest string) error
	Move(uids []mail.UID, dest string) error
	UIDExpunge(uids []mail.UID) error
	// Idle blocks until DONE or server tear-down. onUpdate fires per
	// unilateral response.
	Idle(onUpdate func(mail.Update)) error
	IdleStop()
	FetchBodyStructure(uid mail.UID) (BodyStructure, error)
	FetchBodyPart(uid mail.UID, section string) ([]byte, error)
	// Append writes mime via APPEND. UIDPLUS guarantees an APPENDUID,
	// so the returned UID is always set.
	Append(folder string, mime []byte, flags []string) (mail.UID, error)
}

type listEntry struct {
	Name       string
	Attributes []string // includes \Drafts, \Sent, \Trash, etc. when SPECIAL-USE
}

// BodyStructure is the protocol-agnostic projection of an IMAP
// BODYSTRUCTURE response, narrowed to the fields mailimap needs.
type BodyStructure struct {
	Section     string // "" for root, "1", "2", "2.1" for parts
	MIMEType    string // lowercased
	Filename    string
	SizeBytes   uint32
	ContentID   string
	Disposition string // "attachment", "inline", or "" if unset
	Children    []BodyStructure
}

// sectionString joins a go-imap Walk path into the dot-joined section
// string used in BodyStructure.Section. Empty path is multipart root.
func sectionString(path []int) string {
	if len(path) == 0 {
		return ""
	}
	parts := make([]string, len(path))
	for i, n := range path {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}
