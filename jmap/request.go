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
	// maintains it, and CoreURI is always first. It is only a preview:
	// [Request.MarshalJSON] folds every call's Requires() again at
	// marshal time, so a wire request is always correct even when a
	// caller mutates a method's fields after Invoke returns.
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
//
// Using is folded again here from every call's own Requires(), rather
// than trusting the value Invoke already merged in. A caller can
// still mutate a method's fields after Invoke returns, and a filter
// condition added that way needs a capability Invoke had no chance to
// see. The wire request should declare what its calls actually need,
// and folding again at the point the bytes are built is what keeps
// that declaration correct regardless of what happened in between.
func (r Request) MarshalJSON() ([]byte, error) {
	type request Request
	out := request(r)
	// out.Using is a slice header copied from r.Using, which can still
	// share a backing array with the caller's own Request: a struct
	// copy does not copy what a slice points at. Clip drops any spare
	// capacity so no append below can reach the caller's array: the
	// first one is forced to allocate, and the rest land in the array
	// it made, which is this function's own.
	out.Using = slices.Clip(out.Using)
	for _, call := range out.MethodCalls {
		if m, ok := call.Args.(Method); ok {
			out.Using = mergeURIs(out.Using, m.Requires())
		}
	}
	if out.Using == nil {
		out.Using = []URI{}
	}
	if out.MethodCalls == nil {
		out.MethodCalls = []*Invocation{}
	}
	return json.Marshal(out)
}

// mergeURIs adds each URI in required that using does not already
// hold, placing CoreURI first when the caller did not already name it
// and keeping everything else in first-seen order, so one request
// marshals to the same bytes twice. Every request that invokes a
// method uses the core capability, whatever those methods declare.
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
