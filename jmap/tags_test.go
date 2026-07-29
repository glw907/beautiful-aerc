package jmap

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The two tables below exist for one reason: JT-19's failure class is
// "a mistyped struct tag that silently reads zero forever", and no
// semantic test observes most of a tag layer. Renaming a tag no case
// names leaves the whole suite green while poplar reads that property
// as absent against every server.
//
// One type, one case, one JSON string. A case's literal is checked
// only against the tags of the type it is named for, so a tag never
// counts as observed because a different type happens to carry that
// name. Five names in this package are already ambiguous: blobId,
// value, headers, name, and size.
//
// TestEveryJSONTagIsObserved holds the tables to the whole tag layer,
// so a type or a field added later fails rather than slipping in
// unwatched.

// tagCase pairs a Go type with JSON that spells every tag it carries.
// Request types are marshalled from a fully populated value and
// compared byte for byte. Response types round-trip a fully populated
// literal, which fails the same way: a renamed tag decodes as the
// zero value and, since every response property carries omitempty,
// vanishes from the re-marshal.
type tagCase struct {
	// name is the Go type the case stands for, and the key the
	// meta-test matches against. No two cases share one.
	name string

	value any
	json  string
}

// requestTagCases marshals a fully populated value of every type
// poplar sends.
func requestTagCases() []tagCase {
	received := Date(mustParseDate("2026-07-28T09:00:00Z"))
	before := Date(mustParseDate("2026-07-28T00:00:00Z"))
	after := Date(mustParseDate("2026-01-01T00:00:00Z"))

	return []tagCase{
		{
			name: "Request",
			value: Request{
				Using:       []URI{CoreURI},
				MethodCalls: []*Invocation{{Name: "Core/echo", Args: Echo{}, CallID: "0"}},
				CreatedIDs:  map[ID]ID{"k1": "M1"},
			},
			json: `{"using":["urn:ietf:params:jmap:core"],` +
				`"methodCalls":[["Core/echo",{},"0"]],"createdIds":{"k1":"M1"}}`,
		},
		{
			name:  "ResultReference",
			value: &ResultReference{ResultOf: "0", Name: "Email/query", Path: "/ids"},
			json:  `{"resultOf":"0","name":"Email/query","path":"/ids"}`,
		},
		{
			name: "Comparator",
			value: &Comparator{
				Property:    "name",
				IsAscending: new(false),
				Collation:   ASCIICasemap,
				Keyword:     "$flagged",
			},
			json: `{"property":"name","isAscending":false,` +
				`"collation":"i;ascii-casemap","keyword":"$flagged"}`,
		},
		{
			name: "FilterOperator",
			value: &FilterOperator{
				Operator:   OperatorAND,
				Conditions: []Filter{&EmailFilterCondition{InMailbox: "MA"}},
			},
			json: `{"operator":"AND","conditions":[{"inMailbox":"MA"}]}`,
		},
		{
			name: "MailboxFilterCondition",
			value: &MailboxFilterCondition{
				ParentID:     "MP",
				Name:         "Receipts",
				Role:         RoleInbox,
				HasAnyRole:   new(false),
				IsSubscribed: new(false),
			},
			json: `{"parentId":"MP","name":"Receipts","role":"inbox",` +
				`"hasAnyRole":false,"isSubscribed":false}`,
		},
		{
			// Every optional Boolean here carries false, which is the
			// state the pointer exists for. A true marshals identically
			// under bool and *bool, so it leaves the field type
			// unpinned.
			name: "EmailFilterCondition",
			value: &EmailFilterCondition{
				InMailbox:                  "MA",
				InMailboxOtherThan:         []ID{"MB"},
				Before:                     &before,
				After:                      &after,
				MinSize:                    10,
				MaxSize:                    20,
				AllInThreadHaveKeyword:     "$seen",
				SomeInThreadHaveKeyword:    "$flagged",
				NoneInThreadHaveKeyword:    "$draft",
				HasKeyword:                 "$answered",
				NotKeyword:                 "$junk",
				HasAttachment:              new(false),
				Text:                       "t",
				From:                       "f",
				To:                         "o",
				Cc:                         "c",
				Bcc:                        "b",
				Subject:                    "s",
				Body:                       "y",
				Header:                     []string{"X-A", "1"},
				HasSMIME:                   new(false),
				HasVerifiedSMIME:           new(false),
				HasVerifiedSMIMEAtDelivery: new(false),
			},
			json: `{"inMailbox":"MA","inMailboxOtherThan":["MB"],` +
				`"before":"2026-07-28T00:00:00Z","after":"2026-01-01T00:00:00Z",` +
				`"minSize":10,"maxSize":20,"allInThreadHaveKeyword":"$seen",` +
				`"someInThreadHaveKeyword":"$flagged","noneInThreadHaveKeyword":"$draft",` +
				`"hasKeyword":"$answered","notKeyword":"$junk","hasAttachment":false,` +
				`"text":"t","from":"f","to":"o","cc":"c","bcc":"b","subject":"s",` +
				`"body":"y","header":["X-A","1"],"hasSmime":false,` +
				`"hasVerifiedSmime":false,"hasVerifiedSmimeAtDelivery":false}`,
		},
		{
			name: "MailboxGet",
			value: &MailboxGet{
				Account:      "A1",
				IDs:          []ID{"MA"},
				Properties:   []string{"name"},
				ReferenceIDs: &ResultReference{ResultOf: "0", Name: "Mailbox/query", Path: "/ids"},
			},
			json: `{"accountId":"A1","ids":["MA"],"properties":["name"],` +
				`"#ids":{"resultOf":"0","name":"Mailbox/query","path":"/ids"}}`,
		},
		{
			name:  "MailboxChanges",
			value: &MailboxChanges{Account: "A1", SinceState: "s0", MaxChanges: 50},
			json:  `{"accountId":"A1","sinceState":"s0","maxChanges":50}`,
		},
		{
			name: "MailboxQuery",
			value: &MailboxQuery{
				Account:        "A1",
				Filter:         &MailboxFilterCondition{Name: "Receipts"},
				Sort:           []*Comparator{{Property: "name"}},
				Position:       5,
				Anchor:         "MA",
				AnchorOffset:   -2,
				Limit:          10,
				CalculateTotal: true,
				SortAsTree:     true,
				FilterAsTree:   true,
			},
			json: `{"accountId":"A1","filter":{"name":"Receipts"},` +
				`"sort":[{"property":"name"}],"position":5,"anchor":"MA",` +
				`"anchorOffset":-2,"limit":10,"calculateTotal":true,` +
				`"sortAsTree":true,"filterAsTree":true}`,
		},
		{
			name: "MailboxSet",
			value: &MailboxSet{
				Account:               "A1",
				IfInState:             "s0",
				Create:                map[ID]*Mailbox{"k1": {Name: "Receipts"}},
				Update:                map[ID]Patch{"MB": {"name": "Renamed"}},
				Destroy:               []ID{"MC"},
				OnDestroyRemoveEmails: true,
			},
			json: `{"accountId":"A1","ifInState":"s0","create":{"k1":{"name":"Receipts"}},` +
				`"update":{"MB":{"name":"Renamed"}},"destroy":["MC"],"onDestroyRemoveEmails":true}`,
		},
		{
			name: "EmailGet",
			value: &EmailGet{
				Account:             "A1",
				IDs:                 []ID{"M1"},
				Properties:          []string{"subject"},
				BodyProperties:      []string{"partId"},
				FetchTextBodyValues: true,
				FetchHTMLBodyValues: true,
				FetchAllBodyValues:  true,
				MaxBodyValueBytes:   4096,
				ReferenceIDs:        &ResultReference{ResultOf: "0", Name: "Email/query", Path: "/ids"},
			},
			json: `{"accountId":"A1","ids":["M1"],"properties":["subject"],` +
				`"bodyProperties":["partId"],"fetchTextBodyValues":true,` +
				`"fetchHTMLBodyValues":true,"fetchAllBodyValues":true,"maxBodyValueBytes":4096,` +
				`"#ids":{"resultOf":"0","name":"Email/query","path":"/ids"}}`,
		},
		{
			name:  "EmailChanges",
			value: &EmailChanges{Account: "A1", SinceState: "s0", MaxChanges: 50},
			json:  `{"accountId":"A1","sinceState":"s0","maxChanges":50}`,
		},
		{
			name: "EmailQuery",
			value: &EmailQuery{
				Account: "A1",
				Filter: &FilterOperator{
					Operator:   OperatorAND,
					Conditions: []Filter{&EmailFilterCondition{InMailbox: "MA"}},
				},
				Sort:            []*Comparator{{Property: "receivedAt"}},
				Position:        5,
				Anchor:          "M1",
				AnchorOffset:    -2,
				Limit:           10,
				CalculateTotal:  true,
				CollapseThreads: true,
			},
			json: `{"accountId":"A1","filter":{"operator":"AND",` +
				`"conditions":[{"inMailbox":"MA"}]},"sort":[{"property":"receivedAt"}],` +
				`"position":5,"anchor":"M1","anchorOffset":-2,"limit":10,` +
				`"calculateTotal":true,"collapseThreads":true}`,
		},
		{
			name: "EmailSet",
			value: &EmailSet{
				Account:   "A1",
				IfInState: "s0",
				Create:    map[ID]*Email{"k1": {Subject: "Lunch"}},
				Update:    map[ID]Patch{"M1": {Pointer("keywords", "$seen"): true}},
				Destroy:   []ID{"M2"},
			},
			json: `{"accountId":"A1","ifInState":"s0","create":{"k1":{"subject":"Lunch"}},` +
				`"update":{"M1":{"keywords/$seen":true}},"destroy":["M2"]}`,
		},
		{
			name: "EmailImportItem",
			value: &EmailImportItem{
				BlobID:     "G1",
				MailboxIDs: map[ID]bool{"MA": true},
				Keywords:   map[string]bool{"$seen": true},
				ReceivedAt: &received,
			},
			json: `{"blobId":"G1","mailboxIds":{"MA":true},"keywords":{"$seen":true},` +
				`"receivedAt":"2026-07-28T09:00:00Z"}`,
		},
		{
			name: "EmailImport",
			value: &EmailImport{
				Account:   "A1",
				IfInState: "s0",
				Emails:    map[ID]*EmailImportItem{"k1": {BlobID: "G1"}},
			},
			json: `{"accountId":"A1","ifInState":"s0","emails":{"k1":{"blobId":"G1"}}}`,
		},
		{
			name: "IdentityGet",
			value: &IdentityGet{
				Account:      "A1",
				IDs:          []ID{"I1"},
				Properties:   []string{"name"},
				ReferenceIDs: &ResultReference{ResultOf: "0", Name: "Identity/get", Path: "/ids"},
			},
			json: `{"accountId":"A1","ids":["I1"],"properties":["name"],` +
				`"#ids":{"resultOf":"0","name":"Identity/get","path":"/ids"}}`,
		},
		{
			name: "EmailSubmissionSet",
			value: &EmailSubmissionSet{
				Account:               "A1",
				IfInState:             "s0",
				Create:                map[ID]*EmailSubmission{"k1": {IdentityID: "I1", EmailID: "#m1"}},
				Update:                map[ID]Patch{"ES1": {"undoStatus": "canceled"}},
				Destroy:               []ID{"ES2"},
				OnSuccessUpdateEmail:  map[ID]Patch{"#k1": {Pointer("keywords", "$draft"): nil}},
				OnSuccessDestroyEmail: []ID{"#k1"},
			},
			json: `{"accountId":"A1","ifInState":"s0",` +
				`"create":{"k1":{"identityId":"I1","emailId":"#m1"}},` +
				`"update":{"ES1":{"undoStatus":"canceled"}},"destroy":["ES2"],` +
				`"onSuccessUpdateEmail":{"#k1":{"keywords/$draft":null}},` +
				`"onSuccessDestroyEmail":["#k1"]}`,
		},
	}
}

// responseTagCases round-trips a fully populated literal through every
// type a server sends.
func responseTagCases() []tagCase {
	return []tagCase{
		{
			name:  "Response",
			value: &Response{},
			json: `{"methodResponses":[["Core/echo",{},"0"]],"createdIds":{"k1":"M1"},` +
				`"sessionState":"s1"}`,
		},
		{
			name:  "Session",
			value: &Session{},
			json: `{"capabilities":{"urn:ietf:params:jmap:core":{}},` +
				`"accounts":{"A1":{"accountCapabilities":{},"name":"n",` +
				`"isPersonal":true,"isReadOnly":true}},` +
				`"primaryAccounts":{"urn:ietf:params:jmap:mail":"A1"},"username":"u",` +
				`"apiUrl":"a","downloadUrl":"d","uploadUrl":"up","eventSourceUrl":"e",` +
				`"state":"s"}`,
		},
		{
			name:  "Account",
			value: &Account{},
			json: `{"accountCapabilities":{"urn:ietf:params:jmap:mail":{}},"name":"n",` +
				`"isPersonal":true,"isReadOnly":true}`,
		},
		{
			name:  "Core",
			value: &Core{},
			json: `{"maxSizeUpload":1,"maxConcurrentUpload":2,"maxSizeRequest":3,` +
				`"maxConcurrentRequests":4,"maxCallsInRequest":5,"maxObjectsInGet":6,` +
				`"maxObjectsInSet":7,"collationAlgorithms":["i;ascii-numeric"]}`,
		},
		{
			name:  "Mail",
			value: &Mail{},
			json: `{"maxMailboxesPerEmail":1,"maxMailboxDepth":2,"maxSizeMailboxName":3,` +
				`"maxSizeAttachmentsPerEmail":4,"emailQuerySortOptions":["receivedAt"],` +
				`"mayCreateTopLevelMailbox":true}`,
		},
		{
			name:  "Submission",
			value: &Submission{},
			json:  `{"maxDelayedSend":1,"submissionExtensions":{"FUTURERELEASE":["86400"]}}`,
		},
		{
			name:  "Rights",
			value: &Rights{},
			json: `{"mayReadItems":true,"mayAddItems":true,"mayRemoveItems":true,` +
				`"maySetSeen":true,"maySetKeywords":true,"mayCreateChild":true,` +
				`"mayRename":true,"mayDelete":true,"maySubmit":true}`,
		},
		{
			name:  "Mailbox",
			value: &Mailbox{},
			json: `{"id":"MA","name":"Inbox","parentId":"MP","role":"inbox","sortOrder":10,` +
				`"totalEmails":5,"unreadEmails":4,"totalThreads":3,"unreadThreads":2,` +
				`"myRights":{"mayReadItems":true},"isSubscribed":true}`,
		},
		{
			name:  "MailboxGetResponse",
			value: &MailboxGetResponse{},
			json:  `{"accountId":"A1","state":"s1","list":[{"id":"MA"}],"notFound":["MX"]}`,
		},
		{
			name:  "MailboxChangesResponse",
			value: &MailboxChangesResponse{},
			json: `{"accountId":"A1","oldState":"s0","newState":"s1","hasMoreChanges":true,` +
				`"created":["MA"],"updated":["MB"],"destroyed":["MC"],` +
				`"updatedProperties":["totalEmails"]}`,
		},
		{
			name:  "MailboxQueryResponse",
			value: &MailboxQueryResponse{},
			json: `{"accountId":"A1","queryState":"q1","canCalculateChanges":true,` +
				`"position":1,"ids":["MA"],"total":2,"limit":3}`,
		},
		{
			name:  "MailboxSetResponse",
			value: &MailboxSetResponse{},
			json: `{"accountId":"A1","oldState":"s0","newState":"s1",` +
				`"created":{"k1":{"id":"MA"}},"updated":{"MB":{"id":"MB"}},"destroyed":["MC"],` +
				`"notCreated":{"k2":{"type":"invalidProperties"}},` +
				`"notUpdated":{"MD":{"type":"notFound"}},` +
				`"notDestroyed":{"ME":{"type":"mailboxHasEmail"}}}`,
		},
		{
			name:  "Address",
			value: &Address{},
			json:  `{"name":"Ann","email":"ann@x"}`,
		},
		{
			name:  "Header",
			value: &Header{},
			json:  `{"name":"X-Spam","value":"0.1"}`,
		},
		{
			name:  "BodyValue",
			value: &BodyValue{},
			json:  `{"value":"hi","isEncodingProblem":true,"isTruncated":true}`,
		},
		{
			name:  "BodyPart",
			value: &BodyPart{},
			json: `{"partId":"1","blobId":"G2","size":12,` +
				`"headers":[{"name":"Content-Type"}],"name":"a.txt","type":"text/plain",` +
				`"charset":"utf-8","disposition":"inline","cid":"c1","language":["en"],` +
				`"location":"http://x/","subParts":[{"partId":"2"}]}`,
		},
		{
			name:  "Email",
			value: &Email{},
			json: `{"id":"M1","blobId":"G1","threadId":"T1","mailboxIds":{"MA":true},` +
				`"keywords":{"$seen":true},"size":4127,"receivedAt":"2026-07-28T09:00:00Z",` +
				`"headers":[{"name":"X-Spam"}],"messageId":["m1@x"],"inReplyTo":["m0@x"],` +
				`"references":["m0@x"],"sender":[{"email":"s@x"}],"from":[{"email":"f@x"}],` +
				`"to":[{"email":"t@x"}],"cc":[{"email":"c@x"}],"bcc":[{"email":"b@x"}],` +
				`"replyTo":[{"email":"r@x"}],"subject":"Lunch",` +
				`"sentAt":"2026-07-28T08:59:00Z","bodyStructure":{"partId":"1"},` +
				`"bodyValues":{"1":{"value":"hi"}},"textBody":[{"partId":"1"}],` +
				`"htmlBody":[{"partId":"2"}],"attachments":[{"partId":"3"}],` +
				`"hasAttachment":true,"preview":"hi","smimeStatus":"signed",` +
				`"smimeStatusAtDelivery":"signed","smimeErrors":["e"],` +
				`"smimeVerifiedAt":"2026-07-28T09:01:00Z"}`,
		},
		{
			name:  "EmailGetResponse",
			value: &EmailGetResponse{},
			json:  `{"accountId":"A1","state":"s1","list":[{"id":"M1"}],"notFound":["MX"]}`,
		},
		{
			name:  "EmailChangesResponse",
			value: &EmailChangesResponse{},
			json: `{"accountId":"A1","oldState":"s0","newState":"s1","hasMoreChanges":true,` +
				`"created":["M1"],"updated":["M2"],"destroyed":["M3"]}`,
		},
		{
			name:  "EmailQueryResponse",
			value: &EmailQueryResponse{},
			json: `{"accountId":"A1","queryState":"q1","canCalculateChanges":true,` +
				`"position":1,"ids":["M1"],"total":2,"limit":3}`,
		},
		{
			name:  "EmailSetResponse",
			value: &EmailSetResponse{},
			json: `{"accountId":"A1","oldState":"s0","newState":"s1",` +
				`"created":{"k1":{"id":"M1"}},"updated":{"M2":null},"destroyed":["M3"],` +
				`"notCreated":{"k2":{"type":"blobNotFound","notFound":["G9"]}},` +
				`"notUpdated":{"M4":{"type":"tooManyMailboxes"}},` +
				`"notDestroyed":{"M5":{"type":"forbidden"}}}`,
		},
		{
			name:  "EmailImportResponse",
			value: &EmailImportResponse{},
			json: `{"accountId":"A1","oldState":"s0","newState":"s1",` +
				`"created":{"k1":{"id":"M1"}},"notCreated":{"k2":{"type":"invalidEmail"}}}`,
		},
		{
			name:  "Identity",
			value: &Identity{},
			json: `{"id":"I1","name":"Ann","email":"ann@x","replyTo":[{"email":"r@x"}],` +
				`"bcc":[{"email":"b@x"}],"textSignature":"-- Ann",` +
				`"htmlSignature":"Ann, in HTML","mayDelete":true}`,
		},
		{
			name:  "IdentityGetResponse",
			value: &IdentityGetResponse{},
			json:  `{"accountId":"A1","state":"s1","list":[{"id":"I1"}],"notFound":["IX"]}`,
		},
		{
			name:  "DeliveryStatus",
			value: &DeliveryStatus{},
			json:  `{"smtpReply":"250 ok","delivered":"yes","displayed":"unknown"}`,
		},
		{
			name:  "EnvelopeAddress",
			value: &EnvelopeAddress{},
			json:  `{"email":"f@x","parameters":{"HOLDFOR":"86400"}}`,
		},
		{
			name:  "Envelope",
			value: &Envelope{},
			json:  `{"mailFrom":{"email":"f@x"},"rcptTo":[{"email":"t@x"}]}`,
		},
		{
			name:  "EmailSubmission",
			value: &EmailSubmission{},
			json: `{"id":"ES1","identityId":"I1","emailId":"M1","threadId":"T1",` +
				`"envelope":{"mailFrom":{"email":"f@x"}},"sendAt":"2026-07-28T09:00:00Z",` +
				`"undoStatus":"final","deliveryStatus":{"t@x":{"delivered":"yes"}},` +
				`"dsnBlobIds":["G3"],"mdnBlobIds":["G4"]}`,
		},
		{
			name:  "EmailSubmissionSetResponse",
			value: &EmailSubmissionSetResponse{},
			json: `{"accountId":"A1","oldState":"s0","newState":"s1",` +
				`"created":{"k1":{"id":"ES1"}},"updated":{"ES2":{"id":"ES2"}},` +
				`"destroyed":["ES3"],` +
				`"notCreated":{"k2":{"type":"tooManyRecipients","maxRecipients":25}},` +
				`"notUpdated":{"ES4":{"type":"cannotUnsend"}},` +
				`"notDestroyed":{"ES5":{"type":"forbidden"}}}`,
		},
		{
			name:  "SetError",
			value: &SetError{},
			json: `{"type":"invalidRecipients","description":"two addresses refuse mail",` +
				`"properties":["envelope"],"notFound":["G9"],"maxRecipients":25,` +
				`"invalidRecipients":["a@","b@"]}`,
		},
		{
			name:  "MethodError",
			value: &MethodError{},
			json:  `{"type":"invalidArguments","description":"sinceState is required"}`,
		},
		{
			name:  "RequestError",
			value: &RequestError{},
			json: `{"type":"urn:ietf:params:jmap:error:limit","status":400,` +
				`"detail":"too large","limit":"maxSizeRequest"}`,
		},
		{
			name:  "StateChange",
			value: &StateChange{},
			json:  `{"@type":"StateChange","changed":{"a1":{"Email":"e1"}}}`,
		},
	}
}

// TestRequestTags marshals a fully populated value of every type
// poplar sends and compares it to JSON that names every tag.
func TestRequestTags(t *testing.T) {
	for _, c := range requestTagCases() {
		t.Run(c.name, func(t *testing.T) {
			data, err := json.Marshal(c.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.json {
				t.Errorf("Marshal =\n%s\nwant\n%s", data, c.json)
			}
		})
	}
}

// TestResponseTags round-trips a fully populated literal through every
// type a server sends. A renamed tag decodes as the zero value and,
// because every response property carries omitempty, disappears from
// the re-marshal.
func TestResponseTags(t *testing.T) {
	for _, c := range responseTagCases() {
		t.Run(c.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(c.json), c.value); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			data, err := json.Marshal(c.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(data) != c.json {
				t.Errorf("round trip =\n%s\nwant\n%s", data, c.json)
			}
		})
	}
}

// packageTags reads the package's own non-test source and returns
// each struct type's JSON tag names, in declaration order.
func packageTags(t *testing.T) map[string][]string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	tags := make(map[string][]string)
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structType.Fields.List {
					if field.Tag == nil {
						continue
					}
					raw, err := strconv.Unquote(field.Tag.Value)
					if err != nil {
						t.Fatalf("%s: unquote tag %s: %v", name, field.Tag.Value, err)
					}
					tag, _, _ := strings.Cut(reflect.StructTag(raw).Get("json"), ",")
					if tag == "" || tag == "-" {
						continue
					}
					tags[typeSpec.Name.Name] = append(tags[typeSpec.Name.Name], tag)
				}
			}
		}
	}
	return tags
}

// TestEveryJSONTagIsObserved reads the package's own source and fails
// on a struct no case is named for, or a tag its own case does not
// spell. Without it the tables decay: a field added to a type later
// is unwatched, and renaming it stays green.
//
// Each type is matched against its own case alone, and against that
// case's top-level keys rather than its text. A case literal carries
// the JSON of every type nested inside it, so searching the text
// counts a child's key as the parent's: an "id" added to a response
// type reads as observed because a record in its list has one.
func TestEveryJSONTagIsObserved(t *testing.T) {
	observed := make(map[string]string)
	for _, c := range append(requestTagCases(), responseTagCases()...) {
		if _, duplicate := observed[c.name]; duplicate {
			t.Errorf("two cases are named %s; one type takes one case, or their strings pool", c.name)
		}
		observed[c.name] = c.json
	}

	tags := packageTags(t)

	for _, typeName := range slices.Sorted(maps.Keys(observed)) {
		if _, ok := tags[typeName]; !ok {
			t.Errorf("a case is named %q, which is not a struct with JSON tags here", typeName)
		}
	}

	total := 0
	for _, typeName := range slices.Sorted(maps.Keys(tags)) {
		seen, ok := observed[typeName]
		if !ok {
			t.Errorf("no case is named for type %s; add one to a table in tags_test.go", typeName)
			continue
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal([]byte(seen), &top); err != nil {
			t.Errorf("%s: its case JSON is not an object: %v", typeName, err)
			continue
		}
		for _, tag := range tags[typeName] {
			total++
			if _, named := top[tag]; !named {
				t.Errorf("%s: its case does not name the %q tag", typeName, tag)
			}
		}
	}

	// A guard against the guard: a parser that stopped finding fields
	// would pass this test by having nothing to check.
	if total < 300 {
		t.Errorf("found only %d tags in the package source, want at least 300", total)
	}
}

// mustParseDate builds a Date from an RFC 3339 literal, for the table
// values above.
func mustParseDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
