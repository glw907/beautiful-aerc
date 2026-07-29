package jmap

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
)

// sessionTemplate is a JMAP session resource: Core and Mail
// capabilities plus a submission capability with a positive
// maxDelayedSend, so probeCapabilities has something to read for
// every field it populates.
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
  "downloadUrl": "%[1]s/download/{blobId}",
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
// status from the session endpoint itself (dial-time auth rejection,
// before go-jmap ever builds a *jmap.RequestError) still reaches the
// caller classified, as a DialError, not a bare fmt.Errorf, and
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
	de, ok := errors.AsType[DialError](err)
	if !ok {
		t.Fatalf("Dial error = %v, want a DialError in the chain", err)
	}
	if de.Class != uerr.ClassAuth {
		t.Errorf("Class = %v, want ClassAuth", de.Class)
	}
	if ue, ok := errors.AsType[uerr.Error](err); ok {
		t.Errorf("Dial error carries a uerr.Error (%+v), want none: classification here must not log", ue)
	}
}

// TestDialClassifiesDeadConnectionWithoutLogging asserts a session
// endpoint that refuses the connection outright (nothing listening)
// also classifies as a DialError rather than a bare network error,
// and without logging, the same discipline
// TestDialClassifiesRejectedSessionWithoutLogging proves for a
// rejected status.
func TestDialClassifiesDeadConnectionWithoutLogging(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	deadURL := srv.URL + "/session"
	srv.Close() // nothing listens on deadURL from here on

	_, err := Dial(context.Background(), deadURL, NewStaticCredentials("tok"))
	de, ok := errors.AsType[DialError](err)
	if !ok {
		t.Fatalf("Dial error = %v, want a DialError in the chain", err)
	}
	if de.Class != uerr.ClassConnection {
		t.Errorf("Class = %v, want ClassConnection", de.Class)
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
