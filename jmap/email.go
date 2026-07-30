package jmap

import (
	"encoding/json"
	"slices"
)

// An Address is a name and an email address from a header field (RFC
// 8621 section 4.1.2).
type Address struct {
	// Name is the display name, already decoded from any RFC 2047
	// encoded-word.
	Name string `json:"name,omitempty"`

	// Email is the addr-spec, without angle brackets.
	Email string `json:"email,omitempty"`
}

// String renders a as an RFC 5322 address, with the display name when
// there is one.
func (a *Address) String() string {
	if a.Name == "" {
		return a.Email
	}
	return a.Name + " <" + a.Email + ">"
}

// An Email is one RFC 5322 message as the server parsed it (RFC 8621
// section 4).
type Email struct {
	ID ID `json:"id,omitempty"`

	// BlobID names the raw message, for download or for a resend.
	BlobID ID `json:"blobId,omitempty"`

	// ThreadID is the conversation the server grouped this message
	// into. The grouping is the server's to compute (section 3).
	ThreadID ID `json:"threadId,omitempty"`

	// MailboxIDs holds every mailbox the message sits in, each mapped
	// to true. A message in none is invisible.
	MailboxIDs map[ID]bool `json:"mailboxIds,omitempty"`

	// Keywords holds every set keyword mapped to true, for example
	// "$seen" and "$draft".
	Keywords map[string]bool `json:"keywords,omitempty"`

	Size uint64 `json:"size,omitempty"`

	// ReceivedAt is when the server took delivery, which orders a
	// mailbox even when a sender's clock is wrong.
	ReceivedAt *Date `json:"receivedAt,omitempty"`

	// Headers is every header field in the order it appeared.
	Headers []*Header `json:"headers,omitempty"`

	MessageID  []string `json:"messageId,omitempty"`
	InReplyTo  []string `json:"inReplyTo,omitempty"`
	References []string `json:"references,omitempty"`

	Sender  []*Address `json:"sender,omitempty"`
	From    []*Address `json:"from,omitempty"`
	To      []*Address `json:"to,omitempty"`
	CC      []*Address `json:"cc,omitempty"`
	BCC     []*Address `json:"bcc,omitempty"`
	ReplyTo []*Address `json:"replyTo,omitempty"`

	Subject string `json:"subject,omitempty"`

	// SentAt is the Date header field, which the sender chose.
	SentAt *Date `json:"sentAt,omitempty"`

	// BodyStructure is the whole MIME tree.
	BodyStructure *BodyPart `json:"bodyStructure,omitempty"`

	// BodyValues maps a part id to its fetched content. A /get returns
	// content only for the parts it was asked for.
	BodyValues map[string]*BodyValue `json:"bodyValues,omitempty"`

	// TextBody is the parts to show as the plain-text body.
	TextBody []*BodyPart `json:"textBody,omitempty"`

	// HTMLBody is the parts to show as the HTML body.
	HTMLBody []*BodyPart `json:"htmlBody,omitempty"`

	Attachments []*BodyPart `json:"attachments,omitempty"`

	HasAttachment bool `json:"hasAttachment,omitempty"`

	// Preview is a short plain-text extract the server prepared.
	Preview string `json:"preview,omitempty"`

	// The S/MIME properties come from RFC 9219 and appear only when
	// the server advertises SMIMEVerifyURI.
	SMIMEStatus           string   `json:"smimeStatus,omitempty"`
	SMIMEStatusAtDelivery string   `json:"smimeStatusAtDelivery,omitempty"`
	SMIMEErrors           []string `json:"smimeErrors,omitempty"`
	SMIMEVerifiedAt       *Date    `json:"smimeVerifiedAt,omitempty"`
}

// A Header is one header field, name and raw value (RFC 8621 section
// 4.1.1).
type Header struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// A BodyPart is one node of a message's MIME tree (RFC 8621 section
// 4.1.4).
type BodyPart struct {
	// PartID identifies the part's content in Email.BodyValues. It is
	// empty on a part that holds other parts.
	PartID string `json:"partId,omitempty"`

	// BlobID names the part's raw content for download. It is empty on
	// a multipart node.
	BlobID ID `json:"blobId,omitempty"`

	Size uint64 `json:"size,omitempty"`

	Headers []*Header `json:"headers,omitempty"`

	// Name is the filename from the Content-Disposition or
	// Content-Type header field.
	Name string `json:"name,omitempty"`

	// Type is the media type, lowercased and without parameters.
	Type string `json:"type,omitempty"`

	Charset string `json:"charset,omitempty"`

	// Disposition is "inline", "attachment", or another
	// Content-Disposition value.
	Disposition string `json:"disposition,omitempty"`

	// CID is the Content-ID, which an HTML body references to inline
	// an image.
	CID string `json:"cid,omitempty"`

	Language []string `json:"language,omitempty"`

	Location string `json:"location,omitempty"`

	SubParts []*BodyPart `json:"subParts,omitempty"`
}

// A BodyValue is the decoded content of one body part (RFC 8621
// section 4.1.4).
type BodyValue struct {
	Value string `json:"value,omitempty"`

	// IsEncodingProblem reports that the server could not decode the
	// part cleanly, so Value holds replacement characters.
	IsEncodingProblem bool `json:"isEncodingProblem,omitempty"`

	// IsTruncated reports that Value stops short of the whole part,
	// because EmailGet.MaxBodyValueBytes capped the bytes fetched.
	// Rendering a truncated body as a whole one is silent data loss,
	// so a caller that sets that cap reads this back.
	IsTruncated bool `json:"isTruncated,omitempty"`
}

// EmailGet fetches messages by id (RFC 8621 section 4.2).
type EmailGet struct {
	Account ID `json:"accountId,omitempty"`

	IDs []ID `json:"ids,omitempty"`

	// Properties limits the properties the server returns.
	Properties []string `json:"properties,omitempty"`

	// BodyProperties limits the properties on each BodyPart.
	BodyProperties []string `json:"bodyProperties,omitempty"`

	FetchTextBodyValues bool `json:"fetchTextBodyValues,omitempty"`
	FetchHTMLBodyValues bool `json:"fetchHTMLBodyValues,omitempty"`
	FetchAllBodyValues  bool `json:"fetchAllBodyValues,omitempty"`

	// MaxBodyValueBytes caps each fetched body value. A value the cap
	// cut short comes back with IsTruncated set.
	MaxBodyValueBytes uint64 `json:"maxBodyValueBytes,omitempty"`

	// ReferenceIDs takes the ids from an earlier call's result instead
	// of IDs. Setting both is an invalidArguments error.
	ReferenceIDs *ResultReference `json:"#ids,omitempty"`
}

func (*EmailGet) Name() string { return "Email/get" }

// Requires names the S/MIME capability alongside the mail one when
// the call asks for a property that comes from it.
func (m *EmailGet) Requires() []URI { return withSMIME(smimeProperties(m.Properties)) }

// EmailGetResponse answers an EmailGet.
type EmailGetResponse struct {
	Account ID `json:"accountId,omitempty"`

	State string `json:"state,omitempty"`

	List []*Email `json:"list,omitempty"`

	// NotFound echoes the requested ids the account does not hold.
	// A server that omits it has said nothing, which is not the same
	// as saying nothing was missing.
	NotFound []ID `json:"notFound,omitempty"`
}

// EmailChanges lists what moved since a state (RFC 8621 section 4.3).
type EmailChanges struct {
	Account ID `json:"accountId,omitempty"`

	SinceState string `json:"sinceState,omitempty"`

	MaxChanges uint64 `json:"maxChanges,omitempty"`
}

func (*EmailChanges) Name() string { return "Email/changes" }

func (*EmailChanges) Requires() []URI { return []URI{MailURI} }

// EmailChangesResponse answers an EmailChanges.
type EmailChangesResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldState string `json:"oldState,omitempty"`
	NewState string `json:"newState,omitempty"`

	HasMoreChanges bool `json:"hasMoreChanges,omitempty"`

	Created   []ID `json:"created,omitempty"`
	Updated   []ID `json:"updated,omitempty"`
	Destroyed []ID `json:"destroyed,omitempty"`
}

// EmailQuery lists message ids matching a filter, in a sort order
// (RFC 8621 section 4.4).
type EmailQuery struct {
	Account ID `json:"accountId,omitempty"`

	// Filter is left out entirely when nil. A server is within its
	// rights to reject an explicit null.
	Filter Filter `json:"filter,omitempty"`

	Sort []*Comparator `json:"sort,omitempty"`

	Position int64 `json:"position,omitempty"`

	// Anchor pages from a known id instead of an offset, so a
	// concurrent change cannot shift the window. An anchor the query
	// no longer matches fails with anchorNotFound.
	Anchor ID `json:"anchor,omitempty"`

	AnchorOffset int64 `json:"anchorOffset,omitempty"`

	Limit uint64 `json:"limit,omitempty"`

	CalculateTotal bool `json:"calculateTotal,omitempty"`

	// CollapseThreads returns one message per thread rather than every
	// matching message.
	CollapseThreads bool `json:"collapseThreads,omitempty"`
}

func (*EmailQuery) Name() string { return "Email/query" }

// Requires names the S/MIME capability alongside the mail one when
// the filter constrains on a condition that comes from it.
func (m *EmailQuery) Requires() []URI { return withSMIME(smimeFilter(m.Filter)) }

// MarshalJSON implements json.Marshaler. It drops a Filter holding a
// typed nil rather than sending "filter": null, and carries the
// filter it keeps as bytes so the tree is marshalled once.
func (m EmailQuery) MarshalJSON() ([]byte, error) {
	filter, err := omitNullFilter(m.Filter)
	if err != nil {
		return nil, err
	}

	type emailQuery EmailQuery
	out := emailQuery(m)
	out.Filter = filter
	return json.Marshal(out)
}

// EmailQueryResponse answers an EmailQuery. The ids arrive in the
// server's order, which is the order to show them in.
type EmailQueryResponse struct {
	Account ID `json:"accountId,omitempty"`

	QueryState string `json:"queryState,omitempty"`

	CanCalculateChanges bool `json:"canCalculateChanges,omitempty"`

	Position uint64 `json:"position,omitempty"`

	IDs []ID `json:"ids,omitempty"`

	Total uint64 `json:"total,omitempty"`

	Limit uint64 `json:"limit,omitempty"`
}

// EmailSet creates, updates, and destroys messages (RFC 8621 section
// 4.6). Each record succeeds or fails on its own.
type EmailSet struct {
	Account ID `json:"accountId,omitempty"`

	IfInState string `json:"ifInState,omitempty"`

	Create map[ID]*Email `json:"create,omitempty"`

	// Update patches a message. A patch built with [Pointer] changes
	// one keyword or one mailbox membership; a whole-property value
	// replaces the set.
	Update map[ID]Patch `json:"update,omitempty"`

	Destroy []ID `json:"destroy,omitempty"`
}

func (*EmailSet) Name() string { return "Email/set" }

func (*EmailSet) Requires() []URI { return []URI{MailURI} }

// EmailSetResponse answers an EmailSet. The six result maps are
// independent: a call that created one message and failed to create
// two more fills both Created and NotCreated.
type EmailSetResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldState string `json:"oldState,omitempty"`
	NewState string `json:"newState,omitempty"`

	Created map[ID]*Email `json:"created,omitempty"`

	// Updated maps an id to the properties the server changed beyond
	// what was asked. A null value, which decodes to a nil pointer,
	// means it changed nothing else.
	Updated map[ID]*Email `json:"updated,omitempty"`

	Destroyed []ID `json:"destroyed,omitempty"`

	NotCreated   map[ID]*SetError `json:"notCreated,omitempty"`
	NotUpdated   map[ID]*SetError `json:"notUpdated,omitempty"`
	NotDestroyed map[ID]*SetError `json:"notDestroyed,omitempty"`
}

// EmailImport files already-uploaded messages into mailboxes (RFC
// 8621 section 4.8).
type EmailImport struct {
	Account ID `json:"accountId,omitempty"`

	IfInState string `json:"ifInState,omitempty"`

	// Emails maps a creation id to the message to import.
	Emails map[ID]*EmailImportItem `json:"emails,omitempty"`
}

func (*EmailImport) Name() string { return "Email/import" }

func (*EmailImport) Requires() []URI { return []URI{MailURI} }

// An EmailImportItem is RFC 8621 section 4.8's EmailImport object:
// one uploaded blob and where to file it.
type EmailImportItem struct {
	BlobID ID `json:"blobId,omitempty"`

	MailboxIDs map[ID]bool `json:"mailboxIds,omitempty"`

	Keywords map[string]bool `json:"keywords,omitempty"`

	// ReceivedAt defaults to the time of the import.
	ReceivedAt *Date `json:"receivedAt,omitempty"`
}

// EmailImportResponse answers an EmailImport.
type EmailImportResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldState string `json:"oldState,omitempty"`
	NewState string `json:"newState,omitempty"`

	Created map[ID]*Email `json:"created,omitempty"`

	NotCreated map[ID]*SetError `json:"notCreated,omitempty"`
}

// An EmailFilterCondition matches messages in an EmailQuery (RFC 8621
// section 4.4.1). Every property set must match.
type EmailFilterCondition struct {
	InMailbox ID `json:"inMailbox,omitempty"`

	// InMailboxOtherThan excludes messages that sit only in these
	// mailboxes.
	InMailboxOtherThan []ID `json:"inMailboxOtherThan,omitempty"`

	// Before matches messages received strictly before this instant.
	Before *Date `json:"before,omitempty"`

	// After matches messages received at or after this instant.
	After *Date `json:"after,omitempty"`

	MinSize uint64 `json:"minSize,omitempty"`
	MaxSize uint64 `json:"maxSize,omitempty"`

	AllInThreadHaveKeyword  string `json:"allInThreadHaveKeyword,omitempty"`
	SomeInThreadHaveKeyword string `json:"someInThreadHaveKeyword,omitempty"`
	NoneInThreadHaveKeyword string `json:"noneInThreadHaveKeyword,omitempty"`

	HasKeyword string `json:"hasKeyword,omitempty"`
	NotKeyword string `json:"notKeyword,omitempty"`

	// HasAttachment nil places no constraint. new(false) matches only
	// messages carrying no attachment, which is a different question
	// from not asking.
	HasAttachment *bool `json:"hasAttachment,omitempty"`

	Text    string `json:"text,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Cc      string `json:"cc,omitempty"`
	Bcc     string `json:"bcc,omitempty"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`

	// Header is a one- or two-element list: a field name alone matches
	// messages carrying it, a name and a value matches on both.
	Header []string `json:"header,omitempty"`

	// The S/MIME conditions come from RFC 9219. Nil places no
	// constraint; new(false) is the opposite constraint.
	HasSMIME                   *bool `json:"hasSmime,omitempty"`
	HasVerifiedSMIME           *bool `json:"hasVerifiedSmime,omitempty"`
	HasVerifiedSMIMEAtDelivery *bool `json:"hasVerifiedSmimeAtDelivery,omitempty"`
}

func (*EmailFilterCondition) isFilter() {}

// withSMIME returns the capabilities a mail method needs, adding RFC
// 9219's when the call depends on it.
//
// RFC 8620 section 1.8 has a server behave as though it does not
// implement a capability the request never named, so section 5.5's
// unsupportedFilter is what a hasVerifiedSmime condition sent without
// this URI gets: the call fails rather than matching anything. Naming it on every call is the
// opposite failure, and a worse one, because section 3.3 has the
// server reject the whole request with unknownCapability for a URI it
// does not advertise. Stalwart advertises sixteen capabilities and
// this is not among them, so an unconditional mention would end every
// Email/get against it.
func withSMIME(needed bool) []URI {
	if needed {
		return []URI{MailURI, SMIMEVerifyURI}
	}
	return []URI{MailURI}
}

// smimeProperties reports whether a /get asks for one of the four
// properties RFC 9219 adds to Email.
func smimeProperties(properties []string) bool {
	return slices.ContainsFunc(properties, func(property string) bool {
		switch property {
		case "smimeStatus", "smimeStatusAtDelivery", "smimeErrors", "smimeVerifiedAt":
			return true
		}
		return false
	})
}

// smimeFilter reports whether a filter tree constrains on one of the
// three conditions RFC 9219 adds, at any depth: an operator nests
// conditions, and the condition that needs the capability is as
// likely to be inside one as at the top.
func smimeFilter(filter Filter) bool {
	switch f := filter.(type) {
	case *EmailFilterCondition:
		return f != nil &&
			(f.HasSMIME != nil || f.HasVerifiedSMIME != nil || f.HasVerifiedSMIMEAtDelivery != nil)
	case *FilterOperator:
		return f != nil && slices.ContainsFunc(f.Conditions, smimeFilter)
	}
	return false
}
