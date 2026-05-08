// Package contacts holds poplar's plain-value contact types and
// the CardDAV ingest path. UI surfaces live in internal/ui/contacts;
// they import these types directly.
package contacts

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
	Name   string
	Family string
	Given  string
	Org    string
	Title  string
	Note   string
	Emails []Email
	Phones []Phone
}

// Email pairs an address with an optional label. Index 0 is the
// primary; the form reorders the slice on change.
type Email struct {
	Address string
	Label   string
}

// Phone pairs a number with an optional label. Stored as the user
// typed it; phonenumbers parsing only validates.
type Phone struct {
	E164  string
	Label string
}

// Suggestion is one row in the compose autocomplete dropdown.
type Suggestion struct {
	Name  string
	Email string
	Org   string
	IsOrg bool
}

// AddressBook is one CardDAV collection.
type AddressBook struct {
	Href        string
	DisplayName string
	Description string
}
