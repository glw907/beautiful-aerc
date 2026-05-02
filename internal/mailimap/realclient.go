// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"
	"io"

	imapclient "github.com/emersion/go-imap/v2/imapclient"

	"github.com/glw907/poplar/internal/mail"
)

// realClient adapts *imapclient.Client to the imapClient interface.
// Method bodies that interact with the server are filled in alongside
// the tasks that use them (Tasks 9–16). Until then each unimplemented
// method returns an explicit stub error so the build stays green and
// test failures are easy to trace.
type realClient struct {
	c *imapclient.Client
}

// newRealClient wraps c in a realClient.
func newRealClient(c *imapclient.Client) *realClient {
	return &realClient{c: c}
}

// Logout sends LOGOUT and waits for the server acknowledgement.
func (r *realClient) Logout() error {
	return r.c.Logout().Wait()
}

// Capabilities issues a CAPABILITY command and converts the go-imap v2
// CapSet (map[imap.Cap]struct{}) to the map[string]bool form the
// imapClient interface requires.
func (r *realClient) Capabilities() (map[string]bool, error) {
	caps, err := r.c.Capability().Wait()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(caps))
	for cap := range caps {
		out[string(cap)] = true
	}
	return out, nil
}

// List — filled in by Task 9 (ListFolders + OpenFolder).
func (r *realClient) List(_, _ string, _ bool) ([]listEntry, error) {
	return nil, errors.New("realClient.List: not yet implemented (Task 9)")
}

// Select — filled in by Task 9 (ListFolders + OpenFolder).
func (r *realClient) Select(_ string, _ bool) (mail.Folder, error) {
	return mail.Folder{}, errors.New("realClient.Select: not yet implemented (Task 9)")
}

// Search — filled in by Task 10 (QueryFolder, FetchHeaders, Search).
func (r *realClient) Search(_ mail.SearchCriteria) ([]mail.UID, error) {
	return nil, errors.New("realClient.Search: not yet implemented (Task 10)")
}

// Fetch — filled in by Task 10 (QueryFolder, FetchHeaders).
func (r *realClient) Fetch(_ []mail.UID, _ []string, _ func(mail.UID, map[string]any)) error {
	return errors.New("realClient.Fetch: not yet implemented (Task 10)")
}

// FetchBody — filled in by Task 10 (FetchBody).
func (r *realClient) FetchBody(_ mail.UID) (io.ReadCloser, error) {
	return nil, errors.New("realClient.FetchBody: not yet implemented (Task 10)")
}

// Store — filled in by Task 11 (flag/mark methods).
func (r *realClient) Store(_ []mail.UID, _ string, _ any) error {
	return errors.New("realClient.Store: not yet implemented (Task 11)")
}

// Copy — filled in by Task 11 (Move/Copy/Delete).
func (r *realClient) Copy(_ []mail.UID, _ string) error {
	return errors.New("realClient.Copy: not yet implemented (Task 11)")
}

// Move — filled in by Task 11 (Move/Copy/Delete).
func (r *realClient) Move(_ []mail.UID, _ string) error {
	return errors.New("realClient.Move: not yet implemented (Task 11)")
}

// UIDExpunge — filled in by Task 11 (Move/Copy/Delete).
func (r *realClient) UIDExpunge(_ []mail.UID) error {
	return errors.New("realClient.UIDExpunge: not yet implemented (Task 11)")
}

// Idle — filled in by Task 12 (Idle goroutine).
func (r *realClient) Idle(_ func(mail.Update)) error {
	return errors.New("realClient.Idle: not yet implemented (Task 12)")
}

// IdleStop — filled in by Task 12 (Idle goroutine).
func (r *realClient) IdleStop() {}
