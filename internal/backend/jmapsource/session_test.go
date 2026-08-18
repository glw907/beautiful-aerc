package jmapsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/jmap"
)

// sessionTemplate is a JMAP session resource: Core and Mail
// capabilities plus a submission capability with a positive
// maxDelayedSend, so probeCapabilities has something to read for
// every field it populates. downloadUrl carries {type} and {name}
// (not only {blobId}), so downloadBlob's own values for both reach
// the wire where fakeBlobs.handleDownload (mail_test.go) can record
// them; a URL template with no such placeholder makes a value change
// there invisible to every test, which is exactly how the download
// content type going from application/octet-stream to message/rfc822
// and the download name going from "filename" to "" both landed
// unnoticed in this package's cutover from go-jmap.
const sessionTemplate = `{
  "capabilities": {
    "urn:ietf:params:jmap:core": {
      "maxSizeUpload": 50000000,
      "maxConcurrentUpload": 4,
      "maxSizeRequest": 10000000,
      "maxConcurrentRequests": 8,
      "maxCallsInRequest": 32,
      "maxObjectsInGet": 750,
      "maxObjectsInSet": 750
    },
    "urn:ietf:params:jmap:mail": {},
    "urn:ietf:params:jmap:submission": {"maxDelayedSend": 44236800}
  },
  "accounts": {"u1": {"name": "geoff@907.life"}},
  "primaryAccounts": {
    "urn:ietf:params:jmap:mail": "u1",
    "urn:ietf:params:jmap:submission": "u1"
  },
  "username": "geoff@907.life",
  "apiUrl": "%[1]s/api",
  "downloadUrl": "%[1]s/download/{blobId}/{type}/{name}",
  "uploadUrl": "%[1]s/upload/{accountId}/",
  "eventSourceUrl": "%[1]s/events",
  "state": "session-1"
}`

// fakeAPI scripts one JMAP server's /api endpoint: each call to
// handle serves the next response in order and records the decoded
// request body for the test to assert against.
type fakeAPI struct {
	mu        sync.Mutex
	responses [][]byte
	requests  []map[string]any
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f.mu.Lock()
	f.requests = append(f.requests, decoded)
	idx := len(f.requests) - 1
	f.mu.Unlock()

	if idx >= len(f.responses) {
		http.Error(w, fmt.Sprintf("fakeAPI: no scripted response for call %d", idx), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(f.responses[idx])
}

func (f *fakeAPI) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeAPI) requestAt(i int) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[i]
}

// methodCall reads one JSON-decoded methodCalls[i] triple as
// (name, args, callID).
func methodCall(t *testing.T, req map[string]any, i int) (string, map[string]any, string) {
	t.Helper()
	calls, ok := req["methodCalls"].([]any)
	if !ok || i >= len(calls) {
		t.Fatalf("methodCalls[%d]: not present in %v", i, req)
	}
	call, ok := calls[i].([]any)
	if !ok || len(call) != 3 {
		t.Fatalf("methodCalls[%d]: malformed %v", i, calls[i])
	}
	name, _ := call[0].(string)
	args, _ := call[1].(map[string]any)
	callID, _ := call[2].(string)
	return name, args, callID
}

// newFakeServer starts an httptest server serving sessionTemplate for
// /session and returns its mux so a test can register whatever else
// it needs (/api, /upload/, /download/) before dialing.
func newFakeServer(t *testing.T) (*http.ServeMux, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, sessionTemplate, srv.URL)
	})
	return mux, srv
}

func dialTestSession(t *testing.T, srv *httptest.Server) *Session {
	t.Helper()
	session, err := Dial(context.Background(), srv.URL+"/session", NewStaticCredentials("test-token"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return session
}

// newTestSession dials a Session against an httptest server that
// serves sessionTemplate for /session and plays responses back in
// order for /api.
func newTestSession(t *testing.T, responses ...[]byte) (*Session, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{responses: responses}
	mux, srv := newFakeServer(t)
	mux.HandleFunc("/api", api.handle)
	return dialTestSession(t, srv), api
}

func TestDialProbesCapabilities(t *testing.T) {
	session, _ := newTestSession(t)
	caps := session.Capabilities()

	if caps.Limits.MaxObjectsInGet != 750 {
		t.Errorf("MaxObjectsInGet = %d, want 750", caps.Limits.MaxObjectsInGet)
	}
	if caps.Limits.MaxObjectsInSet != 750 {
		t.Errorf("MaxObjectsInSet = %d, want 750", caps.Limits.MaxObjectsInSet)
	}
	if caps.Limits.MaxCallsInRequest != 32 {
		t.Errorf("MaxCallsInRequest = %d, want 32", caps.Limits.MaxCallsInRequest)
	}
	if caps.Limits.MaxConcurrentRequests != 8 {
		t.Errorf("MaxConcurrentRequests = %d, want 8", caps.Limits.MaxConcurrentRequests)
	}
	if caps.Limits.MaxSizeUpload != 50000000 {
		t.Errorf("MaxSizeUpload = %d, want 50000000", caps.Limits.MaxSizeUpload)
	}
	if !caps.ScheduledSend {
		t.Error("ScheduledSend = false, want true (maxDelayedSend > 0)")
	}
	if caps.PushTransport != backend.PushTransportEventSource {
		t.Errorf("PushTransport = %v, want PushTransportEventSource", caps.PushTransport)
	}
	if caps.AccountIDs["mail"] != "u1" {
		t.Errorf("AccountIDs[mail] = %q, want u1", caps.AccountIDs["mail"])
	}
}

// TestDialHonorsContextTimeout asserts Dial returns once its context
// expires against a session endpoint that never responds, rather
// than blocking forever: go-jmap's own Client.Authenticate builds its
// request with http.NewRequest and no context, so Dial fetches the
// session resource itself, ctx-bound.
func TestDialHonorsContextTimeout(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := Dial(ctx, srv.URL+"/session", NewStaticCredentials("tok"))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Dial: want an error from a hung session endpoint, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Dial did not return within 2s of its 20ms context timeout expiring")
	}
}

// TestDialClassifiesRejectedSessionWithoutLogging asserts a non-200
// status from the session endpoint itself (a rejection with no
// problem-details body, so package jmap's refusal builds a bare
// *jmap.HTTPError rather than a *jmap.RequestError) still reaches the
// caller classified, as a backend.Failure, not a bare fmt.Errorf, and
// without having logged: Dial is retried by its own caller's backoff
// loop (cmd/poplar's retryConnect), and a uerr.Error constructed here
// would write a log line on every attempt rather than only on a state
// transition (ADR-0013 revision 2).
func TestDialClassifiesRejectedSessionWithoutLogging(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := Dial(context.Background(), srv.URL+"/session", NewStaticCredentials("bad-token"))
	failure, ok := errors.AsType[backend.Failure](err)
	if !ok {
		t.Fatalf("Dial error = %v, want a backend.Failure in the chain", err)
	}
	if failure.Class != uerr.ClassAuth {
		t.Errorf("Class = %v, want ClassAuth", failure.Class)
	}
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		t.Errorf("Dial error carries a uerr.Error (%+v), want none: classification here must not log", ue)
	}
}

// TestDialClassifiesDeadConnectionWithoutLogging asserts a session
// endpoint that refuses the connection outright (nothing listening)
// also classifies as a backend.Failure rather than a bare network error,
// and without logging, the same discipline
// TestDialClassifiesRejectedSessionWithoutLogging proves for a
// rejected status.
func TestDialClassifiesDeadConnectionWithoutLogging(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	deadURL := srv.URL + "/session"
	srv.Close() // nothing listens on deadURL from here on

	_, err := Dial(context.Background(), deadURL, NewStaticCredentials("tok"))
	failure, ok := errors.AsType[backend.Failure](err)
	if !ok {
		t.Fatalf("Dial error = %v, want a backend.Failure in the chain", err)
	}
	if failure.Class != uerr.ClassConnection {
		t.Errorf("Class = %v, want ClassConnection", failure.Class)
	}
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		t.Errorf("Dial error carries a uerr.Error (%+v), want none: classification here must not log", ue)
	}
}

func TestAuthTransportSendsCredentialToken(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = fmt.Fprintf(w, sessionTemplate, srv.URL)
	})

	_, err := Dial(context.Background(), srv.URL+"/session", NewStaticCredentials("s3cr3t"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if want := "Bearer s3cr3t"; gotAuth != want {
		t.Errorf("Authorization header = %q, want %q", gotAuth, want)
	}
}

// newStateServer serves a session resource whose state the test moves,
// counting fetches, and an /api that answers every call with body.
func newStateServer(t *testing.T, body func() string) (*httptest.Server, func() int) {
	t.Helper()

	var mu sync.Mutex
	fetches := 0
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		fetches++
		mu.Unlock()
		_, _ = fmt.Fprintf(w, `{"capabilities":{},"accounts":{},"primaryAccounts":{"urn:ietf:params:jmap:mail":"u1"},"apiUrl":%q,"state":"session-1"}`, srv.URL+"/api")
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body())
	})
	return srv, func() int {
		mu.Lock()
		defer mu.Unlock()
		return fetches
	}
}

// TestSessionRefetchesOncePerStateItMovesTo covers JT-21's trigger
// half. RFC 8620 section 2 puts the session state on every response
// and recommends refetching when it differs from the one in hand;
// which responses that adds up to is policy, and belongs here rather
// than in the library. Every response carries the state, so a run of
// responses reporting the same new one must cost one refetch, not one
// each: a refetch storm on a busy account is the failure mode.
func TestSessionRefetchesOncePerStateItMovesTo(t *testing.T) {
	var mu sync.Mutex
	state := "s2"
	srv, fetches := newStateServer(t, func() string {
		mu.Lock()
		defer mu.Unlock()
		return `{"methodResponses":[],"sessionState":"` + state + `"}`
	})
	session := dialTestSession(t, srv)

	if got := fetches(); got != 1 {
		t.Fatalf("session fetches after the dial = %d, want 1", got)
	}

	for range 3 {
		if _, err := session.do(t.Context(), &jmap.Request{}); err != nil {
			t.Fatalf("do: %v", err)
		}
	}
	if got := fetches(); got != 2 {
		t.Fatalf("session fetches after three responses reporting one new state = %d, want 2 (the dial and one refetch)", got)
	}

	mu.Lock()
	state = "s3"
	mu.Unlock()
	if _, err := session.do(t.Context(), &jmap.Request{}); err != nil {
		t.Fatalf("do: %v", err)
	}
	if got := fetches(); got != 3 {
		t.Errorf("session fetches after the state moved again = %d, want 3: a later move must still be followed", got)
	}
}

// TestSessionRefetchesOnceUnderConcurrentCalls is the same claim where
// it is load-bearing. The sync worker and the outbox dispatcher call
// through one Session at once, so the responses that first report a
// new state arrive together, and the count has to hold across them
// rather than only in sequence.
func TestSessionRefetchesOnceUnderConcurrentCalls(t *testing.T) {
	srv, fetches := newStateServer(t, func() string {
		return `{"methodResponses":[],"sessionState":"s2"}`
	})
	session := dialTestSession(t, srv)

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := session.do(t.Context(), &jmap.Request{}); err != nil {
				t.Errorf("do: %v", err)
			}
		})
	}
	wg.Wait()

	if got := fetches(); got != 2 {
		t.Errorf("session fetches across 8 concurrent responses reporting one new state = %d, want 2 (the dial and one refetch)", got)
	}
}

// TestSessionDoesNotRefetchOnTheStateItDialedWith keeps the trigger
// from firing on the ordinary case. Every response carries the state,
// so a session whose state has not moved would otherwise refetch on
// every call poplar makes.
func TestSessionDoesNotRefetchOnTheStateItDialedWith(t *testing.T) {
	srv, fetches := newStateServer(t, func() string {
		return `{"methodResponses":[],"sessionState":"session-1"}`
	})
	session := dialTestSession(t, srv)

	for range 5 {
		if _, err := session.do(t.Context(), &jmap.Request{}); err != nil {
			t.Fatalf("do: %v", err)
		}
	}
	if got := fetches(); got != 1 {
		t.Errorf("session fetches = %d over five responses reporting the state the dial resolved, want 1", got)
	}
}

// TestSessionRefetchIgnoresAStaleStateInterleavedWithANewOne is
// JT-21's named failure mode reached the other way. Responses to
// concurrent calls straddle a state change, so the older ones report
// the state they started under after the newer ones have already
// reported the move. Keyed on the newest state alone, every flip
// between the two looks like fresh news and refetches, which is the
// storm on a busy account the criterion exists to rule out.
func TestSessionRefetchIgnoresAStaleStateInterleavedWithANewOne(t *testing.T) {
	states := []string{"s2", "session-1", "s2", "session-1", "s2"}
	var mu sync.Mutex
	next := 0
	srv, fetches := newStateServer(t, func() string {
		mu.Lock()
		defer mu.Unlock()
		state := states[min(next, len(states)-1)]
		next++
		return `{"methodResponses":[],"sessionState":"` + state + `"}`
	})
	session := dialTestSession(t, srv)

	for range states {
		if _, err := session.do(t.Context(), &jmap.Request{}); err != nil {
			t.Fatalf("do: %v", err)
		}
	}

	if got := fetches(); got != 2 {
		t.Errorf("session fetches across %d responses flipping between two states = %d, want 2 (the dial and one refetch)", len(states), got)
	}
}

// TestSessionRefetchDoesNotBlockOtherCalls pins the lock scope. do
// follows the state on every successful response, so a session
// resource that is slow or black-holing would park every other JMAP
// call on this Session behind one fetch if the dedup held its lock
// across the wire.
func TestSessionRefetchDoesNotBlockOtherCalls(t *testing.T) {
	hang := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(hang) }) }

	var mu sync.Mutex
	fetches := 0
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	// Registered after the server, so it runs before it: cleanups run
	// last in first out, and httptest.Server.Close waits out a handler
	// still blocked on the gate. Registered the other way round, a test
	// that fails here panics the whole binary at the package timeout
	// instead of reporting its own failure.
	t.Cleanup(release)
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		fetches++
		first := fetches == 1
		mu.Unlock()
		if !first {
			<-hang // the refetch never comes back on its own
		}
		_, _ = fmt.Fprintf(w, `{"capabilities":{},"accounts":{},"primaryAccounts":{"urn:ietf:params:jmap:mail":"u1"},"apiUrl":%q,"state":"session-1"}`, srv.URL+"/api")
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"methodResponses":[],"sessionState":"s2"}`)
	})

	session, err := Dial(context.Background(), srv.URL+"/session", NewStaticCredentials("test-token"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	stuck := make(chan struct{})
	go func() {
		defer close(stuck)
		_, _ = session.do(context.Background(), &jmap.Request{})
	}()

	// The refetch is in flight and will not finish. Every other call
	// runs against the session in hand meanwhile.
	waitFor(t, "the refetch to reach the server", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return fetches > 1
	})

	done := make(chan error, 1)
	go func() {
		_, err := session.do(context.Background(), &jmap.Request{})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("do while a refetch was in flight: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a call made while the session refetch was in flight never returned")
	}

	release()
	<-stuck
}

// TestSessionRefetchAnswersEachStateOnce is the acceptance criterion
// without the depth condition the pair carried. Responses cycling
// through three states, the shape a run of concurrent calls straddling
// two moves produces, must cost one refetch per state the server moved
// to and nothing for the repeats. Remembering only the newest and the
// one it superseded, every response in such a cycle looks like fresh
// news.
func TestSessionRefetchAnswersEachStateOnce(t *testing.T) {
	cycle := []string{"s2", "s3", "s4"}
	var mu sync.Mutex
	seen := 0
	srv, fetches := newStateServer(t, func() string {
		mu.Lock()
		defer mu.Unlock()
		state := cycle[seen%len(cycle)]
		seen++
		return `{"methodResponses":[],"sessionState":"` + state + `"}`
	})
	session := dialTestSession(t, srv)

	for range 3 * len(cycle) {
		if _, err := session.do(t.Context(), &jmap.Request{}); err != nil {
			t.Fatalf("do: %v", err)
		}
	}

	// The dial, and one refetch for each distinct state.
	if want := 1 + len(cycle); fetches() != want {
		t.Errorf("session fetches across %d responses cycling %d states = %d, want %d",
			3*len(cycle), len(cycle), fetches(), want)
	}
}

// TestSessionRefetchRemembersItsWholeRing walks the ring past its own
// size, which is where a fixed-size scan hides an off-by-one. The
// oldest answered state falls out, and everything still in it stays
// answered.
func TestSessionRefetchRemembersItsWholeRing(t *testing.T) {
	var r refetchState
	r.seed("dial")

	// The dial fills one slot, so this fills the rest.
	filled := make([]string, answeredStates-1)
	for i := range filled {
		filled[i] = fmt.Sprintf("s%d", i)
		if !r.claim(filled[i]) {
			t.Fatalf("claim(%q) = false, want true on a state never seen", filled[i])
		}
		r.done()
	}

	for _, state := range append([]string{"dial"}, filled...) {
		if r.claim(state) {
			t.Errorf("claim(%q) = true, want false: a state already answered and still in the ring", state)
			r.done()
		}
	}

	// One more evicts the oldest, which is the dial's own state.
	if !r.claim("overflow") {
		t.Fatal(`claim("overflow") = false, want true`)
	}
	r.done()
	if !r.claim("dial") {
		t.Error(`claim("dial") = false, want true: the oldest state should have fallen out of the ring`)
	}
	r.done()
	for _, state := range filled[1:] {
		if r.claim(state) {
			t.Errorf("claim(%q) = true, want false: only the oldest entry should have been evicted", state)
			r.done()
		}
	}

	// Every ring slot now holds a non-empty state, so an empty state
	// reads as unclaimed by the ring scan alone: it is the state ==
	// "" guard, not the ring, that has to refuse it.
	if r.claim("") {
		t.Error(`claim("") = true, want false: an empty sessionState is never a move to answer`)
		r.done()
	}
}

// TestSessionRefetchIsSingleFlight covers what keeping the fetch
// outside the lock costs if nothing replaces the ordering it used to
// give. Two goroutines can otherwise claim different states and fetch
// at once, and the later-returning one wins, so an older session
// overwrites a newer one and the client talks to stale URLs until the
// state moves again. It also bounds the ring: one session request per
// round trip, whatever the responses report.
func TestSessionRefetchIsSingleFlight(t *testing.T) {
	hang := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(hang) }) }

	var mu sync.Mutex
	fetches := 0
	states := []string{"s2", "s3", "s4"}
	responses := 0
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Cleanup(release) // before the server closes; see gate's doc
	mux.HandleFunc("/session", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		fetches++
		first := fetches == 1
		mu.Unlock()
		if !first {
			<-hang
		}
		_, _ = fmt.Fprintf(w, `{"capabilities":{},"accounts":{},"primaryAccounts":{"urn:ietf:params:jmap:mail":"u1"},"apiUrl":%q,"state":"session-1"}`, srv.URL+"/api")
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		state := states[min(responses, len(states)-1)]
		responses++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"methodResponses":[],"sessionState":"`+state+`"}`)
	})

	session, err := Dial(context.Background(), srv.URL+"/session", NewStaticCredentials("test-token"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	stuck := make(chan struct{})
	go func() {
		defer close(stuck)
		_, _ = session.do(context.Background(), &jmap.Request{})
	}()
	waitFor(t, "the first refetch to reach the server", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return fetches > 1
	})

	// Every one of these reports a state the ring has never answered,
	// and every one must leave the session resource alone while a fetch
	// is already in flight. They run on their own goroutines because
	// without single-flight each blocks in its own refetch, and the
	// count is what this test is here to read either way.
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := session.do(context.Background(), &jmap.Request{}); err != nil {
				t.Errorf("do: %v", err)
			}
		})
	}
	settled := make(chan struct{})
	go func() { wg.Wait(); close(settled) }()
	select {
	case <-settled:
	case <-time.After(2 * time.Second):
		t.Error("calls made while a refetch was in flight did not return; they started refetches of their own")
	}

	mu.Lock()
	got := fetches
	mu.Unlock()
	if got != 2 {
		t.Errorf("session fetches while one was in flight = %d, want 2 (the dial and the one in flight)", got)
	}

	// Received unconditionally, even after the select above timed out:
	// a regression that makes the four calls dispatch their own
	// fetches leaves them blocked on hang until release, and letting
	// the test function return before they finish panics the binary
	// with a t.Errorf from a goroutine after the test has completed.
	release()
	<-settled
	<-stuck
}
