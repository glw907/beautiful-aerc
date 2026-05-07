// SPDX-License-Identifier: MIT

// Package contacts provides poplar's address-book UI surfaces:
// the i-popover, Contacts mode, and the contact edit form.
package contacts

// Kind distinguishes a person card from an organization card.
type Kind int

const (
	KindPerson Kind = iota
	KindOrg
)

// Contact is the value rendered by every surface in this package.
// It mirrors the schema in docs/superpowers/specs/2026-05-06-addressbook-design.md
// minus the storage columns (id, source, account, external_id, rev,
// updated_at). Those land in 9.2 alongside the SQLite layer.
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
// primary email. The form reorders the slice on change.
type Email struct {
	Address string
	Label   string // "work", "home", or "" for unlabeled
}

// Phone pairs an E.164 number with an optional label.
type Phone struct {
	E164  string
	Label string // "mobile", "work", "home", "fax", or ""
}

// Suggestion is one row in the compose autocomplete dropdown.
// A contact with N emails appears N times. Each suggestion is one
// (contact, email) pair with the org annotation flattened in.
type Suggestion struct {
	Name  string // person FN or org name
	Email string
	Org   string // dim suffix. Empty when Kind == KindOrg or Org unset.
	IsOrg bool
}
