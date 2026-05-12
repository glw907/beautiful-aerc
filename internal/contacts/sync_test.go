package contacts

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
)

type fakeStore struct {
	books map[string]BookState
	apply []applyCall
}

type applyCall struct {
	bookHref string
	added    []Stored
	removed  []string
	token    string
	ctag     string
}

func (f *fakeStore) Books(_ context.Context) (map[string]BookState, error) {
	if f.books == nil {
		return map[string]BookState{}, nil
	}
	return f.books, nil
}

func (f *fakeStore) UpsertBook(_ context.Context, b BookState) error {
	if f.books == nil {
		f.books = map[string]BookState{}
	}
	f.books[b.Href] = b
	return nil
}

func (f *fakeStore) ApplyChangeset(_ context.Context, bookHref string, added []Stored, removed []string, token, ctag string) error {
	f.apply = append(f.apply, applyCall{bookHref, added, removed, token, ctag})
	return nil
}

var errReadOnly = webdav.NewHTTPError(http.StatusForbidden, fmt.Errorf("read-only"))

// syncTestBackend implements carddav.Backend just enough to drive the
// poplar Sync engine through its branches. Tests mutate the per-test
// fields between rounds to control responses.
type syncTestBackend struct {
	principalPath string
	homeSetPath   string
	addressBooks  []carddav.AddressBook
	parsed        []carddav.AddressObject
}

func (b *syncTestBackend) CurrentUserPrincipal(_ context.Context) (string, error) {
	return b.principalPath, nil
}
func (b *syncTestBackend) AddressBookHomeSetPath(_ context.Context) (string, error) {
	return b.homeSetPath, nil
}
func (b *syncTestBackend) ListAddressBooks(_ context.Context) ([]carddav.AddressBook, error) {
	return b.addressBooks, nil
}
func (b *syncTestBackend) GetAddressBook(_ context.Context, path string) (*carddav.AddressBook, error) {
	for i := range b.addressBooks {
		if b.addressBooks[i].Path == path {
			return &b.addressBooks[i], nil
		}
	}
	return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
}
func (b *syncTestBackend) CreateAddressBook(_ context.Context, _ *carddav.AddressBook) error {
	return errReadOnly
}
func (b *syncTestBackend) DeleteAddressBook(_ context.Context, _ string) error {
	return errReadOnly
}
func (b *syncTestBackend) GetAddressObject(_ context.Context, path string, _ *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	for i := range b.parsed {
		if b.parsed[i].Path == path {
			return &b.parsed[i], nil
		}
	}
	return nil, webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
}
func (b *syncTestBackend) ListAddressObjects(_ context.Context, _ string, _ *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	return b.parsed, nil
}
func (b *syncTestBackend) QueryAddressObjects(_ context.Context, _ string, _ *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	return b.parsed, nil
}
func (b *syncTestBackend) PutAddressObject(_ context.Context, _ string, _ vcard.Card, _ *carddav.PutAddressObjectOptions) (*carddav.AddressObject, error) {
	return nil, errReadOnly
}
func (b *syncTestBackend) DeleteAddressObject(_ context.Context, _ string) error {
	return errReadOnly
}

// newSyncFixture stands up a fake CardDAV server plus a poplar Client
// pointing at it. Tests provide the vCard objects; the backend's
// principal/home-set/book paths are fixed.
func newSyncFixture(t *testing.T, objects map[string]string) (*Client, *syncTestBackend) {
	t.Helper()
	parsed := make([]carddav.AddressObject, 0, len(objects))
	for path, src := range objects {
		card, err := vcard.NewDecoder(strings.NewReader(src)).Decode()
		if err != nil {
			t.Fatalf("parse vcard %s: %v", path, err)
		}
		parsed = append(parsed, carddav.AddressObject{Path: path, Card: card, ETag: `"e-` + path + `"`})
	}
	bk := &syncTestBackend{
		principalPath: "/u/",
		homeSetPath:   "/u/contacts/",
		addressBooks: []carddav.AddressBook{
			{Path: "/u/contacts/default/", Name: "Default"},
		},
		parsed: parsed,
	}
	h := &carddav.Handler{Backend: bk}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(srv.URL+"/.well-known/carddav", "u", "p", false)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, bk
}

const aliceVCard = `BEGIN:VCARD
VERSION:4.0
UID:alice
FN:Alice Adams
N:Adams;Alice;;;
EMAIL:alice@example.com
END:VCARD
`

const bobVCard = `BEGIN:VCARD
VERSION:4.0
UID:bob
FN:Bob Beam
N:Beam;Bob;;;
EMAIL:bob@example.com
END:VCARD
`

const teamGroupVCard = `BEGIN:VCARD
VERSION:4.0
UID:team
KIND:group
FN:Team
END:VCARD
`

func TestSync_FirstRun_FullPull(t *testing.T) {
	c, _ := newSyncFixture(t, map[string]string{
		"/u/contacts/default/alice.vcf": aliceVCard,
		"/u/contacts/default/bob.vcf":   bobVCard,
	})
	s := &fakeStore{}

	if err := Sync(context.Background(), c, s); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(s.apply) != 1 {
		t.Fatalf("ApplyChangeset calls = %d, want 1", len(s.apply))
	}
	if got := len(s.apply[0].added); got != 2 {
		t.Errorf("added = %d, want 2", got)
	}
	if s.apply[0].bookHref != "/u/contacts/default/" {
		t.Errorf("bookHref = %q", s.apply[0].bookHref)
	}
	book, ok := s.books["/u/contacts/default/"]
	if !ok {
		t.Fatalf("UpsertBook never called for default book; got %v", s.books)
	}
	if book.LastSyncedAt.IsZero() {
		t.Error("LastSyncedAt was not set")
	}
}

func TestSync_GroupVCard_Skipped(t *testing.T) {
	c, _ := newSyncFixture(t, map[string]string{
		"/u/contacts/default/alice.vcf": aliceVCard,
		"/u/contacts/default/team.vcf":  teamGroupVCard,
	})
	s := &fakeStore{}

	if err := Sync(context.Background(), c, s); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(s.apply) != 1 {
		t.Fatalf("ApplyChangeset calls = %d, want 1", len(s.apply))
	}
	if got := len(s.apply[0].added); got != 1 {
		t.Errorf("added = %d, want 1 (group vCard skipped)", got)
	}
	if got := s.apply[0].added[0].UID; got != "alice" {
		t.Errorf("kept contact UID = %q, want alice", got)
	}
}

// The carddav.Handler does not emit the getctag property, so the
// CTAG short-circuit path is untestable here. This locks in the
// fallback shape: every Sync runs a full pull.
func TestSync_RepeatedSyncWithoutCTAG_FullPullEachTime(t *testing.T) {
	c, _ := newSyncFixture(t, map[string]string{
		"/u/contacts/default/alice.vcf": aliceVCard,
	})
	s := &fakeStore{}

	if err := Sync(context.Background(), c, s); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if err := Sync(context.Background(), c, s); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if len(s.apply) != 2 {
		t.Errorf("ApplyChangeset calls = %d, want 2 (no CTAG support → full pull every time)", len(s.apply))
	}
}

// The fake server omits REPORT sync-collection, so the SupportsSync
// probe silently fails and SupportsSync stays false.
func TestSync_SyncCollection_Unsupported(t *testing.T) {
	c, _ := newSyncFixture(t, map[string]string{
		"/u/contacts/default/alice.vcf": aliceVCard,
	})
	s := &fakeStore{}

	if err := Sync(context.Background(), c, s); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	book := s.books["/u/contacts/default/"]
	if book.SupportsSync {
		t.Error("SupportsSync = true, want false (server omits sync-collection)")
	}
	if book.SyncToken != "" {
		t.Errorf("SyncToken = %q, want empty", book.SyncToken)
	}
}
