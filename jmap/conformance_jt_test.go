//go:build conformance

package jmap_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/glw907/poplar/jmap"
)

// TestConformanceSessionIsUsable covers JT-19 and JT-20 against a
// second server. The typed capability objects decode from what the
// server advertises rather than from an RFC transcription, and a
// capability URI this package does not model survives in raw form
// instead of failing the session.
func TestConformanceSessionIsUsable(t *testing.T) {
	tg := dial(t)

	core, ok := tg.session.Capabilities[jmap.CoreURI].(*jmap.Core)
	if !ok {
		t.Fatalf("core capability decoded as %T, want *jmap.Core", tg.session.Capabilities[jmap.CoreURI])
	}
	for _, limit := range []struct {
		name  string
		value uint64
	}{
		{"MaxSizeUpload", core.MaxSizeUpload},
		{"MaxSizeRequest", core.MaxSizeRequest},
		{"MaxConcurrentRequests", core.MaxConcurrentRequests},
		{"MaxCallsInRequest", core.MaxCallsInRequest},
		{"MaxObjectsInGet", core.MaxObjectsInGet},
		{"MaxObjectsInSet", core.MaxObjectsInSet},
	} {
		if limit.value == 0 {
			t.Errorf("Core.%s = 0; the server states a real limit, so a zero is a mistyped tag", limit.name)
		}
	}
	if len(core.CollationAlgorithms) == 0 {
		t.Error("Core.CollationAlgorithms is empty")
	}

	account, ok := tg.session.Accounts[tg.account]
	if !ok {
		t.Fatalf("session has no account %q", tg.account)
	}
	mail, ok := account.Capabilities[jmap.MailURI].(*jmap.Mail)
	if !ok {
		t.Fatalf("account mail capability decoded as %T, want *jmap.Mail", account.Capabilities[jmap.MailURI])
	}
	if mail.MaxSizeMailboxName == 0 {
		t.Error("Mail.MaxSizeMailboxName = 0")
	}
	if len(mail.EmailQuerySortOptions) == 0 {
		t.Error("Mail.EmailQuerySortOptions is empty; nothing would name a legal sort")
	}

	// JT-20: the raw map holds every URI the server sent, whether this
	// package models it or not, so a server extension costs nothing.
	var advertised struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(tg.rawSession(t), &advertised); err != nil {
		t.Fatalf("decode the session as bytes: %v", err)
	}
	var unmodelled int
	for uri := range advertised.Capabilities {
		if _, kept := tg.session.RawCapabilities[jmap.URI(uri)]; !kept {
			t.Errorf("the session dropped capability %q", uri)
		}
		if _, modelled := tg.session.Capabilities[jmap.URI(uri)]; !modelled {
			unmodelled++
		}
	}
	if unmodelled == 0 {
		t.Errorf("every one of %s's %d capabilities is modelled here, so nothing proves an unknown one survives",
			tg.profile.name, len(advertised.Capabilities))
	}
}

// TestConformanceSetPartialFailure covers JT-01, the highest-value
// case in the inventory, against a real server. One Mailbox/set asks
// for three creates, two of which the server must refuse, and the
// account afterwards holds exactly the one that worked.
func TestConformanceSetPartialFailure(t *testing.T) {
	tg := dial(t)

	good := scratch("partial-ok")
	resp := &jmap.MailboxSetResponse{}
	tg.call(t, &jmap.MailboxSet{
		Account: tg.account,
		Create: map[jmap.ID]*jmap.Mailbox{
			"ok":       {Name: good},
			"orphan":   {Name: scratch("partial-orphan"), ParentID: "this-parent-does-not-exist"},
			"nameless": {},
		},
	}, resp)

	created := resp.Created["ok"]
	if created == nil {
		t.Fatalf("the valid create failed: %v", resp.NotCreated["ok"])
	}
	t.Cleanup(func() { tg.destroyMailbox(created.ID) })

	for _, key := range []jmap.ID{"orphan", "nameless"} {
		refused := resp.NotCreated[key]
		if refused == nil {
			t.Errorf("create %q was not refused; NotCreated holds %v", key, resp.NotCreated)
			continue
		}
		if refused.Type == "" {
			t.Errorf("NotCreated[%s] carries no type: %s", key, refused.Raw)
		}
	}

	// The state advanced for the record that landed, which is what
	// makes a caller reading only the method's error return believe
	// every record landed.
	if resp.NewState == resp.OldState {
		t.Errorf("state stayed %q across a call that created a mailbox", resp.NewState)
	}

	// The server's own view: one mailbox by that name, not three.
	query := &jmap.MailboxQueryResponse{}
	tg.call(t, &jmap.MailboxQuery{
		Account: tg.account,
		Filter:  &jmap.MailboxFilterCondition{Name: good},
	}, query)
	if len(query.IDs) != 1 {
		t.Errorf("the account holds %d mailboxes named %q, want 1", len(query.IDs), good)
	}
}

// TestConformancePatchTouchesOneLeaf covers JT-02 and JT-04 against a
// real server: the two cases where a wrong patch destroys mail rather
// than reporting an error. Every step reads the record back, because
// the damage a whole-property replacement does is invisible in the
// /set response.
func TestConformancePatchTouchesOneLeaf(t *testing.T) {
	tg := dial(t)

	from := tg.newMailbox(t, scratch("patch-from"))
	to := tg.newMailbox(t, scratch("patch-to"))
	email := tg.newEmail(t, from, scratch("patch"), "$draft", "$flagged")

	// JT-02: setting one keyword leaves the other two alone, and
	// clearing it with a null removes that leaf rather than the set.
	tg.update(t, email, jmap.Patch{jmap.Pointer("keywords", "$seen"): true})
	assertKeywords(t, tg.emails(t, email), email, "$draft", "$flagged", "$seen")

	tg.update(t, email, jmap.Patch{jmap.Pointer("keywords", "$seen"): nil})
	assertKeywords(t, tg.emails(t, email), email, "$draft", "$flagged")

	// JT-04: a move patches the two membership keys and leaves the
	// message in exactly one mailbox. A message in none is invisible,
	// which is the failure this pins.
	tg.update(t, email, jmap.Patch{
		jmap.Pointer("mailboxIds", string(from)): nil,
		jmap.Pointer("mailboxIds", string(to)):   true,
	})

	moved := tg.emails(t, email)
	if len(moved.List) != 1 {
		t.Fatalf("after the move the server returned %d messages, want 1", len(moved.List))
	}
	if got := slices.Sorted(mapKeys(moved.List[0].MailboxIDs)); len(got) != 1 || got[0] != to {
		t.Errorf("mailboxIds = %v, want exactly [%s]", got, to)
	}
}

// TestConformanceBackReferences covers JT-05 and JT-06 against a real
// server. The chain matters because poplar feeds a query's ids
// straight into a get: a reference that resolved to the wrong set, or
// to an empty one, would read as success.
//
// Three calls, because the second hop's path carries the "*" that
// flattens an array of objects. RFC 8620 section 3.7's own worked
// example reaches that wildcard through Thread/get, which this package
// does not model, so the chain is spelled with two Email/get calls
// instead. The wildcard is the same one, and this is the only place it
// meets a real resolver rather than a fixture.
func TestConformanceBackReferences(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("backref"))
	want := []jmap.ID{
		tg.newEmail(t, mailbox, scratch("backref-1")),
		tg.newEmail(t, mailbox, scratch("backref-2")),
	}

	req := &jmap.Request{}
	queryID := req.Invoke(&jmap.EmailQuery{
		Account: tg.account,
		Filter:  &jmap.EmailFilterCondition{InMailbox: mailbox},
	})
	getID := req.Invoke(&jmap.EmailGet{
		Account:      tg.account,
		ReferenceIDs: &jmap.ResultReference{ResultOf: queryID, Name: "Email/query", Path: "/ids"},
		Properties:   []string{"id"},
	})
	wildcardID := req.Invoke(&jmap.EmailGet{
		Account:      tg.account,
		ReferenceIDs: &jmap.ResultReference{ResultOf: getID, Name: "Email/get", Path: "/list/*/id"},
		Properties:   []string{"id", "subject"},
	})
	resp := tg.do(t, req)

	query := &jmap.EmailQueryResponse{}
	assign(t, only(t, resp, queryID), query)
	fetched := &jmap.EmailGetResponse{}
	assign(t, only(t, resp, getID), fetched)
	flattened := &jmap.EmailGetResponse{}
	assign(t, only(t, resp, wildcardID), flattened)

	if len(query.IDs) != len(want) {
		t.Fatalf("the query matched %v, want the %d messages just filed", query.IDs, len(want))
	}
	got := make([]jmap.ID, len(fetched.List))
	for i, email := range fetched.List {
		got[i] = email.ID
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("the chained get fetched %v, want %v", got, want)
	}

	viaWildcard := make([]jmap.ID, len(flattened.List))
	for i, email := range flattened.List {
		viaWildcard[i] = email.ID
	}
	slices.Sort(viaWildcard)
	if !slices.Equal(viaWildcard, want) {
		t.Errorf("the wildcard hop fetched %v, want %v", viaWildcard, want)
	}

	// JT-06: a reference that does not resolve fails its whole method
	// loudly. An empty list with no error is the failure mode.
	t.Run("broken", func(t *testing.T) {
		req := &jmap.Request{}
		callID := req.Invoke(&jmap.EmailGet{
			Account:      tg.account,
			ReferenceIDs: &jmap.ResultReference{ResultOf: "no-such-call", Name: "Email/query", Path: "/ids"},
		})
		answer := only(t, tg.do(t, req), callID)

		failed, ok := answer.(*jmap.MethodError)
		if !ok {
			t.Fatalf("a broken reference answered %T, want a method error", answer)
		}
		if !errors.Is(failed, jmap.ErrInvalidResultReference) {
			t.Errorf("a broken reference answered %q, want invalidResultReference", failed.Type)
		}
	})
}

// TestConformanceCreationIDs covers JT-08 and JT-09 against a real
// server: a creation id reused across two calls names the second
// record, and a foreign key written as a sharp reference lands on the
// record it names rather than on nothing.
func TestConformanceCreationIDs(t *testing.T) {
	tg := dial(t)

	req := &jmap.Request{Using: []jmap.URI{jmap.MailURI}}
	first := req.Invoke(&jmap.MailboxSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Mailbox{"k1": {Name: scratch("reuse-first")}},
	})
	second := req.Invoke(&jmap.MailboxSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Mailbox{"k1": {Name: scratch("reuse-second")}},
	})
	child := req.Invoke(&jmap.MailboxSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Mailbox{"c": {Name: scratch("reuse-child"), ParentID: "#k1"}},
	})
	// The response echoes the seeded map back with every creation, so
	// seeding one proves the round trip rather than only the echo.
	req.CreatedIDs = map[jmap.ID]jmap.ID{"seed": tg.inbox(t)}
	resp := tg.do(t, req)

	ids := make(map[string]jmap.ID, 3)
	for name, callID := range map[string]string{"first": first, "second": second, "child": child} {
		set := &jmap.MailboxSetResponse{}
		assign(t, only(t, resp, callID), set)
		for key, created := range set.Created {
			if created == nil {
				t.Fatalf("%s create %s failed: %v", name, key, set.NotCreated[key])
			}
			ids[name] = created.ID
			t.Cleanup(func() { tg.destroyMailbox(created.ID) })
		}
	}

	// JT-08: last write wins. A map filled in call order is correct
	// only when it is read after the whole pass.
	parent := tg.mailbox(t, ids["child"]).ParentID
	if parent != ids["second"] {
		t.Errorf("#k1 resolved to %q, want the second create %q (the first was %q)",
			parent, ids["second"], ids["first"])
	}

	// JT-09: the request's seed and every creation come back together.
	if resp.CreatedIDs["seed"] != tg.inbox(t) {
		t.Errorf("createdIds lost the request's seed: %v", resp.CreatedIDs)
	}
	if resp.CreatedIDs["c"] != ids["child"] {
		t.Errorf("createdIds[c] = %q, want %q", resp.CreatedIDs["c"], ids["child"])
	}
}

// TestConformanceChangesPaging covers JT-13, JT-14 and JT-15 against
// real state transitions. A paged /changes sequence is where a
// mailbox drifts out of agreement with the server silently: a record
// reported destroyed before it was reported created is deleted
// locally and never comes back.
func TestConformanceChangesPaging(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("changes"))
	before := tg.emailState(t)

	kept := []jmap.ID{
		tg.newEmail(t, mailbox, scratch("changes-keep-1")),
		tg.newEmail(t, mailbox, scratch("changes-keep-2")),
	}
	// JT-15: a record created and destroyed inside the window. A
	// server may omit it or list it as destroyed, and both have to
	// reach the same place.
	transient := tg.newEmail(t, mailbox, scratch("changes-transient"))
	tg.destroyEmail(t, transient)

	var created, destroyed []jmap.ID
	states := []string{before}
	for since := before; ; {
		page := &jmap.EmailChangesResponse{}
		tg.call(t, &jmap.EmailChanges{Account: tg.account, SinceState: since, MaxChanges: 1}, page)

		// JT-13: nothing may be destroyed before the page that
		// created it. Applied in order, that would leave a live
		// message deleted locally for good.
		for _, id := range page.Destroyed {
			if slices.Contains(kept, id) && !slices.Contains(created, id) {
				t.Errorf("%s was reported destroyed before any page reported it created", id)
			}
		}
		created = append(created, page.Created...)
		destroyed = append(destroyed, page.Destroyed...)

		if page.OldState != since {
			t.Errorf("page answered oldState %q for sinceState %q", page.OldState, since)
		}
		// JT-14: the state advances every page, so the loop ends. A
		// server that repeated one would spin here forever.
		if slices.Contains(states, page.NewState) {
			t.Fatalf("page returned state %q again after %v; the sequence does not advance", page.NewState, states)
		}
		states = append(states, page.NewState)
		since = page.NewState

		if !page.HasMoreChanges {
			break
		}
		if len(states) > 32 {
			t.Fatalf("paging did not terminate in 32 pages, reaching %q", since)
		}
	}

	for _, id := range kept {
		if !slices.Contains(created, id) {
			t.Errorf("%s never appeared in a created page; the pages held %v", id, created)
		}
	}
	// The transient message is the JT-15 case: absent entirely, or
	// present as destroyed. What it must never be is created and left
	// there, which would leave a phantom in the mailbox.
	if slices.Contains(created, transient) && !slices.Contains(destroyed, transient) {
		t.Errorf("%s was reported created and never destroyed, though it no longer exists", transient)
	}
	t.Logf("%s paged %d changes over %d states; the transient message was %s",
		tg.profile.name, len(created)+len(destroyed), len(states)-1, transientOutcome(created, destroyed, transient))
}

func transientOutcome(created, destroyed []jmap.ID, id jmap.ID) string {
	switch {
	case slices.Contains(destroyed, id):
		return "listed as destroyed"
	case slices.Contains(created, id):
		return "listed as created"
	}
	return "omitted"
}

// TestConformanceStaleStateIsLoud covers JT-16 against a real server.
// A state the server cannot answer from must never degrade to an empty
// change set, which reads as "you are caught up" and leaves the
// mailbox stale forever.
//
// cannotCalculateChanges is the one method error poplar branches on,
// so DV-06 requires it tested explicitly rather than assumed. The
// second row is the state that does not even parse, where a server is
// free to answer something else and the protection is only that it
// answers loudly.
func TestConformanceStaleStateIsLoud(t *testing.T) {
	tg := dial(t)

	for _, row := range []struct {
		name  string
		state string
	}{
		{"a well-formed state the server can no longer reach", "s99999999"},
		{"a state the server cannot even parse", "not-a-state-any-server-would-issue!"},
	} {
		t.Run(row.name, func(t *testing.T) {
			req := &jmap.Request{}
			callID := req.Invoke(&jmap.EmailChanges{Account: tg.account, SinceState: row.state})
			answer := only(t, tg.do(t, req), callID)

			if failed, ok := answer.(*jmap.MethodError); ok {
				// Where a server does send the sentinel, errors.Is has
				// to match it, because that match is what forces a
				// resync. RFC 8620 section 3.6.2 folds an unnamed type
				// into serverFail, so the negative check names the
				// sentinel poplar branches on rather than any match.
				if failed.Type == "cannotCalculateChanges" && !errors.Is(failed, jmap.ErrCannotCalculateChanges) {
					t.Error("errors.Is did not match a cannotCalculateChanges the server sent")
				}
				if failed.Type != "cannotCalculateChanges" && errors.Is(failed, jmap.ErrCannotCalculateChanges) {
					t.Errorf("errors.Is matched the resync sentinel against %q", failed.Type)
				}
				t.Logf("%s answers sinceState %q with %q", tg.profile.name, row.state, failed.Type)
				return
			}

			// A server that answers rather than refusing must say
			// which state it paged from (RFC 8620 section 5.2), so a
			// client can tell that its own state was not honoured. The
			// failure this forbids is the silent one: an empty diff
			// echoing the state that was sent, which reads as "you are
			// caught up" and leaves the mailbox stale forever.
			page := &jmap.EmailChangesResponse{}
			assign(t, answer, page)
			if page.OldState == row.state && len(page.Created)+len(page.Updated)+len(page.Destroyed) == 0 {
				t.Fatalf("sinceState %q came back as an empty change set from that very state", row.state)
			}
			t.Logf("DIVERGENCE: %s accepted sinceState %q and paged from %q instead of refusing it",
				tg.profile.name, row.state, page.OldState)
		})
	}
}

// TestConformanceDigitOnlyIDsCannotBePatched pins what happens when a
// server allocates an id RFC 8620 section 1.2 recommends against. A
// patch pointer cannot tell a map key of digits from an array index,
// section 5.3 forbids the index, and [jmap.Pointer] sees only the
// shape. poplar therefore refuses the patch at its own boundary rather
// than sending one a lenient server might apply to an array.
//
// The test asserts the refusal and reports whether this server's ids
// make it reachable, which on Stalwart they do: its alphabet reaches
// the digits after the twenty-six letters.
func TestConformanceDigitOnlyIDsCannotBePatched(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("digits"))
	email := tg.newEmail(t, mailbox, scratch("digits"))

	req := &jmap.Request{}
	req.Invoke(&jmap.EmailSet{
		Account: tg.account,
		Update:  map[jmap.ID]jmap.Patch{email: {jmap.Pointer("mailboxIds", "3"): true}},
	})
	if _, err := tg.client.Do(t.Context(), req); err == nil {
		t.Error("a patch addressing mailboxIds/3 reached the wire; RFC 8620 section 5.3 forbids the form")
	}

	// Whether the case is reachable is a fact about the server's id
	// allocation, so it is measured rather than assumed: ask for
	// mailboxes until one comes back with an id no patch can address.
	for attempt := range 40 {
		set := &jmap.MailboxSetResponse{}
		tg.call(t, &jmap.MailboxSet{
			Account: tg.account,
			Create:  map[jmap.ID]*jmap.Mailbox{"m": {Name: scratch("digits-probe")}},
		}, set)
		made := set.Created["m"]
		if made == nil {
			t.Fatalf("create a probe mailbox: %v", set.NotCreated["m"])
		}
		t.Cleanup(func() { tg.destroyMailbox(made.ID) })

		if digitsOnly(string(made.ID)) {
			t.Logf("DIVERGENCE: %s allocated mailbox id %q after %d records, so no message can be moved into that mailbox by patch",
				tg.profile.name, made.ID, attempt+1)
			return
		}
	}
	t.Logf("%s allocated no digit-only mailbox id across forty records", tg.profile.name)
}

// TestConformanceQueryChangesSplice covers JT-17 against a real
// server, and is [jmap.Splice]'s consumer: the splice math is applied
// to a real /queryChanges result and the outcome is compared against
// the query the server would answer now. An off-by-one duplicates or
// drops a row in a mailbox listing, which reads as a rendering bug.
func TestConformanceQueryChangesSplice(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("splice"))
	for i := range 3 {
		tg.newEmail(t, mailbox, scratch("splice-seed")+string(rune('a'+i)))
	}

	filter := &jmap.EmailFilterCondition{InMailbox: mailbox}
	sort := []*jmap.Comparator{{Property: "receivedAt"}}

	cached := &jmap.EmailQueryResponse{}
	tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: filter, Sort: sort}, cached)
	if !cached.CanCalculateChanges {
		t.Skipf("%s cannot calculate query changes for this query", tg.profile.name)
	}

	// One message in and one out, so the splice has both halves to
	// apply rather than only an insertion.
	added := tg.newEmail(t, mailbox, scratch("splice-added"))
	tg.destroyEmail(t, cached.IDs[0])

	changes := &jmap.EmailQueryChangesResponse{}
	tg.call(t, &jmap.EmailQueryChanges{
		Account:         tg.account,
		Filter:          filter,
		Sort:            sort,
		SinceQueryState: cached.QueryState,
	}, changes)

	spliced, err := jmap.Splice(cached.IDs, changes.Removed, changes.Added)
	if err != nil {
		t.Fatalf("Splice(%v, %v, %v): %v", cached.IDs, changes.Removed, changes.Added, err)
	}

	fresh := &jmap.EmailQueryResponse{}
	tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: filter, Sort: sort}, fresh)
	if !slices.Equal(spliced, fresh.IDs) {
		t.Errorf("Splice produced\n%v\nbut the server now answers\n%v\n(cached %v, removed %v, added %v)",
			spliced, fresh.IDs, cached.IDs, changes.Removed, changes.Added)
	}
	if !slices.Contains(spliced, added) {
		t.Errorf("the spliced list %v does not hold the message added since the cache", spliced)
	}
	if changes.NewQueryState == changes.OldQueryState {
		t.Errorf("the query state stayed %q across a change to the results", changes.NewQueryState)
	}
}

// TestConformanceSessionRefetch covers JT-21's atomicity half against
// a real server: a refetch replaces the session in hand wholesale, and
// every response names the session state a caller compares against.
func TestConformanceSessionRefetch(t *testing.T) {
	tg := dial(t)

	req := &jmap.Request{}
	req.Invoke(jmap.Echo{"conformance": true})
	resp := tg.do(t, req)
	if resp.SessionState == "" {
		t.Fatal("the response carries no sessionState, so nothing would ever trigger a refetch")
	}
	if resp.SessionState != tg.session.State {
		t.Errorf("the response names session state %q while the session in hand says %q",
			resp.SessionState, tg.session.State)
	}

	refetched, err := tg.client.FetchSession(t.Context())
	if err != nil {
		t.Fatalf("FetchSession: %v", err)
	}
	if refetched == tg.session {
		t.Error("FetchSession handed back the session it already held")
	}
	if tg.client.Session() != refetched {
		t.Error("the refetched session did not become the one in hand")
	}
	if refetched.APIURL != tg.session.APIURL || refetched.State != tg.session.State {
		t.Errorf("the refetched session disagrees with the first: %q/%q against %q/%q",
			refetched.APIURL, refetched.State, tg.session.APIURL, tg.session.State)
	}
}

// TestConformancePingReportsItsOwnCadence covers JT-26 against a real
// server. poplar's liveness timer is set from the interval the ping
// carries, so a server that clamps the request has to be believed and
// a payload in the wrong unit disables the detector outright.
func TestConformancePingReportsItsOwnCadence(t *testing.T) {
	tg := dial(t)

	events := tg.readEventStream(t, "types=*&closeafter=no&ping=2", 1)
	if len(events) == 0 {
		t.Fatal("the event source sent nothing within the window; no ping means no liveness signal at all")
	}
	if events[0].name != "ping" {
		t.Fatalf("the first event was %q, want a ping for a stream that asked for one", events[0].name)
	}

	var payload struct {
		Interval int64 `json:"interval"`
	}
	if err := json.Unmarshal([]byte(events[0].data), &payload); err != nil {
		t.Fatalf("decode the ping payload %q: %v", events[0].data, err)
	}
	if payload.Interval <= 0 {
		t.Fatalf("the ping reported interval %d, which says nothing about when to expect the next one", payload.Interval)
	}

	// RFC 8620 section 7.3 puts the interval in seconds and forbids a
	// server from setting a maximum below 300, so a figure far above
	// that is the server answering in another unit. poplar sets its
	// stall window from this number, so the consequence is a detector
	// that never fires.
	if payload.Interval > 300 {
		t.Logf("DIVERGENCE: %s reports a ping interval of %d, which RFC 8620 section 7.3 reads as %d seconds; "+
			"a client that believes it sets a stall window of %s",
			tg.profile.name, payload.Interval, payload.Interval, 2*time.Duration(payload.Interval)*time.Second)
	}
}

// TestConformanceUnknownMethodIsTyped covers JT-31 against a real
// server: a method name the server does not implement comes back as a
// typed error at the right call id rather than as a missing response.
func TestConformanceUnknownMethodIsTyped(t *testing.T) {
	tg := dial(t)

	body := tg.raw(t, `{"using":["urn:ietf:params:jmap:core"],`+
		`"methodCalls":[["Core/echo",{"first":true},"0"],["Nonexistent/method",{},"1"]]}`)

	resp := &jmap.Response{}
	if err := json.Unmarshal(body, resp); err != nil {
		t.Fatalf("decode the server's own bytes: %v", err)
	}

	if len(resp.Invocations("0")) != 1 {
		t.Errorf("the call before the unknown one lost its response")
	}
	answers := resp.Invocations("1")
	if len(answers) != 1 {
		t.Fatalf("the unknown method answered %d times, want once", len(answers))
	}
	failed, ok := answers[0].Args.(*jmap.MethodError)
	if !ok {
		t.Fatalf("the unknown method answered %T, want a method error", answers[0].Args)
	}
	if failed.Type == "" {
		t.Errorf("the error carries no type: %s", failed.Raw)
	}
	if len(failed.Raw) == 0 {
		t.Error("the error dropped the object the server sent")
	}
}

// TestConformanceBlobRoundTrip covers JT-32 against a real server:
// upload, download, and the copy whose notCopied map is the half a
// caller reading only the method's error return never sees.
func TestConformanceBlobRoundTrip(t *testing.T) {
	tg := dial(t)

	const payload = "poplar conformance blob\n"
	uploaded := tg.upload(t, "text/plain", payload)
	if uploaded.Size != uint64(len(payload)) {
		t.Errorf("the server recorded size %d for %d bytes", uploaded.Size, len(payload))
	}
	if uploaded.Type != "text/plain" {
		t.Errorf("the server recorded type %q, want the one the upload sent", uploaded.Type)
	}
	if uploaded.Account != tg.account {
		t.Errorf("the server recorded account %q, want %q", uploaded.Account, tg.account)
	}
	if got := tg.download(t, uploaded.BlobID); got != payload {
		t.Errorf("download returned %q, want %q", got, payload)
	}

	// Blob/copy sits in RFC 8620 section 6.3, so this package asks for
	// no capability beyond core. A server that puts it behind the RFC
	// 9404 blob capability, as Stalwart does, is served by reading the
	// session rather than by the package guessing.
	using := []jmap.URI{jmap.CoreURI}
	if _, offered := tg.session.RawCapabilities[blobURI]; offered {
		using = append(using, blobURI)
	}

	req := &jmap.Request{Using: using}
	callID := req.Invoke(&jmap.BlobCopy{
		From:    tg.account,
		Account: tg.account,
		BlobIDs: []jmap.ID{uploaded.BlobID, "no-such-blob-in-this-account"},
	})
	answer := only(t, tg.do(t, req), callID)
	if failed, ok := answer.(*jmap.MethodError); ok {
		t.Fatalf("Blob/copy: %v", failed)
	}

	copied := &jmap.BlobCopyResponse{}
	assign(t, answer, copied)
	if copied.Account != tg.account || copied.From != tg.account {
		t.Errorf("Blob/copy answered %q -> %q, want %q -> %q", copied.From, copied.Account, tg.account, tg.account)
	}
	if _, ok := copied.Copied[uploaded.BlobID]; !ok {
		t.Errorf("the blob that exists is not in copied: %v", copied.Copied)
	}
	refused := copied.NotCopied["no-such-blob-in-this-account"]
	if refused == nil {
		t.Fatalf("the blob that does not exist was not refused: %v", copied.NotCopied)
	}
	if refused.Type == "" {
		t.Errorf("notCopied carries no type: %s", refused.Raw)
	}
	if len(refused.Raw) == 0 {
		t.Error("notCopied dropped the object the server sent")
	}
}

// TestConformanceEmailCreateConstraints covers JT-33 against a real
// server. Each row is a constraint RFC 8621 section 4.6 puts on a
// created message, and each has to come back as a typed refusal
// naming the property rather than as a message the account does not
// hold.
func TestConformanceEmailCreateConstraints(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("constraints"))
	body := map[string]*jmap.BodyValue{"b": {Value: "constraint probe\n"}}

	for _, row := range []struct {
		name  string
		email *jmap.Email
	}{
		{
			// A message in no mailbox is invisible, which is why the
			// server refuses rather than filing it nowhere.
			name:  "no mailbox",
			email: &jmap.Email{Subject: scratch("no-mailbox"), BodyValues: body, TextBody: []*jmap.BodyPart{{PartID: "b", Type: "text/plain"}}},
		},
		{
			name: "blobId and partId on one part",
			email: &jmap.Email{
				MailboxIDs: map[jmap.ID]bool{mailbox: true},
				Subject:    scratch("both-ids"),
				BodyValues: body,
				TextBody:   []*jmap.BodyPart{{PartID: "b", BlobID: "no-such-blob", Type: "text/plain"}},
			},
		},
		{
			name: "a part naming a blob the account does not hold",
			email: &jmap.Email{
				MailboxIDs: map[jmap.ID]bool{mailbox: true},
				Subject:    scratch("missing-blob"),
				TextBody:   []*jmap.BodyPart{{BlobID: "no-such-blob", Type: "text/plain"}},
			},
		},
		{
			name: "a mailbox the account does not hold",
			email: &jmap.Email{
				MailboxIDs: map[jmap.ID]bool{"no-such-mailbox": true},
				Subject:    scratch("missing-mailbox"),
				BodyValues: body,
				TextBody:   []*jmap.BodyPart{{PartID: "b", Type: "text/plain"}},
			},
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			resp := &jmap.EmailSetResponse{}
			tg.call(t, &jmap.EmailSet{
				Account: tg.account,
				Create:  map[jmap.ID]*jmap.Email{"e": row.email},
			}, resp)

			if created := resp.Created["e"]; created != nil {
				t.Cleanup(func() { tg.destroyEmailQuietly(created.ID) })
				t.Fatalf("the server accepted the message as %s", created.ID)
			}
			refused := resp.NotCreated["e"]
			if refused == nil {
				t.Fatal("the server neither created the message nor said why")
			}
			if refused.Type == "" {
				t.Errorf("the refusal carries no type: %s", refused.Raw)
			}
			if refused.Type == "invalidProperties" && len(refused.Properties) == 0 {
				t.Errorf("invalidProperties named no property: %s", refused.Raw)
			}
			t.Logf("%s answers %q %v", tg.profile.name, refused.Type, refused.Properties)
		})
	}
}

// TestConformanceMailboxDestroyErrors covers JT-34 against a real
// server. Both refusals have to arrive typed and leave the mailbox
// standing, because a client that read either as success would drop
// the folder and its mail from its own view.
func TestConformanceMailboxDestroyErrors(t *testing.T) {
	tg := dial(t)

	parent := tg.newMailbox(t, scratch("destroy-parent"))
	childName := scratch("destroy-child")
	set := &jmap.MailboxSetResponse{}
	tg.call(t, &jmap.MailboxSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Mailbox{"c": {Name: childName, ParentID: parent}},
	}, set)
	child := set.Created["c"]
	if child == nil {
		t.Fatalf("create the child mailbox: %v", set.NotCreated["c"])
	}
	t.Cleanup(func() { tg.destroyMailbox(child.ID) })
	tg.newEmail(t, child.ID, scratch("destroy-message"))

	for _, row := range []struct {
		name    string
		mailbox jmap.ID
		want    string
	}{
		{"a mailbox with a child", parent, "mailboxHasChild"},
		{"a mailbox with a message", child.ID, "mailboxHasEmail"},
	} {
		t.Run(row.name, func(t *testing.T) {
			resp := &jmap.MailboxSetResponse{}
			tg.call(t, &jmap.MailboxSet{Account: tg.account, Destroy: []jmap.ID{row.mailbox}}, resp)

			if slices.Contains(resp.Destroyed, row.mailbox) {
				t.Fatalf("the server destroyed %s", row.mailbox)
			}
			refused := resp.NotDestroyed[row.mailbox]
			if refused == nil {
				t.Fatal("the server neither destroyed the mailbox nor said why")
			}
			if refused.Type != row.want {
				t.Errorf("the refusal is %q, want %q per RFC 8621 section 10.6", refused.Type, row.want)
			}
			// The mailbox is still there, which is the part a client
			// reading only the error return would get wrong.
			if tg.mailbox(t, row.mailbox) == nil {
				t.Errorf("%s is gone despite the refusal", row.mailbox)
			}
		})
	}
}

// TestConformanceDatesAreUTC covers JT-35 against a real server: a
// date built in a non-UTC location reaches the server as the instant
// the caller meant, and comes back as the same instant.
func TestConformanceDatesAreUTC(t *testing.T) {
	tg := dial(t)

	// A zone with a fractional offset, so an implementation that
	// dropped the offset lands on a different instant rather than on a
	// whole hour that could pass by luck.
	kathmandu := time.FixedZone("Asia/Kathmandu", 5*3600+45*60)
	want := time.Date(2026, time.March, 4, 9, 15, 30, 0, kathmandu)
	received := jmap.Date(want)

	mailbox := tg.newMailbox(t, scratch("dates"))
	resp := &jmap.EmailSetResponse{}
	email := tg.draft(mailbox, scratch("dates"), nil)
	email.ReceivedAt = &received
	tg.call(t, &jmap.EmailSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Email{"e": email},
	}, resp)
	if resp.Created["e"] == nil {
		t.Fatalf("create: %v", resp.NotCreated["e"])
	}

	fetched := tg.emails(t, resp.Created["e"].ID)
	if len(fetched.List) != 1 || fetched.List[0].ReceivedAt == nil {
		t.Fatalf("the server returned no receivedAt: %+v", fetched.List)
	}
	if got := fetched.List[0].ReceivedAt.Time(); !got.Equal(want) {
		t.Errorf("receivedAt came back %s, want the same instant as %s", got, want)
	}
	if zone, _ := fetched.List[0].ReceivedAt.Time().Zone(); zone != "UTC" {
		t.Errorf("receivedAt decoded in zone %q, want UTC", zone)
	}
}

// TestConformanceQueryArguments covers JT-42, JT-43, JT-44 and JT-45
// against a real server. Each argument is one poplar sends on every
// mailbox listing, and each is asserted against the messages the
// server actually returns rather than against a marshalled request.
func TestConformanceQueryArguments(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("query"))
	var seeded []jmap.ID
	for i := range 4 {
		keywords := map[string]bool{"$seen": true}
		if i%2 == 0 {
			keywords["$flagged"] = true
		}
		// Distinct receipt times, an hour apart. Messages filed within
		// one second of each other tie on receivedAt, and a sort with
		// every row tied would agree with its own reverse.
		received := jmap.Date(time.Date(2026, time.January, 2, 3+i, 0, 0, 0, time.UTC))
		email := tg.draft(mailbox, scratch("query-seed"), keywords)
		email.ReceivedAt = &received

		resp := &jmap.EmailSetResponse{}
		tg.call(t, &jmap.EmailSet{
			Account: tg.account,
			Create:  map[jmap.ID]*jmap.Email{"e": email},
		}, resp)
		if resp.Created["e"] == nil {
			t.Fatalf("seed %d: %v", i, resp.NotCreated["e"])
		}
		seeded = append(seeded, resp.Created["e"].ID)
	}
	inMailbox := &jmap.EmailFilterCondition{InMailbox: mailbox}
	byAge := []*jmap.Comparator{{Property: "receivedAt"}}

	t.Run("isAscending defaults to true", func(t *testing.T) {
		// JT-42. A Go bool with no omitempty would send false here and
		// silently reverse every unconsidered sort.
		ascending := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: inMailbox, Sort: byAge}, ascending)

		descending := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account: tg.account,
			Filter:  inMailbox,
			Sort:    []*jmap.Comparator{{Property: "receivedAt", IsAscending: new(false)}},
		}, descending)

		reversed := slices.Clone(descending.IDs)
		slices.Reverse(reversed)
		if !slices.Equal(ascending.IDs, reversed) {
			t.Errorf("the default order %v is not the reverse of the explicit descending order %v",
				ascending.IDs, descending.IDs)
		}
	})

	t.Run("compound filters nest", func(t *testing.T) {
		// JT-43. The nesting has to reach the server intact, so the
		// assertion is on which messages come back.
		flagged := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account: tg.account,
			Filter: &jmap.FilterOperator{Operator: jmap.OperatorAND, Conditions: []jmap.Filter{
				inMailbox,
				&jmap.FilterOperator{Operator: jmap.OperatorNOT, Conditions: []jmap.Filter{
					&jmap.EmailFilterCondition{HasKeyword: "$flagged"},
				}},
			}},
			Sort: byAge,
		}, flagged)

		if len(flagged.IDs) != 2 {
			t.Fatalf("AND(inMailbox, NOT(hasKeyword)) matched %v, want the two unflagged messages", flagged.IDs)
		}
		for _, id := range flagged.IDs {
			if !slices.Contains(seeded, id) {
				t.Errorf("the nested filter matched %s, which this test did not file", id)
			}
		}
	})

	t.Run("anchor paging", func(t *testing.T) {
		// JT-44. An anchor pages from a known id, so a concurrent
		// change cannot shift the window under the client.
		all := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{Account: tg.account, Filter: inMailbox, Sort: byAge}, all)

		anchored := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account:      tg.account,
			Filter:       inMailbox,
			Sort:         byAge,
			Anchor:       all.IDs[2],
			AnchorOffset: -1,
		}, anchored)

		if !slices.Equal(anchored.IDs, all.IDs[1:]) {
			t.Errorf("anchoring at %s with offset -1 returned %v, want %v",
				all.IDs[2], anchored.IDs, all.IDs[1:])
		}
		if anchored.Position != 1 {
			t.Errorf("the anchored query reports position %d, want 1", anchored.Position)
		}
	})

	t.Run("anchorNotFound", func(t *testing.T) {
		req := &jmap.Request{}
		callID := req.Invoke(&jmap.EmailQuery{
			Account: tg.account,
			Filter:  inMailbox,
			Sort:    byAge,
			Anchor:  "no-such-message",
		})
		answer := only(t, tg.do(t, req), callID)

		failed, ok := answer.(*jmap.MethodError)
		if !ok {
			t.Fatalf("an unusable anchor answered %T, want a method error", answer)
		}
		t.Logf("%s answers an unusable anchor with %q (RFC 8620 section 5.5 names anchorNotFound)",
			tg.profile.name, failed.Type)
	})

	t.Run("collapseThreads", func(t *testing.T) {
		// JT-45. Sending the argument is poplar's part; whether the
		// server honours it is DV-07's.
		collapsed := &jmap.EmailQueryResponse{}
		tg.call(t, &jmap.EmailQuery{
			Account:         tg.account,
			Filter:          inMailbox,
			Sort:            byAge,
			CollapseThreads: true,
		}, collapsed)

		for _, id := range collapsed.IDs {
			if !slices.Contains(seeded, id) {
				t.Errorf("the collapsed query matched %s, which this test did not file", id)
			}
		}
		if len(collapsed.IDs) > len(seeded) {
			t.Errorf("collapsing threads returned %d messages out of %d", len(collapsed.IDs), len(seeded))
		}
	})
}

// TestConformanceThreadsAreTheServersToCompute covers JT-46. RFC 8621
// section 3 makes the grouping heuristic a SHOULD, so two conformant
// servers may thread one mailbox differently and neither is wrong.
// poplar reports what the server decided, byte for byte, and computes
// nothing of its own.
func TestConformanceThreadsAreTheServersToCompute(t *testing.T) {
	tg := dial(t)

	mailbox := tg.newMailbox(t, scratch("threads"))
	subject := scratch("threads-subject")

	first := tg.draft(mailbox, subject, nil)
	first.MessageID = []string{"poplar-conformance-root@conformance.test"}
	reply := tg.draft(mailbox, "Re: "+subject, nil)
	reply.MessageID = []string{"poplar-conformance-reply@conformance.test"}
	reply.InReplyTo = first.MessageID
	reply.References = first.MessageID

	resp := &jmap.EmailSetResponse{}
	tg.call(t, &jmap.EmailSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Email{"a": first, "b": reply},
	}, resp)
	if resp.Created["a"] == nil || resp.Created["b"] == nil {
		t.Fatalf("create: %v", resp.NotCreated)
	}
	ids := []jmap.ID{resp.Created["a"].ID, resp.Created["b"].ID}

	fetched := tg.emails(t, ids...)
	if len(fetched.List) != 2 {
		t.Fatalf("the server returned %d messages, want 2", len(fetched.List))
	}

	// The thread ids poplar reports are the ones on the wire. Nothing
	// here recomputes the grouping, so the only claim worth making is
	// that the server's answer survives unchanged.
	body := tg.raw(t, `{"using":["urn:ietf:params:jmap:core","urn:ietf:params:jmap:mail"],`+
		`"methodCalls":[["Email/get",{"accountId":"`+string(tg.account)+`","ids":["`+
		string(ids[0])+`","`+string(ids[1])+`"],"properties":["id","threadId"]},"0"]]}`)

	var args struct {
		List []struct {
			ID       jmap.ID `json:"id"`
			ThreadID jmap.ID `json:"threadId"`
		} `json:"list"`
	}
	if err := json.Unmarshal(argumentsOf(t, body, 0), &args); err != nil {
		t.Fatalf("decode the server's own bytes: %v", err)
	}

	onWire := make(map[jmap.ID]jmap.ID, 2)
	for _, email := range args.List {
		onWire[email.ID] = email.ThreadID
	}
	for _, email := range fetched.List {
		if email.ThreadID == "" {
			t.Errorf("%s carries no threadId", email.ID)
		}
		if email.ThreadID != onWire[email.ID] {
			t.Errorf("%s reports thread %q while the wire says %q", email.ID, email.ThreadID, onWire[email.ID])
		}
	}
	t.Logf("%s grouped a message and its reply into %s and %s",
		tg.profile.name, onWire[ids[0]], onWire[ids[1]])
}

// assertKeywords compares the keywords the server holds against the
// whole set the caller expects, because JT-02's failure is a patch
// that removed the keywords it was not asked about.
func assertKeywords(t *testing.T, resp *jmap.EmailGetResponse, id jmap.ID, want ...string) {
	t.Helper()

	if len(resp.List) != 1 || resp.List[0].ID != id {
		t.Fatalf("the server returned %d messages for %s", len(resp.List), id)
	}
	got := slices.Sorted(mapKeys(resp.List[0].Keywords))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("keywords = %v, want %v", got, want)
	}
}
