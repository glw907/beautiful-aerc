// Package contacts holds poplar's plain-value contact types and
// the CardDAV ingest path. UI surfaces live in internal/ui/contacts;
// they import these types directly.
package contacts

import (
	"context"

	"github.com/emersion/go-webdav/carddav"
)

// Kind distinguishes a person card from an organization card.
type Kind int

const (
	KindPerson Kind = iota
	KindOrg
)

// Contact is the value rendered by every UI surface. Storage
// columns (uid, href, etag, vcard blob) live in the cache layer
// and are not part of this projection.
type Contact struct {
	Kind   Kind
	Name   string // FN. For KindOrg this is the entire visible identity.
	Family string // empty for KindOrg
	Given  string // empty for KindOrg
	Org    string // empty for KindOrg
	Title  string // empty for KindOrg
	Note   string
	Emails []Email
	Phones []Phone
}

// Email pairs an address with an optional label. Index 0 is the
// primary; the form reorders the slice on change.
type Email struct {
	Address string
	Label   string // "work", "home", or "" for unlabeled
}

// Phone pairs a number with an optional label. Stored as the user
// typed it; phonenumbers parsing only validates.
type Phone struct {
	E164  string
	Label string // "mobile", "work", "home", "fax", or ""
}

// Suggestion is one row in the compose autocomplete dropdown: one row
// per (contact, email) pair, with the org annotation flattened in.
type Suggestion struct {
	Name  string // person FN or org name
	Email string
	Org   string // dim suffix. Empty when Kind == KindOrg or Org unset.
	IsOrg bool
}

// AddressBook is one CardDAV collection.
type AddressBook struct {
	Href        string
	DisplayName string
	Description string
}

// Writer is the CardDAV write seam. cache.Account wires it at startup;
// tests supply a fake. The cache drainer calls through it on ContactPut
// and ContactDelete ops.
type Writer interface {
	PutAddressObject(ctx context.Context, href, ifMatch string, body []byte) (newHref, newETag string, err error)
	DeleteAddressObject(ctx context.Context, href, ifMatch string) error
	Multiget(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error)
}
