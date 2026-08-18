package jmapsource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
	"github.com/glw907/poplar/internal/uerr/uerrtest"
	"github.com/glw907/poplar/jmap"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want uerr.Class
	}{
		{"unauthorized", &jmap.RequestError{Status: http.StatusUnauthorized}, uerr.ClassAuth},
		{"forbidden", &jmap.RequestError{Status: http.StatusForbidden}, uerr.ClassAuth},
		{"not found", &jmap.RequestError{Status: http.StatusNotFound}, uerr.ClassNotFound},
		{"throttled", &jmap.RequestError{Status: http.StatusTooManyRequests}, uerr.ClassThrottled},
		{"bad request", &jmap.RequestError{Status: http.StatusBadRequest}, uerr.ClassServer},
		{"conflict", &jmap.RequestError{Status: http.StatusConflict}, uerr.ClassServer},
		{"server error", &jmap.RequestError{Status: http.StatusInternalServerError}, uerr.ClassServer},
		{"http error, no problem body", &jmap.HTTPError{Status: http.StatusUnauthorized}, uerr.ClassAuth},
		{"connection dead", io.ErrUnexpectedEOF, uerr.ClassConnection},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A fresh episodeState per case: two cases above
			// (unauthorized, forbidden) both classify ClassAuth, and
			// episodeState.report dedups a second call of a class it is
			// already mid-episode under. Sharing one across cases would make
			// the second case's classify return the first case's cached
			// value instead of classifying its own error.
			got := classify("jmapsource.test", c.err, &episodeState{})
			var ue uerr.Error
			if !errors.As(got, &ue) {
				t.Fatalf("classify(%v) = %v, want a uerr.Error", c.err, got)
			}
			if ue.Class != c.want {
				t.Errorf("Class = %v, want %v", ue.Class, c.want)
			}
			if !errors.Is(got, c.err) {
				t.Errorf("classify(%v) does not unwrap to the original error", c.err)
			}
		})
	}
}

// TestRequestLevelRejectionCarriesItsClassToEveryCaller covers the
// rejection RFC 8620 section 3.6.1 makes routine: notRequest, notJSON,
// limit and unknownCapability all arrive as HTTP 400 with a
// problem-details body. Leaving it unclassified here left the class to
// whichever engine made the call, and the two chose differently, so the
// same rejection reached the user as a connectivity problem from a sync
// flush and a server problem from an outbox dispatch. Both engines'
// entry points are exercised, and each error must arrive already
// carrying its class, which is what makes the two agree.
func TestRequestLevelRejectionCarriesItsClassToEveryCaller(t *testing.T) {
	uerrtest.Capture(t)

	mux, srv := newFakeServer(t)
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"type":"urn:ietf:params:jmap:error:limit","limit":"maxCallsInRequest","status":400}`)
	})
	session := dialTestSession(t, srv)

	calls := []struct {
		name string
		call func() error
	}{
		{"Changes, the sync engine's call", func() error {
			_, err := session.Mail().Changes(context.Background(), backend.ObjectKindMailbox, "tok-1", 0)
			return err
		}},
		{"ApplyBatch, the outbox dispatcher's call", func() error {
			_, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{{
				Op:     backend.MutationUpdate,
				Kind:   backend.ObjectKindMessage,
				ID:     "m1",
				Fields: backend.MessagePatch{SetFlags: backend.FlagSeen},
			}})
			return err
		}},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			err := c.call()
			if err == nil {
				t.Fatal("the call succeeded against a server answering 400")
			}
			classified, ok := uerr.Peel(err)
			if !ok {
				t.Fatalf("%v carries no class, so the engine's own default decides it", err)
			}
			if class, _ := classified.ClassCause(); class != uerr.ClassServer {
				t.Errorf("class = %v, want ClassServer: the server answered, so nothing here is a connectivity problem", class)
			}
		})
	}
}

// TestMethodLevelErrorCarriesItsClassToEveryCaller is the second half
// of the rejection story: RFC 8620 section 3.6.2 routes a per-call
// failure as an "error" invocation inside an otherwise fine 200, so
// accountNotFound, invalidArguments, serverFail and unknownMethod
// never reach the status classification at all. Unclassified, they
// took each engine's own default, and the sync worker's default made
// a server that answered read as a server nobody could reach.
func TestMethodLevelErrorCarriesItsClassToEveryCaller(t *testing.T) {
	uerrtest.Capture(t)

	for _, methodErrorType := range []string{"accountNotFound", "invalidArguments", "serverFail", "unknownMethod"} {
		t.Run(methodErrorType, func(t *testing.T) {
			body := fmt.Appendf(nil, `{"methodResponses":[["error",{"type":%q},"0"]],"sessionState":"s-1"}`, methodErrorType)
			session, _ := newTestSession(t, body, body)

			calls := []struct {
				name string
				call func() error
			}{
				{"Changes, the sync worker's flush path", func() error {
					_, err := session.Mail().Changes(context.Background(), backend.ObjectKindMailbox, "tok-1", 0)
					return err
				}},
				{"ApplyBatch, the outbox dispatcher's path", func() error {
					_, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{{
						Op:     backend.MutationUpdate,
						Kind:   backend.ObjectKindMessage,
						ID:     "m1",
						Fields: backend.MessagePatch{SetFlags: backend.FlagSeen},
					}})
					return err
				}},
			}
			for _, c := range calls {
				t.Run(c.name, func(t *testing.T) {
					err := c.call()
					if err == nil {
						t.Fatalf("the call succeeded against a %s method error", methodErrorType)
					}
					classified, ok := uerr.Peel(err)
					if !ok {
						t.Fatalf("%v carries no class, so the engine's own default decides it", err)
					}
					if class, _ := classified.ClassCause(); class != uerr.ClassServer {
						t.Errorf("class = %v, want ClassServer: the server answered the request", class)
					}
					if !strings.Contains(err.Error(), methodErrorType) {
						t.Errorf("error = %q, want the server's own type in it", err)
					}
				})
			}
		})
	}
}

// TestMethodLevelErrorKeepsTheSignalTypesTranslated pins what the
// classification above must not break. Two method-error types are
// signals rather than failures, and each engine reads its own by
// errors.Is against a seam sentinel; wrapping the MethodError in a
// classified failure has to leave those matches intact.
func TestMethodLevelErrorKeepsTheSignalTypesTranslated(t *testing.T) {
	uerrtest.Capture(t)

	t.Run("cannotCalculateChanges is still a state reset", func(t *testing.T) {
		session, _ := newTestSession(t, []byte(`{"methodResponses":[["error",{"type":"cannotCalculateChanges"},"0"]],"sessionState":"s-1"}`))
		_, err := session.Mail().Changes(context.Background(), backend.ObjectKindMailbox, "tok-1", 0)
		if !errors.Is(err, backend.ErrStateReset) {
			t.Fatalf("Changes error = %v, want backend.ErrStateReset", err)
		}
	})

	t.Run("stateMismatch is still the batch sentinel", func(t *testing.T) {
		session, _ := newTestSession(t, []byte(`{"methodResponses":[["error",{"type":"stateMismatch"},"0"]],"sessionState":"s-1"}`))
		_, err := session.Mail().ApplyBatch(context.Background(), []backend.Mutation{{Op: backend.MutationDestroy, ID: "msg-1"}})
		if !errors.Is(err, backend.ErrStateMismatch) {
			t.Fatalf("ApplyBatch error = %v, want backend.ErrStateMismatch", err)
		}
	})
}

// TestStandingRejectionLogsOncePerEpisode covers what widening the
// status classification would otherwise have cost. A deterministic
// 400 (RFC 8620 section 3.6.1's unknownCapability and limit are the
// two a client cannot talk its way out of) now classifies, and the
// sync worker asks for changes per kind on every poll, so a line per
// call is roughly 2,880 a day for one standing failure. That is the
// flood the dedup exists to prevent, and it has to hold for every
// class rather than for the one it was first written against.
func TestStandingRejectionLogsOncePerEpisode(t *testing.T) {
	buf := uerrtest.Capture(t)

	rejecting := true
	mux, srv := newFakeServer(t)
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		if rejecting {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"type":"urn:ietf:params:jmap:error:unknownCapability","status":400}`)
			return
		}
		_, _ = io.WriteString(w, `{"methodResponses":[["Mailbox/changes",{"accountId":"u1","oldState":"a","newState":"b","hasMoreChanges":false},"0"],["Mailbox/get",{"accountId":"u1","state":"b","list":[]},"1"],["Mailbox/get",{"accountId":"u1","state":"b","list":[]},"2"]],"sessionState":"s-1"}`)
	})
	session := dialTestSession(t, srv)

	poll := func() error {
		_, err := session.Mail().Changes(context.Background(), backend.ObjectKindMailbox, "tok-1", 0)
		return err
	}

	// Three cycles across two kinds, the shape a standing rejection
	// produces against the sync worker's own cadence.
	for range 6 {
		if err := poll(); err == nil {
			t.Fatal("the poll succeeded against a server answering 400")
		}
	}
	if lines := uerrtest.Lines(t, buf); len(lines) != 1 {
		t.Fatalf("logged %d line(s) across 6 identical rejections, want exactly 1: %v", len(lines), lines)
	}

	rejecting = false
	if err := poll(); err != nil {
		t.Fatalf("the recovery poll failed: %v", err)
	}

	rejecting = true
	if err := poll(); err == nil {
		t.Fatal("the poll after recovery succeeded against a server answering 400")
	}
	lines := uerrtest.Lines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("logged %d line(s) across two episodes, want exactly 2 (one each): %v", len(lines), lines)
	}
	for i, line := range lines {
		if line["class"] != "server" {
			t.Errorf("line %d class = %v, want server", i, line["class"])
		}
	}
}

func TestClassifyPassesThroughUnrecognizedError(t *testing.T) {
	unrecognized := errors.New("something else entirely")
	if got := classify("jmapsource.test", unrecognized, &episodeState{}); got != unrecognized {
		t.Errorf("classify(%v) = %v, want the same error unclassified", unrecognized, got)
	}
	if classify("jmapsource.test", nil, &episodeState{}) != nil {
		t.Error("classify(nil) should return nil")
	}
}

// TestClassifyDedupsRepeatedAuthFailures proves that three sync poll
// cycles across two kinds (six do()-shaped classify calls, the repeat-
// call shape a persisted revoked token produces) against a session
// whose credential stays rejected the whole time must log the
// ClassAuth failure once, not six times: a persisted revoked token
// otherwise floods the log at PollInterval's cadence (about 2,880
// lines a day at ADR-0005's default). Every call still classifies
// ClassAuth, so a caller checking errors.As(err, &uerr.Error{}) never
// silently stops seeing the failure; only the repeated log write is
// deduped.
func TestClassifyDedupsRepeatedAuthFailures(t *testing.T) {
	buf := uerrtest.Capture(t)
	episode := &episodeState{}
	cause := &jmap.HTTPError{Status: http.StatusUnauthorized}

	for cycle := range 3 {
		for range 2 { // two kinds, mailbox and message
			got := classify("jmapsource.do", cause, episode)
			var ue uerr.Error
			if !errors.As(got, &ue) {
				t.Fatalf("cycle %d: classify = %v, want a uerr.Error on every call", cycle, got)
			}
			if ue.Class != uerr.ClassAuth {
				t.Errorf("cycle %d: Class = %v, want ClassAuth", cycle, ue.Class)
			}
		}
	}

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("logged %d line(s) across 6 identical failures, want exactly 1", len(lines))
	}
}

// TestClassifyAuthDedupResetsOnRecovery proves the dedup in
// TestClassifyDedupsRepeatedAuthFailures is a rate limit, not a
// silence: a call that clears the episode (do()'s own success path,
// mirrored here since classify itself never sees the nil-error case)
// lets the next ClassAuth failure log again, so a credential that
// recovers and later gets revoked a second time is not permanently
// muted by the first episode's dedup.
func TestClassifyAuthDedupResetsOnRecovery(t *testing.T) {
	buf := uerrtest.Capture(t)
	episode := &episodeState{}
	cause := &jmap.HTTPError{Status: http.StatusUnauthorized}

	for _, got := range []error{
		classify("jmapsource.do", cause, episode), // episode 1, call 1: logs
		classify("jmapsource.do", cause, episode), // episode 1, call 2: deduped
	} {
		var ue uerr.Error
		if !errors.As(got, &ue) || ue.Class != uerr.ClassAuth {
			t.Fatalf("classify = %v, want a ClassAuth uerr.Error", got)
		}
	}
	episode.clear() // the recovery do() itself performs on a successful call
	if got := classify("jmapsource.do", cause, episode); !errors.As(got, new(uerr.Error)) {
		t.Fatalf("classify after recovery = %v, want a uerr.Error", got)
	}

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("logged %d line(s) across two episodes, want exactly 2 (one per episode)", len(lines))
	}
}

// TestClassifyAuthDedupDoesNotSuppressOtherClasses proves the dedup
// is keyed on the class: a ClassAuth failure followed by a
// differently classified one (a server error, here) still logs the
// second failure, so the dedup cannot be mistaken for a general "log
// the first failure of a session and nothing else after" rule.
func TestClassifyAuthDedupDoesNotSuppressOtherClasses(t *testing.T) {
	buf := uerrtest.Capture(t)
	episode := &episodeState{}

	_ = classify("jmapsource.do", &jmap.HTTPError{Status: http.StatusUnauthorized}, episode)
	_ = classify("jmapsource.do", &jmap.HTTPError{Status: http.StatusInternalServerError}, episode)

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("logged %d line(s), want 2 (the auth failure, then the unrelated server failure)", len(lines))
	}
	if lines[0]["class"] != "auth" || lines[1]["class"] != "server" {
		t.Errorf("logged classes = %v, %v, want auth then server", lines[0]["class"], lines[1]["class"])
	}
}

// TestClassifyMutationFailure covers each JMAP SetError type
// jmapSetErrorClass names a distinct class for, plus a type outside
// that map, and proves the raw SetError type survives as Cause for
// outbox.failure_detail.
func TestClassifyMutationFailure(t *testing.T) {
	cases := []struct {
		setErrorType string
		want         uerr.Class
	}{
		{"notFound", uerr.ClassNotFound},
		{"forbidden", uerr.ClassAuth},
		{"rateLimit", uerr.ClassThrottled},
		{"invalidProperties", uerr.ClassServer},
	}
	for _, c := range cases {
		t.Run(c.setErrorType, func(t *testing.T) {
			got := classifyMutationFailure(c.setErrorType)
			if got.Class != c.want {
				t.Errorf("Class = %v, want %v", got.Class, c.want)
			}
			if got.Cause == nil || got.Cause.Error() != c.setErrorType {
				t.Errorf("Cause = %v, want the raw SetError type %q preserved for failure_detail", got.Cause, c.setErrorType)
			}
		})
	}
}

// TestClassifyMutationFailureDoesNotLog proves classifyMutationFailure
// never reaches uerr.New: it returns a plain backend.Failure,
// not a uerr.Error, so ApplyBatch's per-mutation classification stays
// silent across every dispatch retry attempt and the dispatcher
// (task 10) is the only place that writes a log line, on a state
// transition (ADR-0013 revision 2).
func TestClassifyMutationFailureDoesNotLog(t *testing.T) {
	got := classifyMutationFailure("notFound")
	if ue, ok := errors.AsType[uerr.Error](error(got)); ok {
		t.Fatalf("classifyMutationFailure returned a uerr.Error (%+v), want a plain backend.Failure", ue)
	}
}

// TestDoClassifiesAndLogsA401ThroughChanges is the cutover's one
// permitted behavior change, pinned with the error shape production
// actually produces: Fastmail's 401 body is bare text/plain, not a
// problem-details object, so this scripts that exact shape rather
// than the application/problem+json RFC 8620 section 3.6.1 has the
// server SHOULD send (RFC 7807's media type, not RFC 8620's own).
// TestClassify's "unauthorized" case already covers the
// problem-details shape at the unit level. text/plain drives package
// jmap's refusal() to its *jmap.HTTPError path rather than
// *jmap.RequestError, which discriminates this test to that one shape
// rather than passing under either. go-jmap's decodeHttpError
// recognized neither: it matched only the literal application/json
// content type, so a 401 through Changes reached the caller
// unclassified and uerr.New never ran. classify's uerr.New call now
// fires for both of package jmap's refusal shapes, and this is the
// first test in this pass that can prove it logged, via
// uerrtest.Capture rather than reading the returned error alone.
func TestDoClassifiesAndLogsA401ThroughChanges(t *testing.T) {
	buf := uerrtest.Capture(t)

	mux, srv := newFakeServer(t)
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized"))
	})
	session := dialTestSession(t, srv)

	_, err := session.Mail().Changes(context.Background(), backend.ObjectKindMessage, "1", 50)
	var ue uerr.Error
	if !errors.As(err, &ue) {
		t.Fatalf("Changes error = %v, want a uerr.Error in the chain", err)
	}
	if ue.Class != uerr.ClassAuth {
		t.Errorf("Class = %v, want ClassAuth", ue.Class)
	}

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("logged %d line(s), want exactly 1 for the 401", len(lines))
	}
	if lines[0]["class"] != "auth" {
		t.Errorf("logged class = %v, want %q", lines[0]["class"], "auth")
	}
}
