package jmap

import (
	"encoding/json"
	"strconv"
	"testing"
)

// TestRequestInvokeAssignsSequentialCallIDs covers JT-38. go-jmap
// formatted the index as hex and never invoked twice in a test, so
// its eleventh call was silently called "a".
func TestRequestInvokeAssignsSequentialCallIDs(t *testing.T) {
	var req Request
	for i := range 12 {
		got := req.Invoke(&EmailChanges{Account: "A13824"})
		if want := strconv.Itoa(i); got != want {
			t.Fatalf("call %d got id %q, want %q", i, got, want)
		}
	}
	if len(req.MethodCalls) != 12 {
		t.Fatalf("MethodCalls has %d entries, want 12", len(req.MethodCalls))
	}
	if got := req.MethodCalls[10].CallID; got != "10" {
		t.Errorf("eleventh call id = %q, want %q", got, "10")
	}
}

// TestRequestBackReferenceNamesTheRightCall covers JT-38's second
// half: the id Invoke returned is the id a later call references.
func TestRequestBackReferenceNamesTheRightCall(t *testing.T) {
	var req Request
	req.Invoke(&MailboxGet{Account: "A13824"})
	queryID := req.Invoke(&EmailQuery{Account: "A13824"})
	req.Invoke(&EmailGet{
		Account:      "A13824",
		ReferenceIDs: &ResultReference{ResultOf: queryID, Name: "Email/query", Path: "/ids"},
	})

	get, ok := req.MethodCalls[2].Args.(*EmailGet)
	if !ok {
		t.Fatalf("third call carries %T, want *EmailGet", req.MethodCalls[2].Args)
	}
	reference := get.ReferenceIDs
	if reference.ResultOf != "1" {
		t.Fatalf("reference names call %q, want %q", reference.ResultOf, "1")
	}
	if got := req.MethodCalls[1].Name; got != reference.Name {
		t.Errorf("call %q is %q, but the reference names %q", reference.ResultOf, got, reference.Name)
	}
}

// TestRequestUsing covers JT-39. The core URI is always present, a
// repeated requirement appears once, and the order is stable so one
// request marshals to the same bytes twice.
func TestRequestUsing(t *testing.T) {
	cases := []struct {
		name    string
		seed    []URI
		methods []Method
		want    []URI
	}{
		{
			name:    "no method",
			methods: nil,
			want:    nil,
		},
		{
			name:    "a method requiring nothing",
			methods: []Method{Echo{}},
			want:    []URI{CoreURI},
		},
		{
			name:    "one mail method",
			methods: []Method{&MailboxGet{}},
			want:    []URI{CoreURI, MailURI},
		},
		{
			name:    "two mail methods deduplicate",
			methods: []Method{&MailboxGet{}, &EmailGet{}},
			want:    []URI{CoreURI, MailURI},
		},
		{
			name:    "mail then submission",
			methods: []Method{&EmailSet{}, &IdentityGet{}},
			want:    []URI{CoreURI, MailURI, SubmissionURI},
		},
		{
			name:    "submission then mail keeps first-seen order",
			methods: []Method{&IdentityGet{}, &EmailSet{}},
			want:    []URI{CoreURI, SubmissionURI, MailURI},
		},
		{
			name:    "a method requiring two",
			methods: []Method{&EmailSubmissionSet{}},
			want:    []URI{CoreURI, SubmissionURI, MailURI},
		},
		{
			name:    "a seeded Using still gets the core capability",
			seed:    []URI{"urn:vendor:thing"},
			methods: []Method{&MailboxGet{}},
			want:    []URI{CoreURI, "urn:vendor:thing", MailURI},
		},
		{
			name:    "a seeded Using already holding core is not duplicated",
			seed:    []URI{CoreURI, "urn:vendor:thing"},
			methods: []Method{&MailboxGet{}},
			want:    []URI{CoreURI, "urn:vendor:thing", MailURI},
		},
		{
			name:    "a seeded Using with core last keeps its position",
			seed:    []URI{"urn:vendor:thing", CoreURI},
			methods: []Method{&MailboxGet{}},
			want:    []URI{"urn:vendor:thing", CoreURI, MailURI},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := Request{Using: c.seed}
			for _, m := range c.methods {
				req.Invoke(m)
			}
			if len(req.Using) != len(c.want) {
				t.Fatalf("Using = %v, want %v", req.Using, c.want)
			}
			for i, uri := range c.want {
				if req.Using[i] != uri {
					t.Fatalf("Using = %v, want %v", req.Using, c.want)
				}
			}
		})
	}
}

// TestRequestShape covers JT-40 for the request half. RFC 8620
// section 3.3 makes "using" and "methodCalls" mandatory and
// "createdIds" optional, so a nil slice must not reach the wire as
// null and an absent creation map must not reach it at all.
func TestRequestShape(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "empty request emits both mandatory arrays",
			req:  Request{},
			want: `{"using":[],"methodCalls":[]}`,
		},
		{
			name: "createdIds omitted when empty",
			req: Request{
				Using:       []URI{CoreURI},
				MethodCalls: []*Invocation{{Name: "Core/echo", Args: Echo{}, CallID: "0"}},
			},
			want: `{"using":["urn:ietf:params:jmap:core"],"methodCalls":[["Core/echo",{},"0"]]}`,
		},
		{
			name: "createdIds carried when seeded",
			req: Request{
				Using:       []URI{CoreURI},
				MethodCalls: []*Invocation{{Name: "Core/echo", Args: Echo{}, CallID: "0"}},
				CreatedIDs:  map[ID]ID{"k1": "M1"},
			},
			want: `{"using":["urn:ietf:params:jmap:core"],"methodCalls":[["Core/echo",{},"0"]],` +
				`"createdIds":{"k1":"M1"}}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.req)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.want {
				t.Errorf("Marshal = %s, want %s", data, c.want)
			}
		})
	}
}
