package jmap

import (
	"encoding/json"
	"slices"
	"strconv"
	"sync"
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
			// RFC 8620 section 5.5 has a server answer unsupportedFilter
			// for a filter condition whose capability is missing from
			// "using", so a call that leans on RFC 9219 and stays quiet
			// about it fails outright rather than matching every
			// message: naming the capability is what lets it run.
			name:    "a query filtering on S/MIME asks for the capability",
			methods: []Method{&EmailQuery{Filter: &EmailFilterCondition{HasVerifiedSMIME: new(true)}}},
			want:    []URI{CoreURI, MailURI, SMIMEVerifyURI},
		},
		{
			name: "a filter nesting the S/MIME condition asks for it too",
			methods: []Method{&EmailQuery{Filter: &FilterOperator{
				Operator: OperatorAND,
				Conditions: []Filter{
					&EmailFilterCondition{InMailbox: "MA"},
					&FilterOperator{Operator: OperatorNOT, Conditions: []Filter{
						&EmailFilterCondition{HasSMIME: new(false)},
					}},
				},
			}}},
			want: []URI{CoreURI, MailURI, SMIMEVerifyURI},
		},
		{
			name:    "a queryChanges repeating that filter asks for it",
			methods: []Method{&EmailQueryChanges{Filter: &EmailFilterCondition{HasVerifiedSMIMEAtDelivery: new(true)}}},
			want:    []URI{CoreURI, MailURI, SMIMEVerifyURI},
		},
		{
			name:    "a get asking for an S/MIME property asks for the capability",
			methods: []Method{&EmailGet{Properties: []string{"id", "smimeStatus"}}},
			want:    []URI{CoreURI, MailURI, SMIMEVerifyURI},
		},
		{
			// The other direction, and the one with more at stake:
			// section 3.3 has a server reject the whole request with
			// unknownCapability for a URI it does not advertise, and
			// Stalwart advertises no S/MIME.
			name: "a query with no S/MIME condition does not ask for it",
			methods: []Method{&EmailQuery{Filter: &FilterOperator{
				Operator:   OperatorAND,
				Conditions: []Filter{&EmailFilterCondition{InMailbox: "MA"}, nil},
			}}},
			want: []URI{CoreURI, MailURI},
		},
		{
			name:    "a get asking for ordinary properties does not ask for it",
			methods: []Method{&EmailGet{Properties: []string{"id", "subject", "keywords"}}},
			want:    []URI{CoreURI, MailURI},
		},
		{
			name:    "a query holding a typed nil filter does not ask for it",
			methods: []Method{&EmailQuery{Filter: (*EmailFilterCondition)(nil)}},
			want:    []URI{CoreURI, MailURI},
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

// TestRequestMarshalRecomputesUsingForALateMutation pins that Invoke's
// merge is not the only chance a call gets to declare a capability.
// Requires() reads a method's current fields, and nothing stops a
// caller from mutating one after Invoke returns; a filter added that
// way needs a capability Invoke had no chance to see. A caller
// building the filter after the call, the shape a request builder
// naturally falls into, is exactly the case withSMIME exists to
// cover.
func TestRequestMarshalRecomputesUsingForALateMutation(t *testing.T) {
	q := &EmailQuery{Account: "A13824"}
	req := &Request{}
	req.Invoke(q)
	q.Filter = &EmailFilterCondition{HasVerifiedSMIME: new(true)}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire struct {
		Using []URI `json:"using"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !slices.Contains(wire.Using, SMIMEVerifyURI) {
		t.Fatalf(`"using" = %v, want it to carry %q for a filter set after Invoke`, wire.Using, SMIMEVerifyURI)
	}
}

// TestRequestMarshalDoesNotWriteBehindTheCallersUsing pins that the
// late-mutation fold above never appends into the array behind the
// caller's own Using. MarshalJSON has a Request by value, but Using is
// a slice header, and a struct copy does not copy what the header
// points at: a fold that appends within spare capacity would write
// into memory the caller still holds. Using is built with make here,
// rather than left to grow through append, because append's own
// growth pattern happens to leave exactly this spare slot on the path
// TestRequestMarshalRecomputesUsingForALateMutation takes, and a test
// that depends on that coincidence proves nothing about the general
// case.
func TestRequestMarshalDoesNotWriteBehindTheCallersUsing(t *testing.T) {
	using := make([]URI, 2, 3)
	using[0] = CoreURI
	using[1] = MailURI
	backing := using[:cap(using)]
	spare := len(backing) - 1

	q := &EmailQuery{Account: "A13824", Filter: &EmailFilterCondition{HasVerifiedSMIME: new(true)}}
	req := &Request{Using: using, MethodCalls: []*Invocation{{Name: q.Name(), Args: q, CallID: "0"}}}

	if _, err := json.Marshal(req); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if backing[spare] != "" {
		t.Errorf("marshal wrote into the caller's spare capacity: backing[%d] = %q, want the zero value", spare, backing[spare])
	}
	if len(using) != 2 {
		t.Errorf("marshal grew the caller's own slice header: len(using) = %d, want 2", len(using))
	}
}

// TestRequestMarshalIsRaceFreeAcrossGoroutines pins the same guarantee
// under concurrency, where it matters: a Client shared across
// goroutines marshals the same Request from more than one at once.
// This test only fails under -race; make check does not run with it,
// so it is not this package's only evidence for the guarantee, but it
// is the one that catches a regression the sequential test above
// cannot.
func TestRequestMarshalIsRaceFreeAcrossGoroutines(t *testing.T) {
	q := &EmailQuery{Account: "A13824"}
	req := &Request{}
	req.Invoke(&MailboxGet{Account: "A13824"})
	req.Invoke(&EmailSubmissionSet{})
	req.Invoke(q)
	q.Filter = &EmailFilterCondition{HasVerifiedSMIME: new(true)}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := json.Marshal(req); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
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
