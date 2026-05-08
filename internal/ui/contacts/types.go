// Package contacts provides poplar's address-book UI surfaces:
// the i-popover, Contacts mode, and the contact edit form. The
// plain-value types (Contact, Email, Phone, Kind, Suggestion,
// AddressBook) live in github.com/glw907/poplar/internal/contacts;
// callers in this package import the same name from there.
package contacts

import core "github.com/glw907/poplar/internal/contacts"

// Re-exports keep existing call sites compiling. No behavior here.
type (
	Kind        = core.Kind
	Contact     = core.Contact
	Email       = core.Email
	Phone       = core.Phone
	Suggestion  = core.Suggestion
	AddressBook = core.AddressBook
)

const (
	KindPerson = core.KindPerson
	KindOrg    = core.KindOrg
)
