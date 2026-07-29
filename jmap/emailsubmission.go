package jmap

// An EmailSubmission is one message handed to the server for delivery
// (RFC 8621 section 7).
type EmailSubmission struct {
	ID ID `json:"id,omitempty"`

	// IdentityID is the identity to send as.
	IdentityID ID `json:"identityId,omitempty"`

	// EmailID is the message to send. In a create it is usually the
	// creation id of a draft made in the same request, written with a
	// leading "#".
	EmailID ID `json:"emailId,omitempty"`

	ThreadID ID `json:"threadId,omitempty"`

	// Envelope is the SMTP envelope. Left out, the server derives one
	// from the message's header fields.
	Envelope *Envelope `json:"envelope,omitempty"`

	// SendAt holds the message until this instant. A server accepts a
	// delay only up to [Submission].MaxDelayedSend.
	SendAt *Date `json:"sendAt,omitempty"`

	// UndoStatus is "pending", "final", or "canceled". Only a pending
	// submission can still be recalled.
	UndoStatus string `json:"undoStatus,omitempty"`

	// DeliveryStatus maps each envelope recipient to what became of
	// their copy. A server that does not track delivery leaves it out.
	DeliveryStatus map[string]*DeliveryStatus `json:"deliveryStatus,omitempty"`

	// DSNBlobIDs names the delivery status notifications received for
	// this submission.
	DSNBlobIDs []ID `json:"dsnBlobIds,omitempty"`

	// MDNBlobIDs names the message disposition notifications received
	// for this submission.
	MDNBlobIDs []ID `json:"mdnBlobIds,omitempty"`
}

// An Envelope is the SMTP envelope of a submission (RFC 8621 section
// 7).
type Envelope struct {
	// MailFrom is the return path, where bounces go.
	MailFrom *EnvelopeAddress `json:"mailFrom,omitempty"`

	// RcptTo is who the message is delivered to, which need not match
	// the To, Cc, and Bcc header fields.
	RcptTo []*EnvelopeAddress `json:"rcptTo,omitempty"`
}

// An EnvelopeAddress is one SMTP envelope address and the extension
// parameters to send with it (RFC 8621 section 7).
type EnvelopeAddress struct {
	Email string `json:"email,omitempty"`

	// Parameters maps an SMTP extension parameter to its value, or to
	// nil for a parameter that takes none. Only parameters the server
	// advertised in [Submission].SubmissionExtensions are accepted.
	Parameters map[string]any `json:"parameters,omitempty"`
}

// A DeliveryStatus is what became of one recipient's copy (RFC 8621
// section 7).
type DeliveryStatus struct {
	// SMTPReply is the reply the receiving server gave, verbatim.
	SMTPReply string `json:"smtpReply,omitempty"`

	// Delivered is "queued", "yes", "no", or "unknown".
	Delivered string `json:"delivered,omitempty"`

	// Displayed is "unknown" or "yes".
	Displayed string `json:"displayed,omitempty"`
}

// EmailSubmissionSet sends messages and cancels pending sends (RFC
// 8621 section 7.5).
//
// The two on-success arguments run an implicit Email/set after the
// submissions land. Its response arrives under the same call id as
// this one, so a caller reads both from [Response.Invocations] rather
// than assuming one response per call.
type EmailSubmissionSet struct {
	Account ID `json:"accountId,omitempty"`

	IfInState string `json:"ifInState,omitempty"`

	// Create maps a creation id to the submission to make.
	Create map[ID]*EmailSubmission `json:"create,omitempty"`

	Update map[ID]Patch `json:"update,omitempty"`

	Destroy []ID `json:"destroy,omitempty"`

	// OnSuccessUpdateEmail patches each sent message once its
	// submission succeeds, keyed by the submission's creation id with
	// a leading "#". This is where a draft loses "$draft" and moves to
	// the sent mailbox, and it must not run when the send failed.
	OnSuccessUpdateEmail map[ID]Patch `json:"onSuccessUpdateEmail,omitempty"`

	// OnSuccessDestroyEmail destroys each sent message once its
	// submission succeeds, keyed the same way.
	OnSuccessDestroyEmail []ID `json:"onSuccessDestroyEmail,omitempty"`
}

func (*EmailSubmissionSet) Name() string { return "EmailSubmission/set" }

func (*EmailSubmissionSet) Requires() []URI { return []URI{SubmissionURI, MailURI} }

// EmailSubmissionSetResponse answers an EmailSubmissionSet.
type EmailSubmissionSetResponse struct {
	Account ID `json:"accountId,omitempty"`

	OldState string `json:"oldState,omitempty"`
	NewState string `json:"newState,omitempty"`

	Created map[ID]*EmailSubmission `json:"created,omitempty"`

	Updated map[ID]*EmailSubmission `json:"updated,omitempty"`

	Destroyed []ID `json:"destroyed,omitempty"`

	// NotCreated holds a refused submission per creation id. A caller
	// that reads only Created treats a refused send as a sent one.
	NotCreated map[ID]*SetError `json:"notCreated,omitempty"`

	NotUpdated   map[ID]*SetError `json:"notUpdated,omitempty"`
	NotDestroyed map[ID]*SetError `json:"notDestroyed,omitempty"`
}
