package jmap

import (
	"encoding/json"
	"fmt"
)

// A Session is RFC 8620 section 2's session resource: what the server
// can do, which accounts the credentials reach, and the URLs for the
// API, for blobs, and for the push stream.
type Session struct {
	// Capabilities holds the capability objects this package models,
	// decoded from RawCapabilities.
	Capabilities map[URI]Capability `json:"-"`

	// RawCapabilities is every capability the server advertised,
	// modelled or not, exactly as it arrived.
	RawCapabilities map[URI]json.RawMessage `json:"capabilities"`

	// Accounts maps an account id to the account it names.
	Accounts map[ID]Account `json:"accounts"`

	// PrimaryAccounts maps a capability to the account that is the
	// user's default for it.
	PrimaryAccounts map[URI]ID `json:"primaryAccounts"`

	// Username is the name the credentials authenticated as.
	Username string `json:"username"`

	// APIURL is where method calls are posted.
	APIURL string `json:"apiUrl"`

	// DownloadURL is an RFC 6570 level 1 template over accountId,
	// blobId, type, and name.
	DownloadURL string `json:"downloadUrl"`

	// UploadURL is an RFC 6570 level 1 template over accountId.
	UploadURL string `json:"uploadUrl"`

	// EventSourceURL is an RFC 6570 level 1 template over types,
	// closeafter, and ping.
	EventSourceURL string `json:"eventSourceUrl"`

	// State changes whenever anything else in the session does. It is
	// opaque: compare it for equality and refetch, never parse it.
	// Fastmail's value looks structured, and reading that structure
	// binds a client to one server's internals.
	State string `json:"state"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Session) UnmarshalJSON(data []byte) error {
	type session Session
	if err := json.Unmarshal(data, (*session)(s)); err != nil {
		return err
	}
	decoded, err := decodeCapabilities(s.RawCapabilities)
	if err != nil {
		return err
	}
	s.Capabilities = decoded
	return nil
}

// An Account is one collection of data the credentials reach (RFC
// 8620 section 1.6.2). [Session].Accounts keys each account by its id,
// so the account object carries no id of its own.
type Account struct {
	// Capabilities holds the account capability objects this package
	// models, decoded from RawCapabilities.
	Capabilities map[URI]Capability `json:"-"`

	// RawCapabilities is every capability the server offered for this
	// account, modelled or not, exactly as it arrived.
	RawCapabilities map[URI]json.RawMessage `json:"accountCapabilities"`

	// Name is a label to show for the account, often the owner's email
	// address.
	Name string `json:"name"`

	// IsPersonal reports whether the account belongs to the
	// authenticated user rather than being shared with them.
	IsPersonal bool `json:"isPersonal"`

	// IsReadOnly reports whether the account refuses every change.
	IsReadOnly bool `json:"isReadOnly"`
}

// UnmarshalJSON implements json.Unmarshaler.
func (a *Account) UnmarshalJSON(data []byte) error {
	type account Account
	if err := json.Unmarshal(data, (*account)(a)); err != nil {
		return err
	}
	decoded, err := decodeCapabilities(a.RawCapabilities)
	if err != nil {
		return err
	}
	a.Capabilities = decoded
	return nil
}

// decodeCapabilities decodes each advertised capability this package
// models. A URI the table does not name stays in raw alone.
func decodeCapabilities(raw map[URI]json.RawMessage) (map[URI]Capability, error) {
	decoded := make(map[URI]Capability, len(raw))
	for uri, prototype := range capabilities {
		object, ok := raw[uri]
		if !ok {
			continue
		}
		value := prototype.New()
		if err := json.Unmarshal(object, value); err != nil {
			return nil, fmt.Errorf("capability %s: %w", uri, err)
		}
		decoded[uri] = value
	}
	return decoded, nil
}
