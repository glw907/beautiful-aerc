package jmap

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Resolve evaluates ref against r's method responses and returns the
// JSON value it selects (RFC 8620 section 3.7).
//
// The server does this itself for a reference sent in a request; a
// client resolves to check what the server worked from, or to chain
// two calls locally when it did not send them together.
//
// Every failure comes back as invalidResultReference, matched with
// errors.Is against [ErrInvalidResultReference], and the returned
// value is nil. Section 3.7 rejects the whole method on a reference
// that does not resolve, so degrading to an empty list would hand a
// caller the id set for "nothing matched" when the truth is "the
// question was never asked".
//
// Evaluation reads the raw arguments the server sent, not the decoded
// [Invocation].Args: every response property in this package carries
// omitempty, so a property the server sent as an empty array is gone
// from a re-marshalled Args.
func (r *Response) Resolve(ref ResultReference) (json.RawMessage, error) {
	for _, inv := range r.MethodResponses {
		if inv.CallID != ref.ResultOf {
			continue
		}
		// Section 3.7 step 2 checks the name of the first response
		// under the call id and fails rather than looking further. A
		// call that failed answers under its own id with the name
		// "error", so a reference into it lands here.
		if inv.Name != ref.Name {
			return nil, referenceError("call %q answered %q, not %q", ref.ResultOf, inv.Name, ref.Name)
		}
		value, err := evalPointer(inv.Raw, ref.Path)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, referenceError("no call answered under id %q", ref.ResultOf)
}

// referenceError builds the invalidResultReference RFC 8620 section
// 3.7 mandates, carrying which of its conditions the reference broke.
func referenceError(format string, args ...any) error {
	return &MethodError{
		Type:        ErrInvalidResultReference.Type,
		Description: fmt.Sprintf(format, args...),
	}
}

// evalPointer applies an RFC 6901 JSON pointer to value, with RFC 8620
// section 3.7's addition: the token "*" maps the rest of the pointer
// over an array and flattens one level of the result.
func evalPointer(value json.RawMessage, pointer string) (json.RawMessage, error) {
	if pointer == "" {
		return value, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, referenceError("path %q is not a JSON pointer", pointer)
	}
	tokens := strings.Split(pointer[1:], "/")
	for i, token := range tokens {
		tokens[i] = pointerUnescaper.Replace(token)
	}
	return evalTokens(value, tokens)
}

// pointerUnescaper reverses RFC 6901 section 3's escaping. The order
// matters: "~01" is the escaped form of "~1" and must not decode to
// "/".
var pointerUnescaper = strings.NewReplacer("~1", "/", "~0", "~")

func evalTokens(value json.RawMessage, tokens []string) (json.RawMessage, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	if tokens[0] == "*" {
		return evalWildcard(value, tokens[1:])
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(value, &object); err == nil {
		member, ok := object[tokens[0]]
		if !ok {
			return nil, referenceError("no property %q", tokens[0])
		}
		return evalTokens(member, tokens[1:])
	}

	var array []json.RawMessage
	if err := json.Unmarshal(value, &array); err != nil {
		return nil, referenceError("token %q applies to neither an object nor an array", tokens[0])
	}
	index, ok := arrayIndex(tokens[0])
	if !ok || index >= len(array) {
		return nil, referenceError("array has no index %q", tokens[0])
	}
	return evalTokens(array[index], tokens[1:])
}

// evalWildcard applies the rest of the pointer to every item of an
// array. An item that answers with an array contributes its elements
// rather than itself, which is the flattening RFC 8620 section 3.7
// describes and the reason a Thread/get chain yields one id list.
func evalWildcard(value json.RawMessage, tokens []string) (json.RawMessage, error) {
	var array []json.RawMessage
	if err := json.Unmarshal(value, &array); err != nil {
		return nil, referenceError("wildcard applied to a value that is not an array")
	}

	flattened := []json.RawMessage{}
	for _, item := range array {
		result, err := evalTokens(item, tokens)
		if err != nil {
			return nil, err
		}
		var nested []json.RawMessage
		if err := json.Unmarshal(result, &nested); err == nil {
			flattened = append(flattened, nested...)
			continue
		}
		flattened = append(flattened, result)
	}
	return json.Marshal(flattened)
}

// arrayIndex reports the array index a reference token names. RFC 6901
// section 4 admits only digits with no leading zero, so "01" is a
// member name and never an index, and "-" points one past the end,
// where there is nothing to evaluate.
func arrayIndex(token string) (int, bool) {
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return 0, false
	}
	index := 0
	for _, r := range token {
		if r < '0' || r > '9' {
			return 0, false
		}
		index = index*10 + int(r-'0')
	}
	return index, true
}
