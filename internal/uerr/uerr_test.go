package uerr

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// TestErrorFieldsAreExactlyRedactionSafe pins ADR-0013's structural
// redaction claim: Error carries no field a body, an address, or a
// credential could hide in. A field added later must extend this
// list deliberately, not slip in unnoticed.
func TestErrorFieldsAreExactlyRedactionSafe(t *testing.T) {
	want := []string{"Op", "IDs", "Class", "Message", "Cause"}

	typ := reflect.TypeFor[Error]()
	if typ.NumField() != len(want) {
		t.Fatalf("Error has %d fields, want %d: %v", typ.NumField(), len(want), want)
	}
	for i, name := range want {
		if got := typ.Field(i).Name; got != name {
			t.Errorf("field %d = %s, want %s", i, got, name)
		}
	}
}

func TestConstructorIsTheOnlyPath(t *testing.T) {
	captureLog(t)

	cause := errors.New("dial tcp: connection refused")

	tests := []struct {
		name  string
		class Class
	}{
		{"auth", ClassAuth},
		{"auth refresh failed", ClassAuthRefreshFailed},
		{"not found", ClassNotFound},
		{"connection", ClassConnection},
		{"server", ClassServer},
		{"throttled", ClassThrottled},
		{"store local", ClassStoreLocal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := sentence[tt.class]

			err := New("fetch mailbox", []string{"m-1"}, tt.class, cause)

			if err.Class != tt.class {
				t.Errorf("Class = %v, want %v", err.Class, tt.class)
			}
			if err.Message != want {
				t.Errorf("Message = %q, want %q", err.Message, want)
			}
			if got := err.Error(); got != want {
				t.Errorf("Error() = %q, want %q", got, want)
			}
			if !errors.Is(err, cause) {
				t.Error("errors.Is(err, cause) = false, want true")
			}

			var got Error
			if !errors.As(fmt.Errorf("wrap: %w", err), &got) {
				t.Fatal("errors.As failed to recover Error from a wrapping chain")
			}
			if got.Class != tt.class {
				t.Errorf("recovered Class = %v, want %v", got.Class, tt.class)
			}
		})
	}
}

func TestEveryClassLogs(t *testing.T) {
	buf := captureLog(t)
	cause := errors.New("dial tcp: connection refused")

	classes := []Class{
		ClassAuth, ClassAuthRefreshFailed, ClassNotFound,
		ClassConnection, ClassServer, ClassThrottled, ClassStoreLocal,
	}
	for _, class := range classes {
		t.Run(class.String(), func(t *testing.T) {
			buf.Reset()

			err := New("fetch mailbox", []string{"m-1"}, class, cause)

			rec := decodeLogLine(t, buf)
			if rec["class"] != class.String() {
				t.Errorf("log class = %v, want %v", rec["class"], class.String())
			}
			if rec["cause"] != cause.Error() {
				t.Errorf("log cause = %v, want %v", rec["cause"], cause.Error())
			}
			if rec["message"] != err.Message {
				t.Errorf("log message = %v, want %v (correlation to user sentence)", rec["message"], err.Message)
			}
		})
	}
}
