package backend

// ThreadIdentity names how confidently a backend's server groups
// messages into threads. TH-1's no-false-merge criterion binds
// ReferencesDerived backends: a server that groups strictly by RFC
// 5322 References and In-Reply-To carries that guarantee. A
// ServerHeuristic backend, such as Gmail's subject-window threading,
// applies a heuristic beyond those headers and does not.
type ThreadIdentity int

const (
	// ThreadIdentityNone is a backend with no server thread signal;
	// poplar threads locally by References and In-Reply-To alone.
	ThreadIdentityNone ThreadIdentity = iota
	// ThreadIdentityReferencesDerived is a backend whose server
	// groups strictly by References and In-Reply-To.
	ThreadIdentityReferencesDerived
	// ThreadIdentityServerHeuristic is a backend whose server
	// applies its own grouping heuristic beyond References and
	// In-Reply-To.
	ThreadIdentityServerHeuristic
)

// PushTransport names the transport a backend's Push uses.
type PushTransport int

const (
	// PushTransportNone is a backend with no push transport; the
	// sync engine polls Changes on a timer instead.
	PushTransportNone PushTransport = iota
	// PushTransportEventSource is a server-sent-events push
	// transport.
	PushTransportEventSource
	// PushTransportLongPoll is a long-poll push transport.
	PushTransportLongPoll
)

// DeltaGranularity names the scope one Changes token covers.
type DeltaGranularity int

const (
	// DeltaGranularityAccount is one token for the whole account.
	DeltaGranularityAccount DeltaGranularity = iota
	// DeltaGranularityPerMailbox is a separate token per mailbox.
	DeltaGranularityPerMailbox
)

// RSVPMechanism names how a backend submits an attendee's response
// to a calendar invitation. A backend resolves this once, by probing
// the live server session, rather than assuming a protocol default.
type RSVPMechanism int

const (
	// RSVPMechanismUnknown is a backend that has not yet probed
	// which mechanism its server expects.
	RSVPMechanismUnknown RSVPMechanism = iota
	// RSVPMechanismCalDAVPatch replies by writing the attendee's
	// PARTSTAT directly onto the event resource.
	RSVPMechanismCalDAVPatch
	// RSVPMechanismITIPReply replies by sending an iTIP REPLY
	// message.
	RSVPMechanismITIPReply
)

// ServerLimits carries the numeric ceilings a live session reports.
type ServerLimits struct {
	// MaxObjectsInGet is the most records one Changes round trip may
	// hydrate.
	MaxObjectsInGet int
	// MaxObjectsInSet is the most mutations one ApplyBatch call may
	// carry.
	MaxObjectsInSet int
	// MaxCallsInRequest is the most method calls one batched request
	// may carry.
	MaxCallsInRequest int
	// MaxConcurrentRequests is the most requests the account's shared
	// request budget admits in flight at once (technical design
	// section 5).
	MaxConcurrentRequests int
	// MaxSizeUpload is the largest outgoing message Submit accepts,
	// in bytes.
	MaxSizeUpload int64
}

// Capabilities carries the facts engines branch on: what a backend's
// live session reports about itself, not what poplar assumes about
// the protocol behind it (ADR-0004 revision 2).
type Capabilities struct {
	ThreadIdentity   ThreadIdentity
	PushTransport    PushTransport
	DeltaGranularity DeltaGranularity
	ServerSearch     bool
	// ScheduledSend is whether the backend accepts a future send time
	// on Submit rather than poplar holding the message until then.
	ScheduledSend bool
	RSVP          RSVPMechanism
	Limits        ServerLimits
	// AccountIDs maps a capability name (mail, contacts, calendars)
	// to the account id the live session assigned it. A JMAP session
	// can assign a different account id per capability under one
	// login.
	AccountIDs map[string]string
}
