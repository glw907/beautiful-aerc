package jmap

import (
	"encoding/json"
	"testing"
)

// methodCase drives one concrete method type end to end: through
// [Request.Invoke], to the bytes on the wire, and back through the
// registry that decodes its response.
type methodCase struct {
	name     string
	method   Method
	wantUsed string
	wantArgs string
}

// methodCases covers every method type the package ships. go-jmap
// defined roughly thirty method types and exercised exactly one,
// Core/echo; this generalises that template to all of them.
func methodCases() []methodCase {
	const (
		core       = `"urn:ietf:params:jmap:core"`
		mail       = core + `,"urn:ietf:params:jmap:mail"`
		submission = core + `,"urn:ietf:params:jmap:submission"`
		bothMail   = core + `,"urn:ietf:params:jmap:submission","urn:ietf:params:jmap:mail"`
	)

	return []methodCase{
		{
			name:     "Core/echo",
			method:   Echo{"hello": true, "high": 5},
			wantUsed: core,
			wantArgs: `{"hello":true,"high":5}`,
		},
		{
			name:     "Mailbox/get",
			method:   &MailboxGet{Account: "A13824", IDs: []ID{"MA"}},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","ids":["MA"]}`,
		},
		{
			name:     "Mailbox/changes",
			method:   &MailboxChanges{Account: "A13824", SinceState: "78540", MaxChanges: 50},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","sinceState":"78540","maxChanges":50}`,
		},
		{
			name: "Mailbox/query",
			method: &MailboxQuery{
				Account: "A13824",
				Filter:  &MailboxFilterCondition{Role: RoleSent},
				Sort:    []*Comparator{{Property: "name"}},
				Limit:   10,
			},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","filter":{"role":"sent"},"sort":[{"property":"name"}],"limit":10}`,
		},
		{
			name: "Mailbox/set",
			method: &MailboxSet{
				Account: "A13824",
				Update:  map[ID]Patch{"MA": {"name": "Receipts"}},
			},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","update":{"MA":{"name":"Receipts"}}}`,
		},
		{
			name: "Email/get",
			method: &EmailGet{
				Account:             "A13824",
				Properties:          []string{"subject"},
				FetchTextBodyValues: true,
				ReferenceIDs: &ResultReference{
					ResultOf: "0",
					Name:     "Email/query",
					Path:     "/ids",
				},
			},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","properties":["subject"],"fetchTextBodyValues":true,` +
				`"#ids":{"resultOf":"0","name":"Email/query","path":"/ids"}}`,
		},
		{
			name:     "Email/changes",
			method:   &EmailChanges{Account: "A13824", SinceState: "78540"},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","sinceState":"78540"}`,
		},
		{
			name: "Email/query",
			method: &EmailQuery{
				Account:         "A13824",
				Filter:          &EmailFilterCondition{InMailbox: "MA"},
				Sort:            []*Comparator{{Property: "receivedAt", IsAscending: new(false)}},
				CollapseThreads: true,
				Limit:           10,
			},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","filter":{"inMailbox":"MA"},` +
				`"sort":[{"property":"receivedAt","isAscending":false}],"limit":10,"collapseThreads":true}`,
		},
		{
			name: "Email/set",
			method: &EmailSet{
				Account: "A13824",
				Update:  map[ID]Patch{"M1": {Pointer("keywords", "$seen"): true}},
			},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","update":{"M1":{"keywords/$seen":true}}}`,
		},
		{
			name: "Email/import",
			method: &EmailImport{
				Account: "A13824",
				Emails: map[ID]*EmailImportItem{
					"k1": {BlobID: "G1", MailboxIDs: map[ID]bool{"MA": true}},
				},
			},
			wantUsed: mail,
			wantArgs: `{"accountId":"A13824","emails":{"k1":{"blobId":"G1","mailboxIds":{"MA":true}}}}`,
		},
		{
			name:     "Identity/get",
			method:   &IdentityGet{Account: "A13824"},
			wantUsed: submission,
			wantArgs: `{"accountId":"A13824"}`,
		},
		{
			name: "EmailSubmission/set",
			method: &EmailSubmissionSet{
				Account: "A13824",
				Create: map[ID]*EmailSubmission{
					"k1490": {IdentityID: "I1", EmailID: "#k1"},
				},
				OnSuccessUpdateEmail: map[ID]Patch{
					"#k1490": {Pointer("keywords", "$draft"): nil},
				},
			},
			wantUsed: bothMail,
			wantArgs: `{"accountId":"A13824","create":{"k1490":{"identityId":"I1","emailId":"#k1"}},` +
				`"onSuccessUpdateEmail":{"#k1490":{"keywords/$draft":null}}}`,
		},
		{
			name: "EmailSubmission/set with onSuccessDestroyEmail",
			method: &EmailSubmissionSet{
				Account: "A13824",
				Create: map[ID]*EmailSubmission{
					"k1490": {IdentityID: "I1", EmailID: "#k1"},
				},
				OnSuccessDestroyEmail: []ID{"#k1490"},
			},
			wantUsed: bothMail,
			wantArgs: `{"accountId":"A13824","create":{"k1490":{"identityId":"I1","emailId":"#k1"}},` +
				`"onSuccessDestroyEmail":["#k1490"]}`,
		},
	}
}

// TestMethodInvoke covers JT-36 and JT-39 for every method type: the
// three-tuple shape, the arguments the type marshals to, and the
// capability URIs Invoke merges into "using".
//
// The "manual" subtest is the direct-versus-manual agreement pattern
// carried from go-jmap's mail/mailbox/changes_test.go. It pins
// Invoke's convenience to the wire format, which is the invariant a
// library other projects depend on must not drift on.
func TestMethodInvoke(t *testing.T) {
	for _, c := range methodCases() {
		t.Run(c.name, func(t *testing.T) {
			var req Request
			callID := req.Invoke(c.method)
			if callID != "0" {
				t.Errorf("Invoke returned call id %q, want %q", callID, "0")
			}

			want := `{"using":[` + c.wantUsed + `],"methodCalls":[["` + c.method.Name() +
				`",` + c.wantArgs + `,"0"]]}`
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != want {
				t.Errorf("Marshal =\n%s\nwant\n%s", data, want)
			}

			t.Run("manual", func(t *testing.T) {
				manual := Request{
					Using: req.Using,
					MethodCalls: []*Invocation{
						{Name: c.method.Name(), Args: c.method, CallID: "0"},
					},
				}
				manualData, err := json.Marshal(manual)
				if err != nil {
					t.Fatalf("Marshal: %v", err)
				}
				if string(manualData) != string(data) {
					t.Errorf("hand-built request =\n%s\nInvoke built\n%s", manualData, data)
				}
			})
		})
	}
}

// TestEveryMethodHasARegisteredResponse covers JT-37 from the other
// side: a method the package can send but cannot decode the answer to
// would fail only against a live server.
func TestEveryMethodHasARegisteredResponse(t *testing.T) {
	for _, c := range methodCases() {
		if _, ok := methodResponses[c.method.Name()]; !ok {
			t.Errorf("method %q has no registered response type", c.method.Name())
		}
	}
}

// TestEveryRegisteredResponseHasAMethod is the reverse check. The
// "error" pseudo-method is the one registered name with no method
// type, because the server sends it in place of any method's answer.
func TestEveryRegisteredResponseHasAMethod(t *testing.T) {
	sendable := make(map[string]bool, len(methodCases()))
	for _, c := range methodCases() {
		sendable[c.method.Name()] = true
	}
	sendable["error"] = true

	for name := range methodResponses {
		if !sendable[name] {
			t.Errorf("response type registered for %q, which no method type sends", name)
		}
	}
}
