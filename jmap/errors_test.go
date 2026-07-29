package jmap

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRequestErrorDetailIsNotAFormatString covers JT-30. go-jmap
// called fmt.Sprintf(e.Detail) on server-supplied prose, so a detail
// carrying a percent verb rendered as Go's own error noise and hid
// what the server actually said.
func TestRequestErrorDetailIsNotAFormatString(t *testing.T) {
	cases := []struct {
		name string
		err  RequestError
		want string
	}{
		{
			name: "verb in the detail",
			err:  RequestError{Detail: "capability %s is not supported"},
			want: "capability %s is not supported",
		},
		{
			name: "bad verb in the detail",
			err:  RequestError{Detail: "unexpected %!v(MISSING) token"},
			want: "unexpected %!v(MISSING) token",
		},
		{
			name: "percent in the detail",
			err:  RequestError{Detail: "quota is 100% full"},
			want: "quota is 100% full",
		},
		{
			name: "detail with a limit",
			err:  RequestError{Detail: "request too large %d", Limit: "maxSizeRequest"},
			want: "request too large %d (limit: maxSizeRequest)",
		},
		{
			name: "no detail falls back to the type",
			err:  RequestError{Type: "urn:ietf:params:jmap:error:notJSON"},
			want: "urn:ietf:params:jmap:error:notJSON",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestRequestErrorDecodesProblemDetails proves the RFC 8620 section
// 3.6.1.1 bodies decode, including the "limit" property that names
// which limit was applied.
func TestRequestErrorDecodesProblemDetails(t *testing.T) {
	cases := []struct {
		name      string
		fixture   string
		wantType  string
		wantLimit string
	}{
		{
			name:     "unknownCapability",
			fixture:  "rfc8620-3.6.1.1-unknowncapability.json",
			wantType: "urn:ietf:params:jmap:error:unknownCapability",
		},
		{
			name:      "limit",
			fixture:   "rfc8620-3.6.1.1-limit.json",
			wantType:  "urn:ietf:params:jmap:error:limit",
			wantLimit: "maxSizeRequest",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err RequestError
			if jsonErr := json.Unmarshal(readFixture(t, c.fixture), &err); jsonErr != nil {
				t.Fatalf("Unmarshal: %v", jsonErr)
			}
			if err.Type != c.wantType {
				t.Errorf("Type = %q, want %q", err.Type, c.wantType)
			}
			if err.Status != 400 {
				t.Errorf("Status = %d, want 400", err.Status)
			}
			if err.Detail == "" {
				t.Error("Detail is empty, but the server sent one")
			}
			if err.Limit != c.wantLimit {
				t.Errorf("Limit = %q, want %q", err.Limit, c.wantLimit)
			}
		})
	}
}

// TestWrappedErrorsUnwrap proves the package wraps rather than
// flattens, so a caller can inspect what actually went wrong instead
// of matching on a message.
func TestWrappedErrorsUnwrap(t *testing.T) {
	t.Run("a malformed date", func(t *testing.T) {
		var d Date
		err := json.Unmarshal([]byte(`"not a date"`), &d)
		if err == nil {
			t.Fatal("Unmarshal returned no error")
		}
		if _, ok := errors.AsType[*time.ParseError](err); !ok {
			t.Errorf("errors.AsType did not reach a *time.ParseError in %v", err)
		}
	})

	t.Run("a malformed capability", func(t *testing.T) {
		raw := []byte(`{"capabilities":{"urn:ietf:params:jmap:core":{"maxSizeUpload":"lots"}},` +
			`"accounts":{},"primaryAccounts":{},"username":"","apiUrl":"","state":"1"}`)
		var s Session
		err := json.Unmarshal(raw, &s)
		if err == nil {
			t.Fatal("Unmarshal returned no error")
		}
		typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err)
		if !ok {
			t.Fatalf("errors.AsType did not reach a *json.UnmarshalTypeError in %v", err)
		}
		if typeErr.Field != "maxSizeUpload" {
			t.Errorf("Field = %q, want %q", typeErr.Field, "maxSizeUpload")
		}
	})
}

// TestSetErrorTypedExtras covers JT-12. go-jmap's SetError carried
// only a type, a description, and properties, so every extra below
// was dropped and the user heard "send failed" when the server had
// named the three bad recipients.
func TestSetErrorTypedExtras(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		check func(*testing.T, *SetError)
	}{
		{
			name: "invalidProperties carries properties",
			raw:  `{"type":"invalidProperties","properties":["mailboxIds","keywords"]}`,
			check: func(t *testing.T, e *SetError) {
				if len(e.Properties) != 2 || e.Properties[0] != "mailboxIds" {
					t.Errorf("Properties = %v, want both named", e.Properties)
				}
			},
		},
		{
			name: "blobNotFound carries notFound",
			raw:  `{"type":"blobNotFound","notFound":["G1","G2"]}`,
			check: func(t *testing.T, e *SetError) {
				if len(e.NotFound) != 2 || e.NotFound[1] != "G2" {
					t.Errorf("NotFound = %v, want both blob ids", e.NotFound)
				}
			},
		},
		{
			name: "alreadyExists names the record that already holds it",
			raw:  `{"type":"alreadyExists","existingId":"MB"}`,
			check: func(t *testing.T, e *SetError) {
				if e.ExistingID != "MB" {
					t.Errorf("ExistingID = %q, want MB; RFC 8620 section 5.3 makes it mandatory here", e.ExistingID)
				}
			},
		},
		{
			name: "tooManyRecipients carries maxRecipients",
			raw:  `{"type":"tooManyRecipients","maxRecipients":25}`,
			check: func(t *testing.T, e *SetError) {
				if e.MaxRecipients != 25 {
					t.Errorf("MaxRecipients = %d, want 25", e.MaxRecipients)
				}
			},
		},
		{
			name: "invalidRecipients names the addresses",
			raw:  `{"type":"invalidRecipients","invalidRecipients":["a@","b@"]}`,
			check: func(t *testing.T, e *SetError) {
				if len(e.InvalidRecipients) != 2 || e.InvalidRecipients[0] != "a@" {
					t.Errorf("InvalidRecipients = %v, want both addresses", e.InvalidRecipients)
				}
			},
		},
		{
			name: "invalidEmail carries properties",
			raw:  `{"type":"invalidEmail","properties":["from"]}`,
			check: func(t *testing.T, e *SetError) {
				if len(e.Properties) != 1 || e.Properties[0] != "from" {
					t.Errorf("Properties = %v, want [from]", e.Properties)
				}
			},
		},
		{
			name: "tooManyKeywords carries only a description",
			raw:  `{"type":"tooManyKeywords","description":"at most 100 keywords"}`,
			check: func(t *testing.T, e *SetError) {
				if e.Description != "at most 100 keywords" {
					t.Errorf("Description = %q, want the server's", e.Description)
				}
			},
		},
		{
			name: "tooManyMailboxes decodes",
			raw:  `{"type":"tooManyMailboxes"}`,
			check: func(t *testing.T, e *SetError) {
				if e.Type != "tooManyMailboxes" {
					t.Errorf("Type = %q", e.Type)
				}
			},
		},
		{
			name: "noRecipients decodes",
			raw:  `{"type":"noRecipients"}`,
			check: func(t *testing.T, e *SetError) {
				if e.Type != "noRecipients" {
					t.Errorf("Type = %q", e.Type)
				}
			},
		},
		{
			name: "forbiddenFrom decodes",
			raw:  `{"type":"forbiddenFrom"}`,
			check: func(t *testing.T, e *SetError) {
				if e.Type != "forbiddenFrom" {
					t.Errorf("Type = %q", e.Type)
				}
			},
		},
		{
			name: "cannotUnsend decodes",
			raw:  `{"type":"cannotUnsend"}`,
			check: func(t *testing.T, e *SetError) {
				if e.Type != "cannotUnsend" {
					t.Errorf("Type = %q", e.Type)
				}
			},
		},
		{
			name: "an unregistered type keeps its whole payload",
			raw:  `{"type":"vendorQuotaExceeded","bytesOver":4096,"resetsAt":"2026-07-29T00:00:00Z"}`,
			check: func(t *testing.T, e *SetError) {
				if e.Type != "vendorQuotaExceeded" {
					t.Errorf("Type = %q, want the server's own", e.Type)
				}
				want := `{"type":"vendorQuotaExceeded","bytesOver":4096,"resetsAt":"2026-07-29T00:00:00Z"}`
				if string(e.Raw) != want {
					t.Errorf("Raw = %s, want %s", e.Raw, want)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var setErr SetError
			if err := json.Unmarshal([]byte(c.raw), &setErr); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if setErr.Type == "" {
				t.Fatal("Type is empty")
			}
			c.check(t, &setErr)
		})
	}
}

// TestSetErrorIsAnError is the fix for the defect that SetError alone
// among the three error types in its file did not implement error, so
// every caller rebuilt one by hand and dropped the description doing
// it.
func TestSetErrorIsAnError(t *testing.T) {
	var err error = &SetError{Type: "forbiddenToSend", Description: "sending is disabled"}
	if want := "forbiddenToSend: sending is disabled"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}

	bare := &SetError{Type: "tooManyMailboxes"}
	if want := "tooManyMailboxes"; bare.Error() != want {
		t.Errorf("Error() = %q, want %q", bare.Error(), want)
	}

	wrapped := errors.Join(errors.New("send k1490"), &SetError{Type: "noRecipients"})
	var recovered *SetError
	if !errors.As(wrapped, &recovered) {
		t.Fatal("errors.As did not recover the SetError from a joined error")
	}
	if recovered.Type != "noRecipients" {
		t.Errorf("recovered type = %q, want %q", recovered.Type, "noRecipients")
	}
}

// TestSetPartialFailure covers JT-01, the highest-value case in the
// plan. A /set that created one record and refused two reports all
// three, and the response offers no single flag that reads as "the
// whole batch worked".
func TestSetPartialFailure(t *testing.T) {
	resp := decodeResponse(t, "set-partial-failure.json")

	under := resp.Invocations("0")
	if len(under) != 1 {
		t.Fatalf("Invocations returned %d, want 1", len(under))
	}
	set := argsOf[*EmailSetResponse](t, resp, "0")

	if len(set.Created) != 1 {
		t.Errorf("Created = %v, want one record", set.Created)
	}
	if created := set.Created["k1"]; created == nil || created.ID != "M1ba2ffb0d2b7a1a4" {
		t.Errorf("Created[k1] = %+v, want the server's assigned id", created)
	}

	if len(set.NotCreated) != 2 {
		t.Fatalf("NotCreated = %v, want two refusals", set.NotCreated)
	}
	blobMissing := set.NotCreated["k2"]
	if blobMissing.Type != "blobNotFound" {
		t.Errorf("k2 type = %q, want %q", blobMissing.Type, "blobNotFound")
	}
	if len(blobMissing.NotFound) != 1 || blobMissing.NotFound[0] != "G0000000000000000" {
		t.Errorf("k2 NotFound = %v, want the missing blob id", blobMissing.NotFound)
	}
	if set.NotCreated["k3"].Type != "tooManyMailboxes" {
		t.Errorf("k3 type = %q, want %q", set.NotCreated["k3"].Type, "tooManyMailboxes")
	}

	// The state moved even though two records failed, which is why a
	// caller cannot read success off the call's own outcome.
	if set.OldState == set.NewState {
		t.Errorf("state did not move, but one record was created")
	}
}

// TestMethodErrorIsMatchesOnlyItsOwnType guards the sentinel table
// against a copy-paste that gives two sentinels one type string.
func TestMethodErrorIsMatchesOnlyItsOwnType(t *testing.T) {
	sentinels := []*MethodError{
		ErrServerUnavailable,
		ErrServerFail,
		ErrServerPartialFail,
		ErrUnknownMethod,
		ErrInvalidArguments,
		ErrInvalidResultReference,
		ErrForbidden,
		ErrAccountNotFound,
		ErrAccountNotSupportedByMethod,
		ErrAccountReadOnly,
		ErrCannotCalculateChanges,
		ErrAnchorNotFound,
	}

	for _, sentinel := range sentinels {
		if sentinel.Type == "" {
			t.Fatalf("sentinel %v has an empty type", sentinel)
		}
		decoded := &MethodError{Type: sentinel.Type}
		matches := 0
		for _, other := range sentinels {
			if errors.Is(decoded, other) {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("a %q error matched %d sentinels, want exactly 1", sentinel.Type, matches)
		}
	}

	if errors.Is(ErrServerFail, errors.New("serverFail")) {
		t.Error("a MethodError matched a plain error with the same text")
	}
}

// TestMethodErrorRawSurvivesRemarshal proves the raw payload is
// captured rather than aliased into the decoder's buffer.
func TestMethodErrorRawSurvivesRemarshal(t *testing.T) {
	raw := []byte(`{"methodResponses":[["error",{"type":"limitReached","limit":"maxObjectsInGet"},"c1"]],` +
		`"sessionState":"75128aab4b1b"}`)

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	methodErr := argsOf[*MethodError](t, &resp, "c1")

	var payload map[string]any
	if err := json.Unmarshal(methodErr.Raw, &payload); err != nil {
		t.Fatalf("Raw does not parse: %v", err)
	}
	if payload["limit"] != "maxObjectsInGet" {
		t.Errorf("Raw = %s, want it to keep the limit property", methodErr.Raw)
	}
	if !strings.Contains(methodErr.Error(), "limitReached") {
		t.Errorf("Error() = %q, want it to name the type", methodErr.Error())
	}
}
