package jmap

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// An Invocation is RFC 8620 section 3.2's three-tuple, carrying one
// method call or one method response: the method name, its arguments,
// and the call id that ties a response back to its call.
type Invocation struct {
	// Name is the JMAP method name, or "error" on a response the
	// server could not fulfil.
	Name string

	// Args is a [Method] on a call. On a decoded response it is a
	// pointer to the response type registered for Name, or to a
	// [MethodError] when Name is "error".
	Args any

	// CallID is the client's label for the call. It need not be
	// unique: one call can produce more than one response under it.
	CallID string

	// Raw is the arguments object exactly as the server sent it, set
	// on decode and empty on a call the client built. Every response
	// property in this package carries omitempty, so re-marshaling
	// Args loses one the server sent as an empty array, and an RFC
	// 8620 section 3.7 pointer has to resolve against what arrived.
	Raw json.RawMessage
}

// MarshalJSON implements json.Marshaler.
func (i Invocation) MarshalJSON() ([]byte, error) {
	args, err := json.Marshal(i.Args)
	if err != nil {
		return nil, err
	}
	if err := checkReferenceCollision(args); err != nil {
		return nil, fmt.Errorf("%s: %w", i.Name, err)
	}
	return json.Marshal([3]any{i.Name, json.RawMessage(args), i.CallID})
}

// UnmarshalJSON implements json.Unmarshaler.
func (i *Invocation) UnmarshalJSON(data []byte) error {
	var tuple []json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return err
	}
	if len(tuple) != 3 {
		return fmt.Errorf("invocation has %d elements, want 3", len(tuple))
	}
	if err := json.Unmarshal(tuple[0], &i.Name); err != nil {
		return err
	}
	newArgs, ok := methodResponses[i.Name]
	if !ok {
		return fmt.Errorf("no response type registered for method %q", i.Name)
	}
	args := newArgs()
	if err := json.Unmarshal(tuple[1], args); err != nil {
		return err
	}
	if err := json.Unmarshal(tuple[2], &i.CallID); err != nil {
		return err
	}
	i.Args = args
	i.Raw = slices.Clone(tuple[1])
	return nil
}

// checkReferenceCollision reports an arguments object that names one
// argument in both its normal and its referenced form, which RFC 8620
// section 3.7 makes an invalidArguments error. The server would pick
// one of the two, so the argument the caller meant can go missing
// without a word.
//
// Arguments that are not a JSON object pass: RFC 8620 section 3.2
// requires an object, so anything else is already wrong in a way the
// server reports.
func checkReferenceCollision(args []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(args, &object); err != nil {
		return nil
	}
	for name := range object {
		normal, isReference := strings.CutPrefix(name, "#")
		if !isReference {
			continue
		}
		if _, both := object[normal]; both {
			return fmt.Errorf("argument %q given in both normal and referenced form", normal)
		}
	}
	return nil
}
