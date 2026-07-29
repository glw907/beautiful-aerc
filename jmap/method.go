package jmap

// A Method is one JMAP method call: an arguments object that knows
// its own method name and the capabilities the server must have
// advertised before it can run.
//
// Every method type in this package implements it, so the concrete
// Name and Requires methods carry no documentation of their own.
type Method interface {
	// Name is the JMAP method name, for example "Mailbox/get".
	Name() string

	// Requires names the capability URIs the call needs in the
	// request's "using" list. CoreURI need not appear: every request
	// carries it.
	Requires() []URI
}

// methodResponses gives, per method name, the value a response under
// that name decodes into.
//
// The table is fixed at build time rather than filled by registration
// calls, so decoding has no import-order or concurrency question in
// it. A method name absent here fails [Invocation.UnmarshalJSON] by
// name instead of decoding to nothing.
var methodResponses = map[string]func() any{
	"error": func() any { return &MethodError{} },

	"Core/echo": func() any { return &Echo{} },

	"Mailbox/get":     func() any { return &MailboxGetResponse{} },
	"Mailbox/changes": func() any { return &MailboxChangesResponse{} },
	"Mailbox/query":   func() any { return &MailboxQueryResponse{} },

	"Mailbox/queryChanges": func() any { return &MailboxQueryChangesResponse{} },
	"Mailbox/set":          func() any { return &MailboxSetResponse{} },

	"Email/get":     func() any { return &EmailGetResponse{} },
	"Email/changes": func() any { return &EmailChangesResponse{} },
	"Email/query":   func() any { return &EmailQueryResponse{} },

	"Email/queryChanges": func() any { return &EmailQueryChangesResponse{} },
	"Email/set":          func() any { return &EmailSetResponse{} },
	"Email/import":       func() any { return &EmailImportResponse{} },

	"Identity/get": func() any { return &IdentityGetResponse{} },

	"EmailSubmission/set": func() any { return &EmailSubmissionSetResponse{} },
}
