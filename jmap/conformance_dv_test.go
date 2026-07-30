//go:build conformance

// The twelve divergence tests. Each pins poplar's behaviour where two
// conformant servers may differ, so a future server cannot change one
// of these silently. Where a divergence is a fact about the server
// rather than a choice poplar makes, the test records it and asserts
// only what poplar owes: surfacing what the server said, and never
// compensating for it client-side.
package jmap_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/jmap"
)

// TestDV01UploadStatus covers DV-01. RFC 8620 section 6.1 mandates the
// response body and says nothing about the status, so Cyrus answers
// 201 and adds a fifth property while Fastmail and Stalwart answer
// 200. go-jmap accepted only an exact 200, which made uploading to
// Cyrus fail outright.
func TestDV01UploadStatus(t *testing.T) {
	tg := dial(t)

	const payload = "DV-01\n"
	url := strings.ReplaceAll(tg.session.UploadURL, "{accountId}", string(tg.account))
	resp, body := tg.post(t, url, "text/plain", payload)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("upload answered %d: %s", resp.StatusCode, body)
	}
	if want := tg.profile.uploadStatus; want != 0 && resp.StatusCode != want {
		t.Errorf("%s answered %d, and its profile says %d", tg.profile.name, resp.StatusCode, want)
	}

	// Whatever the status, the same upload through the package
	// succeeds and reports what the server stored.
	uploaded := tg.upload(t, "text/plain", payload)
	if uploaded.Size != uint64(len(payload)) {
		t.Errorf("Upload reported size %d for %d bytes", uploaded.Size, len(payload))
	}
	if got := tg.download(t, uploaded.BlobID); got != payload {
		t.Errorf("the stored blob reads %q, want %q", got, payload)
	}

	// A property beyond the RFC's four decodes away rather than
	// failing the upload. Cyrus sends expires, and a client that
	// rejected the extra could not talk to it at all.
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decode the upload response: %v", err)
	}
	var extra []string
	for property := range sent {
		if !slices.Contains([]string{"accountId", "blobId", "type", "size"}, property) {
			extra = append(extra, property)
		}
	}
	t.Logf("%s answers an upload with %d and %d properties beyond the RFC's four %v",
		tg.profile.name, resp.StatusCode, len(extra), extra)
}

// TestDV02OptionalBooleanSortOrder covers DV-02. Sorting on
// hasKeyword put Fastmail and Stalwart in different orders, because
// nothing in the specification fixes the order of a Boolean, let alone
// an optional one. Either order is conformant, so what poplar owes is
// to hand back the server's order untouched.
func TestDV02OptionalBooleanSortOrder(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("dv02"))
	for i := range 4 {
		keywords := []string{}
		if i%2 == 0 {
			keywords = append(keywords, "$flagged")
		}
		tg.newEmail(t, mailbox, scratch("dv02-seed"), keywords...)
	}

	for _, ascending := range []bool{true, false} {
		sort := []*jmap.Comparator{{Property: "hasKeyword", Keyword: "$flagged"}}
		if !ascending {
			sort[0].IsAscending = new(false)
		}

		typed := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: &jmap.EmailFilterCondition{InMailbox: mailbox}, Sort: sort}, typed)

		body := tg.raw(t, `{"using":["urn:ietf:params:jmap:core","urn:ietf:params:jmap:mail"],`+
			`"methodCalls":[["Email/query",{"accountId":"`+string(tg.account)+
			`","filter":{"inMailbox":"`+string(mailbox)+`"},"sort":[{"property":"hasKeyword",`+
			`"keyword":"$flagged"`+ascendingArgument(ascending)+`}]},"0"]]}`)

		var wire struct {
			IDs []jmap.ID `json:"ids"`
		}
		if err := json.Unmarshal(argumentsOf(t, body, 0), &wire); err != nil {
			t.Fatalf("decode the server's own bytes: %v", err)
		}
		if !slices.Equal(typed.IDs, wire.IDs) {
			t.Errorf("ascending=%v: the package reports %v while the server sent %v",
				ascending, typed.IDs, wire.IDs)
		}
	}
}

func ascendingArgument(ascending bool) string {
	if ascending {
		return ""
	}
	return `,"isAscending":false`
}

// TestDV03NoNullFilter covers DV-03. Stalwart rejected an explicit
// filter of null until v0.16.10 and RFC 8620 does not forbid it, so
// poplar leaves the property out entirely rather than depending on
// the ambiguity.
func TestDV03NoNullFilter(t *testing.T) {
	tg := dial(t)

	for _, row := range []struct {
		name  string
		query jmap.Method
	}{
		{"Email/query", &jmap.EmailQuery{Account: tg.account, Limit: 1}},
		{"Mailbox/query", &jmap.MailboxQuery{Account: tg.account, Limit: 1}},
		{"Email/queryChanges", &jmap.EmailQueryChanges{Account: tg.account, SinceQueryState: "x"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			data, err := json.Marshal(row.query)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if strings.Contains(string(data), `"filter"`) {
				t.Errorf("a query with no filter marshalled to %s", data)
			}
		})
	}

	// The unfiltered form the package does send is one the server
	// answers, which is the half a marshalling assertion cannot show.
	resp := &jmap.EmailQueryResponse{}
	tg.call(t, &jmap.EmailQuery{Account: tg.account, Limit: 1}, resp)
	if resp.QueryState == "" {
		t.Error("the unfiltered query came back with no query state")
	}
}

// TestDV04MissingNotFound covers DV-04. Stalwart omitted notFound from
// /get responses until v0.16.10, so poplar treats an absent array as
// the server saying nothing rather than as nothing being missing, and
// works out the missing set itself.
func TestDV04MissingNotFound(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("dv04"))
	real := tg.newEmail(t, mailbox, scratch("dv04"))
	const ghost jmap.ID = "thisMessageDoesNotExist"

	resp := &jmap.EmailGetResponse{}
	tg.call(t, &jmap.EmailGet{
		Account:    tg.account,
		IDs:        []jmap.ID{real, ghost},
		Properties: []string{"id"},
	}, resp)

	if len(resp.List) != 1 || resp.List[0].ID != real {
		t.Fatalf("the server returned %d messages, want only %s", len(resp.List), real)
	}

	// The set poplar can always compute: what it asked for, minus what
	// came back. It holds whether or not the server said anything.
	returned := make(map[jmap.ID]bool, len(resp.List))
	for _, email := range resp.List {
		returned[email.ID] = true
	}
	var computed []jmap.ID
	for _, id := range []jmap.ID{real, ghost} {
		if !returned[id] {
			computed = append(computed, id)
		}
	}
	if !slices.Equal(computed, []jmap.ID{ghost}) {
		t.Errorf("requested minus returned is %v, want [%s]", computed, ghost)
	}

	// A nil NotFound is "the server did not tell me": absent and empty
	// decode differently, so the distinction survives.
	if resp.NotFound == nil {
		t.Logf("DIVERGENCE: %s omitted notFound, which RFC 8620 section 5.1 makes mandatory", tg.profile.name)
	} else {
		if !slices.Contains(resp.NotFound, ghost) {
			t.Errorf("notFound = %v, want it to name %s", resp.NotFound, ghost)
		}
		if slices.Contains(resp.NotFound, real) {
			t.Errorf("notFound named %s, which the server also returned", real)
		}
	}

	// The other end of the same distinction, and the one that pins it:
	// a get whose ids all exist. An empty notFound is the server saying
	// nothing was missing, an absent one is the server saying nothing
	// at all, and the two must not arrive as the same Go value. They do
	// the moment anything reads the decoded response instead of the
	// bytes: every property here carries omitempty, so an empty array
	// re-marshals to nothing.
	req := &jmap.Request{}
	callID := req.Invoke(&jmap.EmailGet{
		Account:    tg.account,
		IDs:        []jmap.ID{real},
		Properties: []string{"id"},
	})
	answer := only(t, tg.do(t, req), callID)

	var properties map[string]json.RawMessage
	if err := json.Unmarshal(answer.Raw, &properties); err != nil {
		t.Fatalf("decode the response arguments the server sent: %v", err)
	}
	sent, onTheWire := properties["notFound"]

	present := &jmap.EmailGetResponse{}
	assign(t, answer, present)
	switch {
	case onTheWire && present.NotFound == nil:
		t.Errorf("%s sent notFound as %s and it decoded as absent", tg.profile.name, sent)
	case !onTheWire && present.NotFound != nil:
		t.Errorf("%s sent no notFound and it decoded as %v", tg.profile.name, present.NotFound)
	case onTheWire && len(present.NotFound) != 0:
		t.Errorf("%s named %v as missing from a get whose every id exists", tg.profile.name, present.NotFound)
	}
}

// TestDV05SessionStateIsOpaque covers DV-05, and JT-18 against a
// second server. Fastmail's value is visibly structured and Stalwart's
// is not; RFC 8620 section 2 says a client should parse neither. The
// assertion is that what the package exposes is what the server sent,
// byte for byte, whatever shape that is.
func TestDV05SessionStateIsOpaque(t *testing.T) {
	tg := dial(t)

	var wire struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(tg.rawSession(t), &wire); err != nil {
		t.Fatalf("decode the session as bytes: %v", err)
	}
	if wire.State == "" {
		t.Fatal("the session carries no state, so nothing would ever trigger a refetch")
	}
	if tg.session.State != wire.State {
		t.Errorf("the session exposes state %q while the server sent %q", tg.session.State, wire.State)
	}

	req := &jmap.Request{}
	req.Invoke(jmap.Echo{})
	if got := tg.do(t, req).SessionState; got != wire.State {
		t.Errorf("a response names session state %q while the session says %q", got, wire.State)
	}
	t.Logf("%s spells its session state %q", tg.profile.name, wire.State)
}

// TestDV06ErrorTypeSubstitution covers DV-06. Stalwart was reported
// substituting invalidArguments where the RFC names accountNotFound
// and anchorNotFound. poplar therefore never branches on a method
// error type for a correctness-critical decision, with the single
// exception of cannotCalculateChanges, which JT-16 tests on its own.
//
// What every row asserts is the invariant that holds whatever the type
// is: an error is never read as success.
func TestDV06ErrorTypeSubstitution(t *testing.T) {
	tg := dial(t)

	// foldExercised is set from the row that lands in the
	// unsupportedSort case below, the one type here Stalwart is known
	// to leave unnamed. Without it, a day Stalwart substitutes a named
	// type for that row too (it already does for the account row)
	// would leave every row asserting only that the fold does not
	// apply, and the test would stay green having never proved the
	// fold applies at all.
	var foldExercised bool
	for _, row := range []struct {
		name   string
		method jmap.Method
		rfc    string
	}{
		// An id RFC 8620 section 1.2's alphabet allows, so the server
		// answers about the account rather than about the string.
		{"an account the credentials do not reach", &jmap.MailboxGet{Account: "zz"}, "accountNotFound"},
		{"an anchor the query does not match", &jmap.EmailQuery{Account: tg.account, Anchor: "no-such-message"}, "anchorNotFound"},
		{"a sort property the server does not offer", &jmap.EmailQuery{
			Account: tg.account,
			Sort:    []*jmap.Comparator{{Property: "noSuchProperty"}},
		}, "unsupportedSort"},
	} {
		t.Run(row.name, func(t *testing.T) {
			req := &jmap.Request{}
			callID := req.Invoke(row.method)
			answer := only(t, tg.do(t, req), callID)

			failed, ok := answer.Args.(*jmap.MethodError)
			if !ok {
				t.Fatalf("%s answered %T, which a caller would read as success", row.method.Name(), answer.Args)
			}
			if failed.Type == "" {
				t.Errorf("the error carries no type: %s", failed.Raw)
			}
			if failed.Type != row.rfc {
				t.Logf("DIVERGENCE: %s answers %q where the RFC names %q", tg.profile.name, failed.Type, row.rfc)
			}

			// RFC 8620 section 3.6.2: a type the client does not
			// understand is treated as serverFail. unsupportedSort is
			// the row that proves it live: it is not in
			// jmap/errors.go's named set, so this is the fold running
			// against a type this package does not recognise, rather
			// than against a sentinel copied here to match itself.
			folds := errors.Is(failed, jmap.ErrServerFail)
			switch failed.Type {
			case "unsupportedSort":
				foldExercised = true
				if !folds {
					t.Errorf("%q does not fold into serverFail; section 3.6.2 requires an unnamed type to read as serverFail", failed.Type)
				}
			case jmap.ErrServerFail.Type:
			default:
				if folds {
					t.Errorf("%q folded into serverFail, but this package names that type; the fold should only catch an unnamed one", failed.Type)
				}
			}
			if folds && failed.Type != jmap.ErrServerFail.Type {
				t.Logf("%s answers %q, which this package does not name and reads as serverFail",
					tg.profile.name, failed.Type)
			}
			// The fold reaches serverFail and stops there. poplar
			// branches on exactly one method error, and an unrelated
			// error that also matched it would force a full resync of
			// the mailbox every time the server substituted a type.
			if errors.Is(failed, jmap.ErrCannotCalculateChanges) && failed.Type != jmap.ErrCannotCalculateChanges.Type {
				t.Errorf("%q matches the resync sentinel", failed.Type)
			}
		})
	}
	if !foldExercised {
		t.Fatal("no row reached the unnamed-type case, so the serverFail fold was never exercised; if the server now answers a different unnamed type, the case literal wants updating rather than the guard removing")
	}
}

// TestDV07QueryArgumentsAreNotCompensated covers DV-07. A live report
// showed collapseThreads, the hasAttachment filter, and the header
// filter all failing against Stalwart. Whether they work is the
// server's business; poplar's is to send the RFC 8621 section 4.4
// forms and surface whatever comes back rather than papering over it.
func TestDV07QueryArgumentsAreNotCompensated(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("dv07"))
	subject := scratch("dv07-thread")
	root := tg.draft(mailbox, subject, nil)
	root.MessageID = []string{"poplar-dv07-root@conformance.test"}
	reply := tg.draft(mailbox, "Re: "+subject, nil)
	reply.MessageID = []string{"poplar-dv07-reply@conformance.test"}
	reply.InReplyTo = root.MessageID
	reply.References = root.MessageID

	set := &jmap.EmailSetResponse{}
	tg.call(t, &jmap.EmailSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Email{"a": root, "b": reply},
	}, set)
	if set.Created["a"] == nil || set.Created["b"] == nil {
		t.Fatalf("create: %v", set.NotCreated)
	}
	inMailbox := &jmap.EmailFilterCondition{InMailbox: mailbox}

	all := &jmap.EmailQueryResponse{}
	tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: inMailbox}, all)
	if len(all.IDs) != 2 {
		t.Fatalf("the mailbox holds %v, want the two messages just filed", all.IDs)
	}

	t.Run("collapseThreads", func(t *testing.T) {
		collapsed := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: inMailbox, CollapseThreads: true}, collapsed)

		body := tg.raw(t, `{"using":["urn:ietf:params:jmap:core","urn:ietf:params:jmap:mail"],`+
			`"methodCalls":[["Email/query",{"accountId":"`+string(tg.account)+
			`","filter":{"inMailbox":"`+string(mailbox)+`"},"collapseThreads":true},"0"]]}`)
		var wire struct {
			IDs []jmap.ID `json:"ids"`
		}
		if err := json.Unmarshal(argumentsOf(t, body, 0), &wire); err != nil {
			t.Fatalf("decode the server's own bytes: %v", err)
		}
		if !slices.Equal(collapsed.IDs, wire.IDs) {
			t.Errorf("the package reports %v while the server sent %v", collapsed.IDs, wire.IDs)
		}
		if len(collapsed.IDs) == len(all.IDs) {
			t.Logf("DIVERGENCE: %s returned %d messages for a thread of %d with collapseThreads set",
				tg.profile.name, len(collapsed.IDs), len(all.IDs))
		}
	})

	t.Run("hasAttachment", func(t *testing.T) {
		// Neither message carries an attachment, so the two filters
		// partition the mailbox and the partition is checkable.
		without := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account: tg.account,
			Filter: &jmap.FilterOperator{Operator: jmap.OperatorAND, Conditions: []jmap.Filter{
				inMailbox, &jmap.EmailFilterCondition{HasAttachment: new(false)},
			}},
		}, without)
		with := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account: tg.account,
			Filter: &jmap.FilterOperator{Operator: jmap.OperatorAND, Conditions: []jmap.Filter{
				inMailbox, &jmap.EmailFilterCondition{HasAttachment: new(true)},
			}},
		}, with)

		if len(without.IDs)+len(with.IDs) != len(all.IDs) {
			t.Logf("DIVERGENCE: %s matched %d without and %d with an attachment, out of %d messages",
				tg.profile.name, len(without.IDs), len(with.IDs), len(all.IDs))
		}
		for _, id := range with.IDs {
			t.Errorf("hasAttachment true matched %s, which carries none", id)
		}
	})

	t.Run("header name only", func(t *testing.T) {
		// Every message has a Subject header, so a name-only header
		// condition matches all of them or the server does not
		// implement the form.
		matched := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account: tg.account,
			Filter: &jmap.FilterOperator{Operator: jmap.OperatorAND, Conditions: []jmap.Filter{
				inMailbox, &jmap.EmailFilterCondition{Header: []string{"Subject"}},
			}},
		}, matched)

		for _, id := range matched.IDs {
			if !slices.Contains(all.IDs, id) {
				t.Errorf("the header filter matched %s, which is not in the mailbox", id)
			}
		}
		if len(matched.IDs) != len(all.IDs) {
			t.Logf("DIVERGENCE: %s matched %d of %d messages on a name-only Subject header condition",
				tg.profile.name, len(matched.IDs), len(all.IDs))
		}
	})
}

// TestDV08PushIsReadFromTheSession covers DV-08. Cyrus implements RFC
// 8620 section 7.2's PushSubscription object and Stalwart does; a
// client must never assume either. This package models no
// PushSubscription at all, so what it owes is that its one push
// mechanism comes out of the session rather than out of a guess.
func TestDV08PushIsReadFromTheSession(t *testing.T) {
	tg := dial(t)

	var wire struct {
		EventSourceURL string `json:"eventSourceUrl"`
	}
	if err := json.Unmarshal(tg.rawSession(t), &wire); err != nil {
		t.Fatalf("decode the session as bytes: %v", err)
	}
	if tg.session.EventSourceURL != wire.EventSourceURL {
		t.Errorf("the session exposes event source %q while the server sent %q",
			tg.session.EventSourceURL, wire.EventSourceURL)
	}
	if wire.EventSourceURL == "" {
		t.Skipf("%s advertises no event source, so there is nothing to fall back to", tg.profile.name)
	}

	// The URL is a template over the three variables RFC 8620 section
	// 7.3 names. A server that offered fewer would leave Listen
	// building a URL the server never described.
	for _, variable := range []string{"{types}", "{closeafter}", "{ping}"} {
		if !strings.Contains(wire.EventSourceURL, variable) {
			t.Errorf("the event source template %q has no %s", wire.EventSourceURL, variable)
		}
	}
}

// TestDV09EventSourceResumption covers DV-09, and JT-22 against a real
// server. RFC 8620 section 7.3 makes replaying missed events on
// Last-Event-ID a SHOULD and defines no error for a server that
// remembers nothing, so poplar cannot depend on it. What it does
// instead is treat every new connection as a gap and hand the caller
// the signal to pull /changes, which is what this asserts.
func TestDV09EventSourceResumption(t *testing.T) {
	tg := dial(t)

	// A state event is what a resume point would have to identify, so
	// the stream is read while one is provoked rather than waiting for
	// a ping.
	mailbox := tg.newMailbox(t, scratch("dv09"))
	stop := tg.provoke(t, mailbox)
	events := tg.readEventStream(t, "types=*&closeafter=state&ping=0", 1)
	stop()

	if len(events) == 0 {
		t.Fatal("the event source sent nothing within the window")
	}
	if events[0].id == "" {
		t.Logf("DIVERGENCE: %s sends no id field, so Last-Event-ID has nothing to resume from and every reconnect is a gap",
			tg.profile.name)
	}

	// The gap signal itself: one connection, one OnConnect, before any
	// event reaches the caller.
	var mu sync.Mutex
	var connects int
	var sawChange bool
	open := make(chan struct{}, 1)
	done := make(chan error, 1)

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	go func() {
		done <- tg.client.Listen(ctx, jmap.EventSource{
			CloseAfterState: true,
			OnConnect: func() {
				mu.Lock()
				connects++
				mu.Unlock()
				open <- struct{}{}
			},
			OnChange: func(*jmap.StateChange) {
				mu.Lock()
				sawChange = true
				mu.Unlock()
			},
		})
	}()
	waitForConnection(t, open, done)

	tg.newEmail(t, mailbox, scratch("dv09-listen"))

	if err := <-done; err != nil {
		t.Fatalf("Listen: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if connects != 1 {
		t.Errorf("OnConnect fired %d times for one connection", connects)
	}
	if !sawChange {
		t.Error("the one-shot stream returned without delivering a state change")
	}
}

// waitForConnection blocks until the stream is open, so a change made
// afterwards is one the stream can carry. A change made before the
// connection is a change the server has no reason to replay.
func waitForConnection(t *testing.T, open <-chan struct{}, done <-chan error) {
	t.Helper()

	select {
	case <-open:
	case err := <-done:
		t.Fatalf("Listen returned before connecting: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("Listen never announced a connection")
	}
}

// TestDV10ProblemDetails covers DV-10, and JT-29 and JT-30 against a
// real server. RFC 7807 specifies application/problem+json and go-jmap
// required exactly application/json, which reduced a conformant
// server's whole diagnostic to an HTTP status.
func TestDV10ProblemDetails(t *testing.T) {
	tg := dial(t)

	// The body is not JSON, so the server rejects the request itself
	// rather than any method in it. The literal verbs are there because
	// Stalwart echoes the body into the detail, which makes this the
	// one place a server-supplied format string reaches poplar's error
	// text for real.
	const poisoned = `%s %d %!v(MISSING) not a request`
	resp, body := tg.post(t, tg.session.APIURL, "application/json", poisoned)
	if resp.StatusCode < 400 {
		t.Fatalf("a malformed request body answered %d: %s", resp.StatusCode, body)
	}
	t.Logf("%s rejects a malformed request with %d and content type %q",
		tg.profile.name, resp.StatusCode, resp.Header.Get("Content-Type"))

	requestErr := &jmap.RequestError{}
	if err := json.Unmarshal(body, requestErr); err != nil {
		t.Fatalf("decode the problem details: %v", err)
	}
	if requestErr.Type == "" {
		t.Errorf("the problem details carry no type: %s", body)
	}
	if !strings.HasPrefix(requestErr.Type, "urn:ietf:params:jmap:error:") {
		t.Errorf("the problem type is %q, want one of RFC 8620 section 3.6.1's URNs", requestErr.Type)
	}

	// JT-30 from the wire: whatever prose the server put in the
	// detail reaches the error text unchanged.
	if requestErr.Detail != "" {
		if got := requestErr.Error(); !strings.Contains(got, requestErr.Detail) {
			t.Errorf("Error() = %q, which does not carry the server's detail %q", got, requestErr.Detail)
		}
		if strings.Contains(requestErr.Error(), "MISSING)%!") {
			t.Errorf("Error() = %q; the detail was used as a format string", requestErr.Error())
		}
	}
}

// TestDV11DuplicateMailboxName covers DV-11. The alreadyExists against
// invalidProperties boundary is underspecified for a duplicate name
// under one parent, so poplar accepts either and the only hard
// assertion is the one about the account: no second mailbox.
func TestDV11DuplicateMailboxName(t *testing.T) {
	tg := dial(t)

	// Created by hand rather than through the harness, because this
	// test asks the server about the name it chose and the harness is
	// free to vary it.
	name := scratch("dv11")
	made := &jmap.MailboxSetResponse{}
	tg.call(t, &jmap.MailboxSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Mailbox{"m": {Name: name}},
	}, made)
	if made.Created["m"] == nil {
		t.Fatalf("create mailbox %q: %v", name, made.NotCreated["m"])
	}
	first := made.Created["m"].ID
	t.Cleanup(func() { tg.destroyMailbox(first) })

	resp := &jmap.MailboxSetResponse{}
	tg.call(t, &jmap.MailboxSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Mailbox{"again": {Name: name}},
	}, resp)

	if created := resp.Created["again"]; created != nil {
		t.Cleanup(func() { tg.destroyMailbox(created.ID) })
		t.Fatalf("the server created a second mailbox named %q as %s", name, created.ID)
	}
	refused := resp.NotCreated["again"]
	if refused == nil {
		t.Fatal("the server neither created the mailbox nor said why")
	}
	if refused.Type != "alreadyExists" && refused.Type != "invalidProperties" {
		t.Errorf("the refusal is %q, want alreadyExists or invalidProperties", refused.Type)
	}
	if refused.Error() == "" {
		t.Error("the refusal renders as nothing, so a user would be told only that something failed")
	}
	// RFC 8620 section 5.3 makes existingId mandatory on
	// alreadyExists, and it is the one refusal a client can act on
	// without asking the user anything.
	if refused.Type == "alreadyExists" && refused.ExistingID != first {
		t.Errorf("alreadyExists names existing record %q, want %q", refused.ExistingID, first)
	}

	query := &jmap.MailboxQueryResponse{}
	tg.call(t, &jmap.MailboxQuery{
		Account: tg.account,
		Filter:  &jmap.MailboxFilterCondition{Name: name},
	}, query)
	if !slices.Equal(query.IDs, []jmap.ID{first}) {
		t.Errorf("the account holds %v named %q, want only %s", query.IDs, name, first)
	}
	t.Logf("%s refuses a duplicate name with %q", tg.profile.name, refused.Type)
}

// TestDV12EventSourceDeliversStateChanges covers DV-12, and JT-27 and
// JT-28 against a real server. A live report had the state-change test
// failing against Fastmail while passing against Stalwart, and called
// some of those results spurious, so this asserts the mechanism rather
// than encoding a defect that may not exist.
func TestDV12EventSourceDeliversStateChanges(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("dv12"))

	changes := make(chan *jmap.StateChange, 4)
	open := make(chan struct{}, 1)
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	go func() {
		done <- tg.client.Listen(ctx, jmap.EventSource{
			Types:           []jmap.EventType{"Email"},
			CloseAfterState: true,
			OnConnect:       func() { open <- struct{}{} },
			OnChange:        func(change *jmap.StateChange) { changes <- change },
		})
	}()
	waitForConnection(t, open, done)

	tg.newEmail(t, mailbox, scratch("dv12"))

	// JT-27: closeafter=state ends the stream after one event, so
	// Listen returns nil rather than reconnecting.
	if err := <-done; err != nil {
		t.Fatalf("Listen: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("the one-shot stream delivered %d state changes, want 1", len(changes))
	}

	change := <-changes
	if change.Type != "StateChange" {
		t.Errorf("the event's @type is %q, want StateChange", change.Type)
	}
	// JT-28: the fan-out is per account and per type, and the state it
	// names is the one a /get on that type reports now.
	types, ok := change.Changed[tg.account]
	if !ok {
		t.Fatalf("the change names accounts %v, not %s", slices.Collect(maps.Keys(change.Changed)), tg.account)
	}
	pushed, ok := types["Email"]
	if !ok {
		t.Fatalf("the change names types %v, not Email", slices.Collect(maps.Keys(types)))
	}
	if current := tg.emailState(t); current != pushed {
		t.Errorf("the push named Email state %q while Email/get reports %q", pushed, current)
	}
}
