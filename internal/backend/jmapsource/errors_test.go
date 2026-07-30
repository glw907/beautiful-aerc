package jmapsource

import (
	"context"
	"errors"
	"io"
	"net/http"
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
		{"server error", &jmap.RequestError{Status: http.StatusInternalServerError}, uerr.ClassServer},
		{"http error, no problem body", &jmap.HTTPError{Status: http.StatusUnauthorized}, uerr.ClassAuth},
		{"connection dead", io.ErrUnexpectedEOF, uerr.ClassConnection},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A fresh authState per case: two cases above (unauthorized,
			// forbidden) both classify ClassAuth, and authState.report
			// dedups a second ClassAuth call against a state it already
			// saw one for. Sharing one authState across cases would make
			// the second case's classify return the first case's cached
			// value instead of classifying its own error.
			got := classify("jmap.test", c.err, &authState{})
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

func TestClassifyPassesThroughUnrecognizedError(t *testing.T) {
	unrecognized := errors.New("something else entirely")
	if got := classify("jmap.test", unrecognized, &authState{}); got != unrecognized {
		t.Errorf("classify(%v) = %v, want the same error unclassified", unrecognized, got)
	}
	if classify("jmap.test", nil, &authState{}) != nil {
		t.Error("classify(nil) should return nil")
	}
}

// TestClassifyDedupsRepeatedAuthFailures pins F4's fix: three sync
// poll cycles across two kinds (six do()-shaped classify calls, the
// shape the finding's own experiment used) against a session whose
// credential stays rejected the whole time must log the ClassAuth
// failure once, not six times: a persisted revoked token otherwise
// floods the log at PollInterval's cadence (about 2,880 lines a day
// at ADR-0005's default), which is what fixing the 401-classification
// defect reintroduced. Every call still classifies ClassAuth, so a
// caller checking errors.As(err, &uerr.Error{}) never silently stops
// seeing the failure; only the repeated log write is deduped.
func TestClassifyDedupsRepeatedAuthFailures(t *testing.T) {
	buf := uerrtest.Capture(t)
	auth := &authState{}
	cause := &jmap.HTTPError{Status: http.StatusUnauthorized}

	for cycle := range 3 {
		for range 2 { // two kinds, mailbox and message
			got := classify("jmap.do", cause, auth)
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
	auth := &authState{}
	cause := &jmap.HTTPError{Status: http.StatusUnauthorized}

	for _, got := range []error{
		classify("jmap.do", cause, auth), // episode 1, call 1: logs
		classify("jmap.do", cause, auth), // episode 1, call 2: deduped
	} {
		var ue uerr.Error
		if !errors.As(got, &ue) || ue.Class != uerr.ClassAuth {
			t.Fatalf("classify = %v, want a ClassAuth uerr.Error", got)
		}
	}
	auth.clear() // the recovery do() itself performs on a successful call
	if got := classify("jmap.do", cause, auth); !errors.As(got, new(uerr.Error)) {
		t.Fatalf("classify after recovery = %v, want a uerr.Error", got)
	}

	lines := uerrtest.Lines(t, buf)
	if len(lines) != 2 {
		t.Fatalf("logged %d line(s) across two episodes, want exactly 2 (one per episode)", len(lines))
	}
}

// TestClassifyAuthDedupDoesNotSuppressOtherClasses proves the dedup
// is scoped to ClassAuth, per the finding's own instruction: a
// ClassAuth failure followed by a differently classified one (a
// server error, here) still logs the second failure, so the dedup
// cannot be mistaken for a general "log the first failure of a
// session and nothing else after" rule.
func TestClassifyAuthDedupDoesNotSuppressOtherClasses(t *testing.T) {
	buf := uerrtest.Capture(t)
	auth := &authState{}

	_ = classify("jmap.do", &jmap.HTTPError{Status: http.StatusUnauthorized}, auth)
	_ = classify("jmap.do", &jmap.HTTPError{Status: http.StatusInternalServerError}, auth)

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

// TestDialErrorMessageSurvivesAZeroValue proves DialError.Error does
// not panic on a value carrying no cause. DialError is exported, so a
// caller outside this package can build or copy one, and an error type
// that panics when printed takes down whatever was already reporting a
// failure.
func TestDialErrorMessageSurvivesAZeroValue(t *testing.T) {
	if msg := (DialError{}).Error(); msg == "" {
		t.Error("a zero-value DialError's message is empty, want a fixed string")
	}

	cause := errors.New("session: unexpected status 401")
	if msg := (DialError{Class: uerr.ClassAuth, Cause: cause}).Error(); msg != cause.Error() {
		t.Errorf("Error() = %q, want the cause's own message %q", msg, cause.Error())
	}
}
