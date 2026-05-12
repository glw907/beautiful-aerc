package mailimap

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glw907/poplar/internal/mail"
)

// fakeClient is an in-memory imapClient for unit tests. Tests
// populate the maps directly. Methods return canned data or run
// caller-supplied funcs.
type fakeClient struct {
	caps          map[string]bool
	folders       []listEntry
	folderSummary map[string]mail.Folder
	selected      string

	bodies        map[mail.UID]string
	bodyStructure map[mail.UID]BodyStructure
	bodyParts     map[string][]byte // key "<uid>::<section>"

	storeCalls   [][3]any // {uids, item, value}
	moveCalls    [][2]any // {uids, dest}
	copyCalls    [][2]any
	expungeCalls [][]mail.UID
	appendCalls  []appendCall
	// nextUID is the APPENDUID returned by the next Append call. Zero
	// disables: Append returns "" with no error.
	nextUID uint32

	onIdle   func(emit func(mail.Update)) error
	idleStop func()

	logoutErr  error
	copyErr    error
	moveErr    error
	expungeErr error
	appendErr  error

	// searchFn, when non-nil, overrides the default Search stub.
	searchFn func(mail.SearchCriteria) ([]mail.UID, error)
	// fetchFn, when non-nil, overrides the default Fetch stub.
	fetchFn func(uids []mail.UID, items []string, resultFn func(mail.UID, map[string]any)) error
	// storeFn, when non-nil, overrides the default Store stub. Lets
	// tests inject errors for the redial path.
	storeFn func(uids []mail.UID, item string, value any) error
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		caps:          map[string]bool{},
		folderSummary: map[string]mail.Folder{},
		bodies:        map[mail.UID]string{},
		bodyStructure: map[mail.UID]BodyStructure{},
		bodyParts:     map[string][]byte{},
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

func (f *fakeClient) Search(c mail.SearchCriteria) ([]mail.UID, error) {
	if f.searchFn != nil {
		return f.searchFn(c)
	}
	return nil, nil
}

func (f *fakeClient) Fetch(uids []mail.UID, items []string, resultFn func(mail.UID, map[string]any)) error {
	if f.fetchFn != nil {
		return f.fetchFn(uids, items, resultFn)
	}
	return nil
}

func (f *fakeClient) FetchBody(uid mail.UID) (io.ReadCloser, error) {
	body, ok := f.bodies[uid]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(strings.NewReader(body)), nil
}

func (f *fakeClient) FetchBodyStructure(uid mail.UID) (BodyStructure, error) {
	bs, ok := f.bodyStructure[uid]
	if !ok {
		return BodyStructure{}, errors.New("not found")
	}
	return bs, nil
}

func (f *fakeClient) FetchBodyPart(uid mail.UID, section string) ([]byte, error) {
	key := string(uid) + "::" + section
	data, ok := f.bodyParts[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (f *fakeClient) Store(uids []mail.UID, item string, value any) error {
	f.storeCalls = append(f.storeCalls, [3]any{uids, item, value})
	if f.storeFn != nil {
		return f.storeFn(uids, item, value)
	}
	return nil
}

func (f *fakeClient) Copy(uids []mail.UID, dest string) error {
	f.copyCalls = append(f.copyCalls, [2]any{uids, dest})
	return f.copyErr
}

func (f *fakeClient) Move(uids []mail.UID, dest string) error {
	f.moveCalls = append(f.moveCalls, [2]any{uids, dest})
	return f.moveErr
}

func (f *fakeClient) UIDExpunge(uids []mail.UID) error {
	f.expungeCalls = append(f.expungeCalls, uids)
	return f.expungeErr
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

type appendCall struct {
	folder string
	mime   []byte
	flags  []string
}

func (f *fakeClient) Append(folder string, mime []byte, flags []string) (mail.UID, error) {
	f.appendCalls = append(f.appendCalls, appendCall{folder: folder, mime: append([]byte(nil), mime...), flags: append([]string(nil), flags...)})
	if f.appendErr != nil {
		return "", f.appendErr
	}
	if f.nextUID == 0 {
		return "", nil
	}
	uid := f.nextUID
	f.nextUID++
	return mail.UID(strconv.FormatUint(uint64(uid), 10)), nil
}
