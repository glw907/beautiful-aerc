package contacts

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
)

// Store is the persistence seam. internal/cache implements it; the
// sync engine has no SQLite dependency.
type Store interface {
	// Books returns cached book state keyed by href.
	Books(ctx context.Context) (map[string]BookState, error)

	// UpsertBook persists discovered or refreshed metadata. Empty
	// SyncToken and CTAG are valid (uninitialized).
	UpsertBook(ctx context.Context, b BookState) error

	// ApplyChangeset writes added/updated contacts and removes deleted
	// ones atomically per book. token and ctag are stored only on
	// success.
	ApplyChangeset(ctx context.Context, bookHref string, added []Stored, removed []string, token, ctag string) error
}

// BookState is the cache projection of one address book row.
type BookState struct {
	Href         string
	DisplayName  string
	Description  string
	SyncToken    string
	CTAG         string
	SupportsSync bool
	LastSyncedAt time.Time
}

// Stored is one parsed vCard plus its server identity.
type Stored struct {
	Parsed
	Href string
	ETag string
}

// Sync runs one pass over every discovered book for the given client.
func Sync(ctx context.Context, c *Client, s Store) error {
	home, err := c.HomeSet(ctx)
	if err != nil {
		return fmt.Errorf("home set: %w", err)
	}
	books, err := c.AddressBooks(ctx, home)
	if err != nil {
		return fmt.Errorf("list books: %w", err)
	}

	cached, err := s.Books(ctx)
	if err != nil {
		return fmt.Errorf("load cached books: %w", err)
	}

	for _, b := range books {
		state := cached[b.Path]
		state.Href = b.Path
		state.DisplayName = b.Name
		state.Description = b.Description
		if err := syncOne(ctx, c, s, &state); err != nil {
			return fmt.Errorf("book %s: %w", b.Path, err)
		}
		state.LastSyncedAt = time.Now()
		if err := s.UpsertBook(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func syncOne(ctx context.Context, c *Client, s Store, state *BookState) error {
	if state.SupportsSync && state.SyncToken != "" {
		return syncIncremental(ctx, c, s, state)
	}
	if state.CTAG != "" {
		return syncCTAG(ctx, c, s, state)
	}
	return syncFull(ctx, c, s, state)
}

func syncIncremental(ctx context.Context, c *Client, s Store, state *BookState) error {
	resp, err := c.SyncCollection(ctx, state.Href, &SyncQuery{SyncToken: state.SyncToken})
	if err != nil {
		// Token rejection: fall back to full pull.
		state.SyncToken = ""
		return syncFull(ctx, c, s, state)
	}
	added, removed, err := materializeChangeset(ctx, c, state.Href, resp)
	if err != nil {
		return err
	}
	if err := s.ApplyChangeset(ctx, state.Href, added, removed, resp.SyncToken, state.CTAG); err != nil {
		return err
	}
	state.SyncToken = resp.SyncToken
	return nil
}

func syncCTAG(ctx context.Context, c *Client, s Store, state *BookState) error {
	ctag, err := c.CTAG(ctx, state.Href)
	if err == nil && ctag == state.CTAG && state.CTAG != "" {
		return nil
	}
	if err := syncFull(ctx, c, s, state); err != nil {
		return err
	}
	if ctag != "" {
		state.CTAG = ctag
	}
	return nil
}

func syncFull(ctx context.Context, c *Client, s Store, state *BookState) error {
	hrefs, etags, err := listAllResources(ctx, c, state.Href)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	objs, err := c.Multiget(ctx, state.Href, hrefs)
	if err != nil {
		return fmt.Errorf("multiget: %w", err)
	}
	added := make([]Stored, 0, len(objs))
	for i, obj := range objs {
		p, err := parseObject(obj)
		if err != nil || p.Skip {
			continue
		}
		added = append(added, Stored{Parsed: p, Href: hrefs[i], ETag: etags[i]})
	}
	if err := s.ApplyChangeset(ctx, state.Href, added, nil, state.SyncToken, state.CTAG); err != nil {
		return err
	}
	if !state.SupportsSync {
		if probe, err := c.SyncCollection(ctx, state.Href, &SyncQuery{SyncToken: ""}); err == nil {
			state.SupportsSync = true
			state.SyncToken = probe.SyncToken
		}
	}
	return nil
}

// materializeChangeset converts a SyncResponse into Stored rows for
// adds/changes and a slice of removed hrefs. Objects in r.Updated already
// carry inline Card data; any with an empty card fall back through Multiget.
func materializeChangeset(ctx context.Context, c *Client, bookHref string, r *SyncResponse) ([]Stored, []string, error) {
	var added []Stored
	var missing []string
	missingSet := map[string]struct{}{}

	for _, obj := range r.Updated {
		if len(obj.Card) == 0 {
			missingSet[obj.Path] = struct{}{}
			missing = append(missing, obj.Path)
			continue
		}
		p, err := parseObject(obj)
		if err != nil || p.Skip {
			continue
		}
		added = append(added, Stored{Parsed: p, Href: obj.Path, ETag: obj.ETag})
	}

	if len(missing) > 0 {
		fetched, err := c.Multiget(ctx, bookHref, missing)
		if err != nil {
			return nil, nil, fmt.Errorf("multiget missing objects: %w", err)
		}
		for _, obj := range fetched {
			if _, ok := missingSet[obj.Path]; !ok {
				continue
			}
			p, err := parseObject(obj)
			if err != nil || p.Skip {
				continue
			}
			added = append(added, Stored{Parsed: p, Href: obj.Path, ETag: obj.ETag})
		}
	}

	return added, r.Deleted, nil
}

// listAllResources fetches every address object path and etag under
// bookHref via an addressbook-query REPORT with no filter.
func listAllResources(ctx context.Context, c *Client, bookHref string) ([]string, []string, error) {
	objs, err := c.cl.QueryAddressBook(ctx, bookHref, &carddav.AddressBookQuery{})
	if err != nil {
		return nil, nil, fmt.Errorf("query address book: %w", err)
	}
	hrefs := make([]string, len(objs))
	etags := make([]string, len(objs))
	for i, obj := range objs {
		hrefs[i] = obj.Path
		etags[i] = obj.ETag
	}
	return hrefs, etags, nil
}

func parseObject(obj carddav.AddressObject) (Parsed, error) {
	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(obj.Card); err != nil {
		return Parsed{}, fmt.Errorf("encode card %s: %w", obj.Path, err)
	}
	return ParseVCard(&buf)
}
