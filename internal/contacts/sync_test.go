package contacts

import (
	"context"
	"testing"
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

func (f *fakeStore) Books(ctx context.Context) (map[string]BookState, error) {
	if f.books == nil {
		return map[string]BookState{}, nil
	}
	return f.books, nil
}

func (f *fakeStore) UpsertBook(ctx context.Context, b BookState) error {
	if f.books == nil {
		f.books = map[string]BookState{}
	}
	f.books[b.Href] = b
	return nil
}

func (f *fakeStore) ApplyChangeset(ctx context.Context, bookHref string, added []Stored, removed []string, token, ctag string) error {
	f.apply = append(f.apply, applyCall{bookHref, added, removed, token, ctag})
	return nil
}

func TestSync_FirstRun_FullPull_ProbesSyncCollection(t *testing.T) {
	// Stand up an httptest CardDAV server returning:
	//   - PROPFIND principal: stub principal
	//   - PROPFIND home-set: one book at /books/default/
	//   - PROPFIND book: two resources
	//   - REPORT addressbook-multiget: two vCards
	//   - REPORT sync-collection (probe): empty token, success
	// Verify ApplyChangeset called once with 2 adds, BookState
	// stored with SupportsSync=true.
	t.Skip("stand up httptest server; implement after Task 4 lands")
}

func TestSync_Incremental_TokenRejection_FallsBackToFull(t *testing.T) {
	// Server returns 412 Precondition Failed on sync-collection.
	// Engine should fall through to full pull and clear token.
	t.Skip("stand up httptest server")
}

func TestSync_CTAG_Unchanged_NoApply(t *testing.T) {
	// Cached state: CTAG="abc", SupportsSync=false.
	// Server PROPFIND returns CTAG="abc". No ApplyChangeset call.
	t.Skip("stand up httptest server")
}

func TestSync_GroupVCard_Skipped(t *testing.T) {
	// One vCard with KIND:group; verify it does not appear in added.
	t.Skip("stand up httptest server")
}
