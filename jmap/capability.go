package jmap

// A Capability is a typed view of one entry in a session's
// capabilities object or an account's accountCapabilities object.
type Capability interface {
	// URI is the capability's identifier.
	URI() URI

	// New returns an empty value of the same concrete type, to decode
	// one server's advertisement into.
	New() Capability
}

// capabilities holds a prototype per capability this package models.
// A URI absent from the table is not lost: it survives in
// [Session].RawCapabilities and [Account].RawCapabilities and
// re-marshals unchanged, so a server extension this package has never
// heard of costs nothing.
var capabilities = map[URI]Capability{
	CoreURI:        &Core{},
	MailURI:        &Mail{},
	SubmissionURI:  &Submission{},
	SMIMEVerifyURI: &smimeVerify{},
}

// Core is the "urn:ietf:params:jmap:core" capability object: the
// limits every request works within (RFC 8620 section 2).
type Core struct {
	// MaxSizeUpload is the largest single upload, in octets.
	MaxSizeUpload uint64 `json:"maxSizeUpload"`

	// MaxConcurrentUpload is how many uploads may be in flight at once.
	MaxConcurrentUpload uint64 `json:"maxConcurrentUpload"`

	// MaxSizeRequest is the largest request body, in octets.
	MaxSizeRequest uint64 `json:"maxSizeRequest"`

	// MaxConcurrentRequests is how many API requests may be in flight
	// at once.
	MaxConcurrentRequests uint64 `json:"maxConcurrentRequests"`

	// MaxCallsInRequest is how many method calls one request may hold.
	MaxCallsInRequest uint64 `json:"maxCallsInRequest"`

	// MaxObjectsInGet is how many records one /get may ask for.
	MaxObjectsInGet uint64 `json:"maxObjectsInGet"`

	// MaxObjectsInSet is how many records one /set may create, update,
	// and destroy together.
	MaxObjectsInSet uint64 `json:"maxObjectsInSet"`

	// CollationAlgorithms names the collations a Comparator may ask
	// for.
	CollationAlgorithms []CollationAlgo `json:"collationAlgorithms"`
}

// URI implements Capability.
func (*Core) URI() URI { return CoreURI }

// New implements Capability.
func (*Core) New() Capability { return &Core{} }

// Mail is the "urn:ietf:params:jmap:mail" capability object (RFC 8621
// section 1.3.1). It is empty in a session's own capabilities and
// carries the limits per account.
type Mail struct {
	// MaxMailboxesPerEmail is how many mailboxes one message may sit
	// in. Nil is the server's spelling of no limit.
	MaxMailboxesPerEmail *uint64 `json:"maxMailboxesPerEmail"`

	// MaxMailboxDepth is how deep the mailbox tree may go. Nil is no
	// limit.
	MaxMailboxDepth *uint64 `json:"maxMailboxDepth"`

	// MaxSizeMailboxName is the longest mailbox name, in UTF-8 octets.
	MaxSizeMailboxName uint64 `json:"maxSizeMailboxName"`

	// MaxSizeAttachmentsPerEmail is the total unencoded attachment
	// size one message may carry, in octets.
	MaxSizeAttachmentsPerEmail uint64 `json:"maxSizeAttachmentsPerEmail"`

	// EmailQuerySortOptions names every property an Email/query sort
	// may compare. It may hold properties this package does not model.
	EmailQuerySortOptions []string `json:"emailQuerySortOptions"`

	// MayCreateTopLevelMailbox reports whether the user may create a
	// mailbox with no parent.
	MayCreateTopLevelMailbox bool `json:"mayCreateTopLevelMailbox"`
}

// URI implements Capability.
func (*Mail) URI() URI { return MailURI }

// New implements Capability.
func (*Mail) New() Capability { return &Mail{} }

// Submission is the "urn:ietf:params:jmap:submission" capability
// object (RFC 8621 section 1.3.2).
type Submission struct {
	// MaxDelayedSend is the longest send delay the server accepts, in
	// seconds. Zero means it does not hold messages back at all.
	MaxDelayedSend uint64 `json:"maxDelayedSend"`

	// SubmissionExtensions maps an SMTP ehlo-name to its ehlo-args.
	SubmissionExtensions map[string][]string `json:"submissionExtensions"`
}

// URI implements Capability.
func (*Submission) URI() URI { return SubmissionURI }

// New implements Capability.
func (*Submission) New() Capability { return &Submission{} }

// smimeVerify is the "urn:ietf:params:jmap:smimeverify" capability of
// RFC 9219. Its object carries no properties, so a caller reads it as
// presence in a session's capability map and nothing more.
type smimeVerify struct{}

func (*smimeVerify) URI() URI { return SMIMEVerifyURI }

func (*smimeVerify) New() Capability { return &smimeVerify{} }
