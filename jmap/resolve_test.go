package jmap

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// decodeInvocations reads a methodResponses array as name, call id,
// and raw arguments, skipping the registry dispatch [Response]'s own
// decode performs. RFC 8620 section 3.7's worked examples name Foo and
// Thread, neither of which this package models, and the resolver reads
// only the raw arguments, so the fixtures stay verbatim.
func decodeInvocations(t *testing.T, name string) *Response {
	t.Helper()
	var tuples []json.RawMessage
	if err := json.Unmarshal(readFixture(t, name), &tuples); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	resp := &Response{}
	for _, tuple := range tuples {
		var parts []json.RawMessage
		if err := json.Unmarshal(tuple, &parts); err != nil {
			t.Fatalf("decode %s invocation: %v", name, err)
		}
		inv := &Invocation{Raw: parts[1]}
		if err := json.Unmarshal(parts[0], &inv.Name); err != nil {
			t.Fatalf("decode %s name: %v", name, err)
		}
		if err := json.Unmarshal(parts[2], &inv.CallID); err != nil {
			t.Fatalf("decode %s call id: %v", name, err)
		}
		resp.MethodResponses = append(resp.MethodResponses, inv)
	}
	return resp
}

func resolveIDs(t *testing.T, resp *Response, ref ResultReference) []ID {
	t.Helper()
	value, err := resp.Resolve(ref)
	if err != nil {
		t.Fatalf("Resolve(%+v): %v", ref, err)
	}
	var ids []ID
	if err := json.Unmarshal(value, &ids); err != nil {
		t.Fatalf("resolved value %s is not an id array: %v", value, err)
	}
	return ids
}

// TestResolveSimpleChain covers JT-05 against RFC 8620 section 3.7's
// one-hop Foo/changes to Foo/get example.
func TestResolveSimpleChain(t *testing.T) {
	resp := decodeInvocations(t, "rfc8620-3.7-backref-simple.json")

	ids := resolveIDs(t, resp, ResultReference{
		ResultOf: "t0",
		Name:     "Foo/changes",
		Path:     "/created",
	})
	want := []ID{"f1", "f4"}
	if !slices.Equal(ids, want) {
		t.Errorf("resolved /created to %v, want %v", ids, want)
	}
}

// TestResolveWildcardChain covers JT-05 against RFC 8620 section 3.7's
// four-call chain, whose final hop flattens an array of arrays.
func TestResolveWildcardChain(t *testing.T) {
	resp := decodeInvocations(t, "rfc8620-3.7-backref-wildcard.json")

	cases := []struct {
		name string
		ref  ResultReference
		want []ID
	}{
		{
			name: "query ids",
			ref:  ResultReference{ResultOf: "t0", Name: "Email/query", Path: "/ids"},
			want: []ID{
				"msg1023", "msg223", "msg110", "msg93", "msg91",
				"msg38", "msg36", "msg33", "msg11", "msg1",
			},
		},
		{
			name: "thread id per message",
			ref:  ResultReference{ResultOf: "t1", Name: "Email/get", Path: "/list/*/threadId"},
			want: []ID{"trd194", "trd114"},
		},
		{
			name: "flattened email ids per thread",
			ref:  ResultReference{ResultOf: "t2", Name: "Thread/get", Path: "/list/*/emailIds"},
			want: []ID{"msg1020", "msg1021", "msg1023", "msg201", "msg223"},
		},
		{
			name: "index token selects one item",
			ref:  ResultReference{ResultOf: "t2", Name: "Thread/get", Path: "/list/1/emailIds"},
			want: []ID{"msg201", "msg223"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if ids := resolveIDs(t, resp, c.ref); !slices.Equal(ids, c.want) {
				t.Errorf("resolved %s to %v, want %v", c.ref.Path, ids, c.want)
			}
		})
	}
}

// TestResolveReadsRawArguments covers JT-05's reason for
// Invocation.Raw. Every response property in this package carries
// omitempty, so an empty array the server sent is gone the moment the
// resolver reads a re-marshalled Args instead of the bytes that
// arrived, and "no changes" becomes "that property does not exist".
func TestResolveReadsRawArguments(t *testing.T) {
	const body = `{
	  "methodResponses": [
	    ["Mailbox/changes", {
	      "accountId": "A1",
	      "oldState": "abcdef",
	      "newState": "123456",
	      "created": ["m1"],
	      "updated": [],
	      "destroyed": []
	    }, "0"]
	  ],
	  "sessionState": "s1"
	}`

	var resp Response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	remarshalled, err := json.Marshal(argsOf[*MailboxChangesResponse](t, &resp, "0"))
	if err != nil {
		t.Fatalf("marshal decoded args: %v", err)
	}
	if strings.Contains(string(remarshalled), `"updated"`) {
		t.Fatalf("re-marshalled args %s still carry updated; the fixture no longer proves omitempty erases it", remarshalled)
	}

	value, err := resp.Resolve(ResultReference{
		ResultOf: "0",
		Name:     "Mailbox/changes",
		Path:     "/updated",
	})
	if err != nil {
		t.Fatalf("Resolve /updated: %v", err)
	}
	if string(value) != "[]" {
		t.Errorf("resolved /updated to %s, want []", value)
	}
}

// TestResolveFailsLoudly covers JT-06. RFC 8620 section 3.7 rejects
// the whole method with invalidResultReference when a reference does
// not resolve, so every row here must produce that error and no value:
// an empty id list with a nil error is how a destroy or a move ends up
// operating on the wrong set.
func TestResolveFailsLoudly(t *testing.T) {
	resp := decodeInvocations(t, "rfc8620-3.7-backref-simple.json")
	errored := &Response{MethodResponses: []*Invocation{{
		Name:   "error",
		CallID: "t0",
		Raw:    json.RawMessage(`{"type":"cannotCalculateChanges"}`),
	}}}

	cases := []struct {
		name string
		resp *Response
		ref  ResultReference
	}{
		{
			name: "no call carries that id",
			resp: resp,
			ref:  ResultReference{ResultOf: "t9", Name: "Foo/changes", Path: "/created"},
		},
		{
			name: "method name does not match",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/get", Path: "/created"},
		},
		{
			name: "path names no property",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/notAProperty"},
		},
		{
			name: "path descends through a scalar",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/newState/nested"},
		},
		{
			name: "wildcard over a non-array",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/newState/*"},
		},
		{
			name: "index past the end",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/created/2"},
		},
		{
			name: "index with a leading zero",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/created/01"},
		},
		// The three rows below are one bug class, not three
		// instances. Accumulating an index digit by digit wraps a
		// token wider than an int: 2^64+1 wraps to 1, which is a real
		// element of this two-id array, so the reference resolved to
		// the wrong id with no error at all. Anything past 2^63 wraps
		// negative, which the "index >= len(array)" bound passes, so
		// the walk indexed out of range and panicked. A panic is the
		// one failure a package forbidden from logging cannot surface.
		{
			name: "index wider than a uint64",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/created/18446744073709551617"},
		},
		{
			name: "index one past the largest int",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/created/9223372036854775808"},
		},
		{
			name: "index of thirty-one digits",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/created/9999999999999999999999999999999"},
		},
		{
			name: "path is not a JSON pointer",
			resp: resp,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "created"},
		},
		{
			name: "the referenced call failed",
			resp: errored,
			ref:  ResultReference{ResultOf: "t0", Name: "Foo/changes", Path: "/created"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value, err := c.resp.Resolve(c.ref)
			if err == nil {
				t.Fatalf("Resolve(%+v) = %s, want an error", c.ref, value)
			}
			if value != nil {
				t.Errorf("Resolve returned %s alongside its error, want nil", value)
			}
			if !errors.Is(err, ErrInvalidResultReference) {
				t.Errorf("Resolve error = %v, want invalidResultReference", err)
			}
		})
	}
}

// TestResolveErrorNamesTheWholePath pins what a failure says. A
// wildcard applies the same token to every item of an array, so a
// message naming only the token it stopped on leaves a reader of a
// four-hop chain no way to tell which hop came apart.
func TestResolveErrorNamesTheWholePath(t *testing.T) {
	resp := decodeInvocations(t, "rfc8620-3.7-backref-wildcard.json")

	paths := []string{
		"/list/*/notAProperty",
		"/list/0/id/deeper",
		"/list/0/id/*",
		"/list/9/id",
		"/list/01/id",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			_, err := resp.Resolve(ResultReference{ResultOf: "t1", Name: "Email/get", Path: path})
			if err == nil {
				t.Fatalf("Resolve(%q) succeeded, want an error", path)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("Resolve error %q does not name the path %q", err, path)
			}
		})
	}
}

// TestResolveUnescapesPointerTokens covers RFC 6901's escaping, which
// RFC 8620 section 3.7 adopts whole: a property name carrying a
// slash reaches the resolver as "~1" and must not be read as a
// separator.
func TestResolveUnescapesPointerTokens(t *testing.T) {
	resp := &Response{MethodResponses: []*Invocation{{
		Name:   "Foo/get",
		CallID: "t0",
		Raw:    json.RawMessage(`{"a/b":{"c~d":["x"]}}`),
	}}}

	ids := resolveIDs(t, resp, ResultReference{
		ResultOf: "t0",
		Name:     "Foo/get",
		Path:     "/a~1b/c~0d",
	})
	if !slices.Equal(ids, []ID{"x"}) {
		t.Errorf("resolved escaped pointer to %v, want [x]", ids)
	}
}

// TestResolveEmptyPathReturnsTheArguments covers RFC 6901's whole-
// document pointer, which is the empty string.
func TestResolveEmptyPathReturnsTheArguments(t *testing.T) {
	resp := decodeInvocations(t, "rfc8620-3.7-backref-simple.json")

	value, err := resp.Resolve(ResultReference{ResultOf: "t0", Name: "Foo/changes"})
	if err != nil {
		t.Fatalf("Resolve empty path: %v", err)
	}
	var args struct {
		Account ID `json:"accountId"`
	}
	if err := json.Unmarshal(value, &args); err != nil {
		t.Fatalf("resolved value is not the arguments object: %v", err)
	}
	if args.Account != "A1" {
		t.Errorf("resolved accountId = %q, want A1", args.Account)
	}
}
