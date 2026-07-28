package jmap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"git.sr.ht/~rockorager/go-jmap"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/internal/uerr"
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
		{"connection dead", io.ErrUnexpectedEOF, uerr.ClassConnection},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classify("jmap.test", c.err)
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
	if got := classify("jmap.test", unrecognized); got != unrecognized {
		t.Errorf("classify(%v) = %v, want the same error unclassified", unrecognized, got)
	}
	if classify("jmap.test", nil) != nil {
		t.Error("classify(nil) should return nil")
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
			got := classifyMutationFailure("jmap.test", c.setErrorType)
			var ue uerr.Error
			if !errors.As(got, &ue) {
				t.Fatalf("classifyMutationFailure(%q) = %v, want a uerr.Error", c.setErrorType, got)
			}
			if ue.Class != c.want {
				t.Errorf("Class = %v, want %v", ue.Class, c.want)
			}
			if ue.Cause == nil || ue.Cause.Error() != c.setErrorType {
				t.Errorf("Cause = %v, want the raw SetError type %q preserved for failure_detail", ue.Cause, c.setErrorType)
			}
		})
	}
}

// TestDoClassifiesTransportFailure asserts that a transport-level
// JMAP failure reaching do() through a real round trip (a caller
// asking Changes to build and send a request) comes back as a
// uerr.Error with the class the failure's HTTP status names,
// confirming classify is actually wired into do() rather than only
// unit-tested in isolation.
func TestDoClassifiesTransportFailure(t *testing.T) {
	mux, srv := newFakeServer(t)
	mux.HandleFunc("/api", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"urn:ietf:params:jmap:error:unauthorized","status":401,"detail":"token rejected"}`))
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
}
