// SPDX-License-Identifier: MIT

package mailimap

import (
	"io"

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
}

// listEntry is the result of a LIST command for one folder.
type listEntry struct {
	Name        string
	Attributes  []string // includes \Drafts, \Sent, \Trash, etc. when SPECIAL-USE
	HasChildren bool
}
