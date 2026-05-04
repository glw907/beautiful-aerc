// SPDX-License-Identifier: MIT

package mailimap

import (
	"io"
	"strconv"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// imapClient is the subset of go-imap's client surface that mailimap
// uses. The real *imapclient.Client satisfies it (via a thin adapter
// in auth.go); tests substitute a fake.
//
// Method signatures will be fleshed out as each task lands. Each
// method should return errors with the wrapped IMAP server response
// when applicable so the error banner can surface useful detail.
type imapClient interface {
	// Authenticate runs SASL with the given mechanism name + client.
	// Logout closes the connection cleanly.
	Logout() error

	// Capabilities returns the advertised capability set as a map.
	Capabilities() (map[string]bool, error)

	// List runs LIST/LSUB and returns folders. specialUse causes
	// the LIST RETURN (SPECIAL-USE) variant when supported.
	List(ref, pattern string, specialUse bool) ([]listEntry, error)

	// Select selects a folder and returns its summary.
	Select(folder string, readOnly bool) (mail.Folder, error)

	// Search runs UID SEARCH with the criteria and returns matching UIDs.
	Search(criteria mail.SearchCriteria) ([]mail.UID, error)

	// Fetch runs UID FETCH; resultFn is called once per message.
	Fetch(uids []mail.UID, items []string, resultFn func(uid mail.UID, items map[string]any)) error

	// FetchBody returns a reader for the full RFC 822 body of one UID.
	FetchBody(uid mail.UID) (io.ReadCloser, error)

	// Store runs UID STORE.
	Store(uids []mail.UID, item string, value any) error

	// Copy and Move are UID COPY / UID MOVE.
	Copy(uids []mail.UID, dest string) error
	Move(uids []mail.UID, dest string) error

	// Expunge runs plain EXPUNGE; UIDExpunge runs UID EXPUNGE (UIDPLUS).
	UIDExpunge(uids []mail.UID) error

	// Idle blocks until the server tears down or DONE is sent;
	// onUpdate is called per unilateral response.
	Idle(onUpdate func(mail.Update)) error
	IdleStop() // sends DONE

	// FetchBodyStructure issues UID FETCH (BODYSTRUCTURE) for one UID
	// and returns the parsed structure. Caller walks the tree.
	FetchBodyStructure(uid mail.UID) (BodyStructure, error)

	// FetchBodyPart returns the decoded bytes of one MIME part.
	// section is an IMAP section identifier ("2", "2.1", etc.).
	FetchBodyPart(uid mail.UID, section string) ([]byte, error)
}

// listEntry is the result of a LIST command for one folder.
type listEntry struct {
	Name       string
	Attributes []string // includes \Drafts, \Sent, \Trash, etc. when SPECIAL-USE
}

// BodyStructure is the protocol-agnostic shape of a parsed IMAP
// BODYSTRUCTURE response. Only the fields mailimap needs are
// retained; the underlying go-imap type carries more.
type BodyStructure struct {
	Section     string          // "" for root, "1", "2", "2.1" for parts
	MIMEType    string          // "text/plain" lowercased
	Filename    string
	SizeBytes   uint32
	ContentID   string
	Disposition string          // "attachment" | "inline" | "" if unset
	Children    []BodyStructure
}

// sectionString converts a []int path (as produced by go-imap Walk) to
// the dot-joined section string used in BodyStructure.Section.
// An empty path (multipart root) produces "".
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
