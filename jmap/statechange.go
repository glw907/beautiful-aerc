package jmap

// An EventType names a record type whose changes a client can
// subscribe to on a push channel, for example "Email".
type EventType string

// AllEvents subscribes to every type the server offers.
const AllEvents EventType = "*"

// A StateChange is what the server pushes when an account's data
// moves (RFC 8620 section 7.1.1). It carries no data, only the news
// that a /changes call would now return something.
type StateChange struct {
	// Type is always "StateChange".
	Type string `json:"@type"`

	// Changed maps an account id to the types that moved within it.
	// An account with nothing new is absent, and one push may cover
	// several accounts.
	Changed map[ID]TypeState `json:"changed"`
}

// A TypeState maps a record type name to the state a /get on that
// type would now report. A type this package does not model still
// appears, so a client can see a server extension move.
type TypeState map[string]string
