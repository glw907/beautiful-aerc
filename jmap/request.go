package jmap

import (
	"encoding/json"
	"slices"
	"strconv"
)

// A Request is RFC 8620 section 3.3's request object: the
// capabilities the server should apply, the calls to run in order,
// and an optional seed for the creation-id map.
type Request struct {
	// Using names the capabilities the request needs. [Request.Invoke]
	// maintains it, and CoreURI is always first.
	Using []URI `json:"using"`

	// MethodCalls holds the calls in the order the server runs them.
	MethodCalls []*Invocation `json:"methodCalls"`

	// CreatedIDs seeds the server's creation-id map, so a proxy can
	// carry ids across requests (section 3.3). The response echoes the
	// map back with every creation this request made.
	CreatedIDs map[ID]ID `json:"createdIds,omitempty"`
}

// Invoke appends m to the request, merges m's required capabilities
// into Using, and returns the call id it assigned. Call ids are the
// decimal index of the call, so the first is "0".
func (r *Request) Invoke(m Method) string {
	callID := strconv.Itoa(len(r.MethodCalls))
	r.MethodCalls = append(r.MethodCalls, &Invocation{
		Name:   m.Name(),
		Args:   m,
		CallID: callID,
	})
	r.Using = mergeURIs(r.Using, m.Requires())
	return callID
}

// MarshalJSON implements json.Marshaler. It writes "using" and
// "methodCalls" as arrays even when the request holds none: RFC 8620
// section 3.3 makes both mandatory, and a nil Go slice would reach the
// wire as null.
func (r Request) MarshalJSON() ([]byte, error) {
	type request Request
	out := request(r)
	if out.Using == nil {
		out.Using = []URI{}
	}
	if out.MethodCalls == nil {
		out.MethodCalls = []*Invocation{}
	}
	return json.Marshal(out)
}

// mergeURIs adds each URI in required that using does not already
// hold, keeping CoreURI first and everything else in first-seen order
// so one request marshals to the same bytes twice. Every request uses
// the core capability, whatever the methods in it declare and whatever
// a caller seeded Using with.
func mergeURIs(using, required []URI) []URI {
	if !slices.Contains(using, CoreURI) {
		using = append([]URI{CoreURI}, using...)
	}
	for _, uri := range required {
		if !slices.Contains(using, uri) {
			using = append(using, uri)
		}
	}
	return using
}
