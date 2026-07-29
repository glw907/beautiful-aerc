package jmap

import (
	"cmp"
	"encoding/json"
	"slices"
)

// A RequestError is the RFC 7807 problem details body a server
// returns when it rejects a whole request (RFC 8620 section 3.6.1).
// No method ran.
type RequestError struct {
	// Type is the problem type URI, for example
	// "urn:ietf:params:jmap:error:limit".
	Type string `json:"type"`

	// Status is the HTTP status the server sent alongside the body.
	Status int `json:"status"`

	// Detail is server prose describing the problem.
	Detail string `json:"detail"`

	// Limit names the request limit the call would have exceeded.
	// Section 3.6.1 makes it mandatory on the limit problem type and
	// leaves it out elsewhere.
	Limit string `json:"limit,omitempty"`
}

// Error implements error. The detail is server-supplied text, so it
// is returned as data and never as a format string.
func (e *RequestError) Error() string {
	message := cmp.Or(e.Detail, e.Type)
	if e.Limit != "" {
		return message + " (limit: " + e.Limit + ")"
	}
	return message
}

// A MethodError is RFC 8620 section 3.6.2's "error" response, which
// takes the place of a method's own response when that method fails.
// The calls around it still run, so a caller reads it per call id
// rather than treating it as a failure of the request.
type MethodError struct {
	// Type names the error, for example "cannotCalculateChanges".
	Type string `json:"type"`

	// Description carries server prose on the errors that define it,
	// notably invalidArguments.
	Description string `json:"description,omitempty"`

	// Raw is the error object exactly as the server sent it, so an
	// error type this package does not name keeps its extra
	// properties instead of decaying to a bare string.
	Raw json.RawMessage `json:"-"`
}

// The method errors RFC 8620 registers. Match one with errors.Is,
// which compares the type and ignores whatever description the server
// attached: errors.Is(err, ErrCannotCalculateChanges) is the test
// that forces a full resync (section 5.2).
var (
	ErrServerUnavailable           = &MethodError{Type: "serverUnavailable"}
	ErrServerFail                  = &MethodError{Type: "serverFail"}
	ErrServerPartialFail           = &MethodError{Type: "serverPartialFail"}
	ErrUnknownMethod               = &MethodError{Type: "unknownMethod"}
	ErrInvalidArguments            = &MethodError{Type: "invalidArguments"}
	ErrInvalidResultReference      = &MethodError{Type: "invalidResultReference"}
	ErrForbidden                   = &MethodError{Type: "forbidden"}
	ErrAccountNotFound             = &MethodError{Type: "accountNotFound"}
	ErrAccountNotSupportedByMethod = &MethodError{Type: "accountNotSupportedByMethod"}
	ErrAccountReadOnly             = &MethodError{Type: "accountReadOnly"}
	ErrCannotCalculateChanges      = &MethodError{Type: "cannotCalculateChanges"}
	ErrAnchorNotFound              = &MethodError{Type: "anchorNotFound"}
)

// Error implements error.
func (e *MethodError) Error() string {
	if e.Description != "" {
		return e.Type + ": " + e.Description
	}
	return e.Type
}

// Is reports whether target is a MethodError naming the same type.
func (e *MethodError) Is(target error) bool {
	other, ok := target.(*MethodError)
	return ok && other.Type == e.Type
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *MethodError) UnmarshalJSON(data []byte) error {
	type methodError MethodError
	if err := json.Unmarshal(data, (*methodError)(e)); err != nil {
		return err
	}
	e.Raw = slices.Clone(data)
	return nil
}

// A SetError says why the server refused one record inside a /set,
// /copy, or /import call (RFC 8620 section 5.3). The call around it
// still succeeds and still reports a new state, so a caller reading
// only the method's error return believes every record landed.
//
// The extra properties below are the ones RFC 8620 and RFC 8621
// define. An error type neither RFC names keeps its whole payload in
// Raw.
type SetError struct {
	// Type names the error, for example "tooManyRecipients".
	Type string `json:"type"`

	// Description is server prose for debugging. RFC 8621 section
	// 7.5.1 has the server localise it on forbiddenToSend, which is
	// the one case meant for a user's eyes.
	Description string `json:"description,omitempty"`

	// Properties lists the record properties at fault: RFC 8620
	// section 5.3 defines it on invalidProperties, and RFC 8621
	// section 7.5 reuses it on invalidEmail.
	Properties []string `json:"properties,omitempty"`

	// NotFound lists every blob id an EmailBodyPart referenced that
	// the server does not hold (RFC 8621 section 4.6, blobNotFound).
	NotFound []ID `json:"notFound,omitempty"`

	// MaxRecipients is the ceiling the envelope exceeded (RFC 8621
	// section 7.5, tooManyRecipients).
	MaxRecipients uint64 `json:"maxRecipients,omitempty"`

	// InvalidRecipients lists the rcptTo addresses the server will not
	// send to (RFC 8621 section 7.5, invalidRecipients).
	InvalidRecipients []string `json:"invalidRecipients,omitempty"`

	// Raw is the error object exactly as the server sent it.
	Raw json.RawMessage `json:"-"`
}

// Error implements error, so a refused record travels up a caller's
// own error path without being rebuilt on the way.
func (e *SetError) Error() string {
	if e.Description != "" {
		return e.Type + ": " + e.Description
	}
	return e.Type
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *SetError) UnmarshalJSON(data []byte) error {
	type setError SetError
	if err := json.Unmarshal(data, (*setError)(e)); err != nil {
		return err
	}
	e.Raw = slices.Clone(data)
	return nil
}
