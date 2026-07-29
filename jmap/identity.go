package jmap

// An Identity is an address the user may send from, with the
// signature and reply-to settings that go with it (RFC 8621 section
// 6).
type Identity struct {
	ID ID `json:"id,omitempty"`

	// Name is the display name to put in the From header field.
	Name string `json:"name,omitempty"`

	// Email is the address to send from. It may hold a "*" local part,
	// meaning the user may send from any address in that domain.
	Email string `json:"email,omitempty"`

	ReplyTo []*Address `json:"replyTo,omitempty"`

	Bcc []*Address `json:"bcc,omitempty"`

	TextSignature string `json:"textSignature,omitempty"`

	HTMLSignature string `json:"htmlSignature,omitempty"`

	// MayDelete reports whether the user may destroy this identity.
	// A server-provisioned identity is often permanent.
	MayDelete bool `json:"mayDelete,omitempty"`
}

// IdentityGet fetches identities by id (RFC 8621 section 6.1). Empty
// IDs and ReferenceIDs together ask for every identity in the
// account.
type IdentityGet struct {
	Account ID `json:"accountId,omitempty"`

	IDs []ID `json:"ids,omitempty"`

	Properties []string `json:"properties,omitempty"`

	// ReferenceIDs takes the ids from an earlier call's result instead
	// of IDs. Setting both is an invalidArguments error.
	ReferenceIDs *ResultReference `json:"#ids,omitempty"`
}

func (*IdentityGet) Name() string { return "Identity/get" }

// Requires names the submission capability, not the mail one:
// identities live with EmailSubmission in RFC 8621 section 1.3.2.
func (*IdentityGet) Requires() []URI { return []URI{SubmissionURI} }

// IdentityGetResponse answers an IdentityGet.
type IdentityGetResponse struct {
	Account ID `json:"accountId,omitempty"`

	State string `json:"state,omitempty"`

	List []*Identity `json:"list,omitempty"`

	NotFound []ID `json:"notFound,omitempty"`
}
