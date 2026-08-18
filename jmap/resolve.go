package jmap

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	return evalTokens(pointer, value, tokens)
}

// pointerUnescaper reverses RFC 6901 section 3's escaping: "~01" is
// the escaped form of "~1" and must not decode to "/". Do not split
// this into two sequential ReplaceAll passes: doing the "~0" pass
// before the "~1" pass turns "~01" into "~1" and then into "/",
// which is wrong.
var pointerUnescaper = strings.NewReplacer("~1", "/", "~0", "~")

// evalTokens carries pointer through the walk so a failure names the
// whole path and not just the token it stopped on. A wildcard applies
// the same token to every item, so the token alone says nothing about
// where a four-hop chain came apart.
func evalTokens(pointer string, value json.RawMessage, tokens []string) (json.RawMessage, error) {
	if len(tokens) == 0 {
		return value, nil
	}
	if tokens[0] == "*" {
		return evalWildcard(pointer, value, tokens[1:])
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(value, &object); err == nil {
		member, ok := object[tokens[0]]
		if !ok {
			return nil, referenceError("path %q: no property %q", pointer, tokens[0])
		}
		return evalTokens(pointer, member, tokens[1:])
	}

	var array []json.RawMessage
	if err := json.Unmarshal(value, &array); err != nil {
		return nil, referenceError("path %q: token %q applies to neither an object nor an array", pointer, tokens[0])
	}
	index, ok := arrayIndex(tokens[0])
	if !ok || index >= len(array) {
		return nil, referenceError("path %q: array has no index %q", pointer, tokens[0])
	}
	return evalTokens(pointer, array[index], tokens[1:])
}

// evalWildcard applies the rest of the pointer to every item of an
// array. An item that answers with an array contributes its elements
// rather than itself, which is the flattening RFC 8620 section 3.7
// describes and the reason a Thread/get chain yields one id list.
func evalWildcard(pointer string, value json.RawMessage, tokens []string) (json.RawMessage, error) {
	var array []json.RawMessage
	if err := json.Unmarshal(value, &array); err != nil {
		return nil, referenceError("path %q: wildcard applied to a value that is not an array", pointer)
	}

	flattened := []json.RawMessage{}
	for _, item := range array {
		result, err := evalTokens(pointer, item, tokens)
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
//
// The digit scan and strconv.Atoi are both load-bearing. The scan
// rejects the leading signs Atoi reads and section 4 does not admit,
// without which "+1" selects a real element and "-1" indexes out of
// range. Atoi rejects a run of digits too large for an int,
// which accumulating the value by hand instead wraps: a token of 2^64+1
// wraps to 1 and resolves to a real element of the array with no error,
// and a larger one wraps negative and indexes out of range.
func arrayIndex(token string) (int, bool) {
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return 0, false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	index, err := strconv.Atoi(token)
	return index, err == nil
}
