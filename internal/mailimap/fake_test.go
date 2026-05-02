// SPDX-License-Identifier: MIT

package mailimap

import (
	"errors"
	"io"

	"github.com/glw907/poplar/internal/mail"
)

// fakeClient is an in-memory imapClient for unit tests. Tests
// populate the maps directly; methods return canned data or run
// caller-supplied funcs.
type fakeClient struct {
	caps          map[string]bool
	folders       []listEntry
	folderSummary map[string]mail.Folder
	selected      string

	bodies map[mail.UID]string

	storeCalls   [][3]any // {uids, item, value}
	moveCalls    [][2]any // {uids, dest}
	copyCalls    [][2]any
	expungeCalls [][]mail.UID

	onIdle   func(emit func(mail.Update)) error
	idleStop func()

	logoutErr error
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		caps:          map[string]bool{},
		folderSummary: map[string]mail.Folder{},
		bodies:        map[mail.UID]string{},
	}
}

func (f *fakeClient) Logout() error { return f.logoutErr }

func (f *fakeClient) Capabilities() (map[string]bool, error) { return f.caps, nil }

func (f *fakeClient) List(ref, pattern string, specialUse bool) ([]listEntry, error) {
	return f.folders, nil
}

func (f *fakeClient) Select(folder string, readOnly bool) (mail.Folder, error) {
	f.selected = folder
	if s, ok := f.folderSummary[folder]; ok {
		return s, nil
	}
	return mail.Folder{Name: folder}, nil
}

func (f *fakeClient) Search(c mail.SearchCriteria) ([]mail.UID, error) { return nil, nil }

func (f *fakeClient) Fetch(uids []mail.UID, items []string, resultFn func(mail.UID, map[string]any)) error {
	return nil
}

func (f *fakeClient) FetchBody(uid mail.UID) (io.ReadCloser, error) {
	body, ok := f.bodies[uid]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(stringReader(body)), nil
}

func (f *fakeClient) Store(uids []mail.UID, item string, value any) error {
	f.storeCalls = append(f.storeCalls, [3]any{uids, item, value})
	return nil
}

func (f *fakeClient) Copy(uids []mail.UID, dest string) error {
	f.copyCalls = append(f.copyCalls, [2]any{uids, dest})
	return nil
}

func (f *fakeClient) Move(uids []mail.UID, dest string) error {
	f.moveCalls = append(f.moveCalls, [2]any{uids, dest})
	return nil
}

func (f *fakeClient) UIDExpunge(uids []mail.UID) error {
	f.expungeCalls = append(f.expungeCalls, uids)
	return nil
}

func (f *fakeClient) Idle(onUpdate func(mail.Update)) error {
	if f.onIdle != nil {
		return f.onIdle(onUpdate)
	}
	return nil
}

func (f *fakeClient) IdleStop() {
	if f.idleStop != nil {
		f.idleStop()
	}
}

type stringReader string

func (s stringReader) Read(p []byte) (int, error) {
	if len(s) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s)
	return n, nil
}
