package jmap

// A Response is RFC 8620 section 3.4's response object.
type Response struct {
	// MethodResponses holds every response the server produced, in the
	// order it produced them.
	//
	// It stays a slice rather than a map keyed by call id because one
	// call can answer more than once under a single id: section 3.4.1
	// shows a method returning two responses, and RFC 8621 section
	// 7.5.1's EmailSubmission/set returns its own response plus an
	// implicit Email/set under the same id. Keying by call id drops the
	// second, which is how a message reads as sent when the send failed.
	MethodResponses []*Invocation `json:"methodResponses"`

	// CreatedIDs maps every creation id used in the request to the id
	// the server assigned.
	CreatedIDs map[ID]ID `json:"createdIds,omitempty"`

	// SessionState is the session state at the time of the response.
	// It is opaque: compare it for equality, and refetch the session
	// when it differs from the one in hand.
	SessionState string `json:"sessionState"`
}

// Invocations returns every response carrying callID, in the order
// the server sent them. The result is empty when no response carries
// it, which is itself a failure a caller must notice.
func (r *Response) Invocations(callID string) []*Invocation {
	var found []*Invocation
	for _, inv := range r.MethodResponses {
		if inv.CallID == callID {
			found = append(found, inv)
		}
	}
	return found
}
