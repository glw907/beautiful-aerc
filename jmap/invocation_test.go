package jmap

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestInvocationUnmarshal covers JT-36's decode half: the tuple
// resolves the method name through the registry and hands back a
// pointer to that method's response type, not a bare map.
func TestInvocationUnmarshal(t *testing.T) {
	raw := []byte(`["Mailbox/get",{"accountId":"A13824","state":"78540","list":[{"id":"MA","name":"Inbox"}]},"c1"]`)

	var inv Invocation
	if err := json.Unmarshal(raw, &inv); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if inv.Name != "Mailbox/get" {
		t.Errorf("Name = %q, want %q", inv.Name, "Mailbox/get")
	}
	if inv.CallID != "c1" {
		t.Errorf("CallID = %q, want %q", inv.CallID, "c1")
	}

	got, ok := inv.Args.(*MailboxGetResponse)
	if !ok {
		t.Fatalf("Args is %T, want *MailboxGetResponse", inv.Args)
	}
	if got.State != "78540" {
		t.Errorf("State = %q, want %q", got.State, "78540")
	}
	if len(got.List) != 1 || got.List[0].Name != "Inbox" {
		t.Errorf("List = %+v, want one mailbox named Inbox", got.List)
	}
}

// TestInvocationUnmarshalRejectsUnknownMethod covers JT-37. go-jmap
// had this branch and never tested it; the failure it guards is a
// response silently decoding to nothing.
func TestInvocationUnmarshalRejectsUnknownMethod(t *testing.T) {
	var inv Invocation
	err := json.Unmarshal([]byte(`["Vendor/frobnicate",{"ok":true},"c1"]`), &inv)
	if err == nil {
		t.Fatal("Unmarshal of an unregistered method returned no error")
	}
	if !strings.Contains(err.Error(), "Vendor/frobnicate") {
		t.Errorf("error = %v, want it to name the method", err)
	}
	if inv.Args != nil {
		t.Errorf("Args = %#v, want nil after a failed decode", inv.Args)
	}
}

// TestInvocationUnmarshalRejectsWrongArity proves a tuple of the
// wrong length is an error rather than a partly filled invocation.
func TestInvocationUnmarshalRejectsWrongArity(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"two elements", `["Core/echo",{}]`},
		{"four elements", `["Core/echo",{},"c1","extra"]`},
		{"empty", `[]`},
		{"not an array", `{"name":"Core/echo"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var inv Invocation
			if err := json.Unmarshal([]byte(c.raw), &inv); err == nil {
				t.Errorf("Unmarshal(%s) returned no error", c.raw)
			}
		})
	}
}

// TestInvocationKeepsRawArguments proves the arguments object is kept
// as it arrived. Every response property in this package carries
// omitempty, so re-marshaling Args drops one the server sent as an
// empty array, and an RFC 8620 section 3.7 pointer resolves against
// what arrived rather than against what survives a round trip.
func TestInvocationKeepsRawArguments(t *testing.T) {
	const raw = `["Mailbox/changes",{"accountId":"A1","newState":"s1","created":[],` +
		`"updated":[],"destroyed":[]},"c1"]`

	var inv Invocation
	if err := json.Unmarshal([]byte(raw), &inv); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := `{"accountId":"A1","newState":"s1","created":[],"updated":[],"destroyed":[]}`
	if string(inv.Raw) != want {
		t.Errorf("Raw = %s, want %s", inv.Raw, want)
	}

	// The decoded value cannot answer the same question: the three
	// empty arrays are gone from it, so a pointer into /created finds
	// nothing where the server sent an empty list.
	changes, ok := inv.Args.(*MailboxChangesResponse)
	if !ok {
		t.Fatalf("Args is %T, want *MailboxChangesResponse", inv.Args)
	}
	reMarshalled, err := json.Marshal(changes)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(reMarshalled), `"created"`) {
		t.Errorf("re-marshal = %s; the test's premise is stale, so Raw may no longer be needed",
			reMarshalled)
	}

	// A call the client built carries no raw form: nothing arrived.
	var req Request
	req.Invoke(&MailboxChanges{Account: "A1"})
	if req.MethodCalls[0].Raw != nil {
		t.Errorf("Raw = %s on an outgoing call, want nil", req.MethodCalls[0].Raw)
	}
}

// TestInvocationRefusesReferenceCollision covers JT-07. RFC 8620
// section 3.7 makes an arguments object holding both "foo" and "#foo"
// an invalidArguments error, so the server picks one of the two and
// the caller's other argument goes missing without a word.
func TestInvocationRefusesReferenceCollision(t *testing.T) {
	get := &EmailGet{
		Account: "A13824",
		IDs:     []ID{"M1"},
		ReferenceIDs: &ResultReference{
			ResultOf: "0",
			Name:     "Email/query",
			Path:     "/ids",
		},
	}

	data, err := json.Marshal(Invocation{Name: get.Name(), Args: get, CallID: "0"})
	if err == nil {
		t.Fatalf("Marshal returned %s, want an error", data)
	}
	if !strings.Contains(err.Error(), `"ids"`) {
		t.Errorf("error = %v, want it to name the colliding argument", err)
	}

	get.IDs = nil
	if _, err := json.Marshal(Invocation{Name: get.Name(), Args: get, CallID: "0"}); err != nil {
		t.Errorf("Marshal of the reference form alone: %v", err)
	}
}

// TestInvocationAllowsAReferenceWithoutItsNormalForm proves the
// collision check does not fire on a reference whose normal form is
// absent, which is every legitimate chained call.
func TestInvocationAllowsAReferenceWithoutItsNormalForm(t *testing.T) {
	var req Request
	req.Invoke(&EmailQuery{Account: "A13824"})
	req.Invoke(&EmailGet{
		Account: "A13824",
		ReferenceIDs: &ResultReference{
			ResultOf: "0",
			Name:     "Email/query",
			Path:     "/ids",
		},
	})

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"#ids":{"resultOf":"0","name":"Email/query","path":"/ids"}`) {
		t.Errorf("Marshal = %s, want it to carry the back-reference", data)
	}
}
