package jmap

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// A URI identifies a JMAP capability.
type URI string

// The capability URIs this package models.
const (
	CoreURI        URI = "urn:ietf:params:jmap:core"
	MailURI        URI = "urn:ietf:params:jmap:mail"
	SubmissionURI  URI = "urn:ietf:params:jmap:submission"
	SMIMEVerifyURI URI = "urn:ietf:params:jmap:smimeverify"
)

// An ID identifies one record. The server assigns every id, and a
// client treats one as opaque bytes.
type ID string

// idAlphabet matches RFC 8620 section 1.2's allowed characters: the
// URL and filename safe base64 alphabet of RFC 4648 section 5, minus
// the pad character.
var idAlphabet = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Valid reports whether id meets RFC 8620 section 1.2: 1 to 255
// octets, each an ASCII alphanumeric, a hyphen, or an underscore.
//
// Marshaling never calls Valid, and no method in this package
// rejects an id. A creation-id reference travels in the same position
// as an id and carries a leading "#" that section 1.2's alphabet
// excludes (section 5.3), so validating on the way out would block a
// legal request. Ids from a server are the server's to allocate.
// Valid is for a caller checking an id it minted or parsed itself.
func (id ID) Valid() bool {
	return len(id) <= 255 && idAlphabet.MatchString(string(id))
}

// A Date is an RFC 8620 section 1.4 UTCDate. It marshals as an
// RFC 3339 timestamp in UTC whatever location the value carries, with
// the fractional second omitted when it is zero.
type Date time.Time

// MarshalJSON implements json.Marshaler.
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d).UTC().Format(time.RFC3339Nano))
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fmt.Errorf("date %q: %v", s, err)
	}
	*d = Date(t)
	return nil
}

// Time returns d as a time.Time in UTC.
func (d Date) Time() time.Time { return time.Time(d).UTC() }

// A CollationAlgo names a string comparison algorithm from the RFC
// 4790 registry. A server advertises the ones it supports in
// [Core].CollationAlgorithms.
type CollationAlgo string

// The collations RFC 8620 section 5.5 names.
const (
	// ASCIINumeric is defined in RFC 4790.
	ASCIINumeric CollationAlgo = "i;ascii-numeric"

	// ASCIICasemap is defined in RFC 4790.
	ASCIICasemap CollationAlgo = "i;ascii-casemap"

	// UnicodeCasemap is defined in RFC 5051.
	UnicodeCasemap CollationAlgo = "i;unicode-casemap"
)

// A ResultReference points at part of an earlier call's result, so a
// client can chain calls in one request without a round trip (RFC
// 8620 section 3.7). The server resolves it; a reference that does
// not resolve fails the whole method with invalidResultReference.
//
// A reference goes in the argument whose name is the target argument
// prefixed with "#". Sending both forms of one argument is an
// invalidArguments error, and [Invocation.MarshalJSON] refuses it.
type ResultReference struct {
	// ResultOf is the call id of an earlier call in this request.
	ResultOf string `json:"resultOf"`

	// Name is the method name of the response being referenced.
	Name string `json:"name"`

	// Path is an RFC 6901 JSON pointer into that response's
	// arguments, extended so "*" maps through an array.
	Path string `json:"path"`
}
