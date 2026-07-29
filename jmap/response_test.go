package jmap

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// argsOf returns the first response under callID as type T. It fails
// the test when the registry decoded something else, so a table row
// that names the wrong type reads as a failure rather than a panic.
func argsOf[T any](t *testing.T, resp *Response, callID string) T {
	t.Helper()
	under := resp.Invocations(callID)
	if len(under) == 0 {
		t.Fatalf("no response under call id %q", callID)
	}
	value, ok := under[0].Args.(T)
	if !ok {
		var want T
		t.Fatalf("response under %q is %T, want %T", callID, under[0].Args, want)
	}
	return value
}

func decodeResponse(t *testing.T, name string) *Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(readFixture(t, name), &resp); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return &resp
}

// TestResponseKeepsEveryInvocation covers JT-11. RFC 8621 section
// 7.5.1's EmailSubmission/set answers twice under one call id, so a
// response indexed by call id loses the implicit Email/set, and with
// it the news that the draft moved.
func TestResponseKeepsEveryInvocation(t *testing.T) {
	resp := decodeResponse(t, "rfc8621-7.5.1-submission-success.json")

	if len(resp.MethodResponses) != 2 {
		t.Fatalf("MethodResponses has %d entries, want 2", len(resp.MethodResponses))
	}
	under := resp.Invocations("0")
	if len(under) != 2 {
		t.Fatalf("Invocations(%q) returned %d, want 2", "0", len(under))
	}
	if under[0].Name != "EmailSubmission/set" || under[1].Name != "Email/set" {
		t.Fatalf("Invocations returned %q then %q, want the submission then the implicit set",
			under[0].Name, under[1].Name)
	}
	if got := resp.Invocations("nosuchcall"); len(got) != 0 {
		t.Errorf("Invocations of an unused call id returned %d entries, want 0", len(got))
	}
}

// TestResponseImplicitEmailSetMatchesByCreationID covers JT-10. The
// submission answers under a creation id, and the implicit Email/set
// names the real message id, so the two are tied by the creation id
// rather than by their position in the array.
func TestResponseImplicitEmailSetMatchesByCreationID(t *testing.T) {
	resp := decodeResponse(t, "rfc8621-7.5.1-submission-success.json")
	under := resp.Invocations("0")

	submission, ok := under[0].Args.(*EmailSubmissionSetResponse)
	if !ok {
		t.Fatalf("first response is %T, want *EmailSubmissionSetResponse", under[0].Args)
	}
	created, ok := submission.Created["k1490"]
	if !ok {
		t.Fatalf("Created = %v, want an entry for creation id k1490", submission.Created)
	}
	if created.ID != "ES-3bab7f9a-623e-4acf-99a5-2e67facb02a0" {
		t.Errorf("created submission id = %q, want the server's", created.ID)
	}
	if len(submission.NotCreated) != 0 {
		t.Errorf("NotCreated = %v, want it empty on a successful send", submission.NotCreated)
	}

	set, ok := under[1].Args.(*EmailSetResponse)
	if !ok {
		t.Fatalf("second response is %T, want *EmailSetResponse", under[1].Args)
	}
	if _, ok := set.Updated["M7f6ed5bcfd7e2604d1753f6c"]; !ok {
		t.Errorf("Updated = %v, want the sent message", set.Updated)
	}
}

// TestResponseRejectedSubmissionHasNoImplicitSet covers JT-10's
// failure half. The send was refused, so nothing applied the
// onSuccessUpdateEmail patch and the draft is still a draft. A
// caller that reads only the call id and assumes a second response
// marks it sent.
func TestResponseRejectedSubmissionHasNoImplicitSet(t *testing.T) {
	resp := decodeResponse(t, "rfc8621-7.5.1-submission-rejected.json")

	under := resp.Invocations("0")
	if len(under) != 1 {
		t.Fatalf("Invocations returned %d, want 1: a refused send triggers no implicit set", len(under))
	}

	submission, ok := under[0].Args.(*EmailSubmissionSetResponse)
	if !ok {
		t.Fatalf("response is %T, want *EmailSubmissionSetResponse", under[0].Args)
	}
	if len(submission.Created) != 0 {
		t.Errorf("Created = %v, want it empty", submission.Created)
	}
	refused, ok := submission.NotCreated["k1490"]
	if !ok {
		t.Fatalf("NotCreated = %v, want an entry for k1490", submission.NotCreated)
	}
	if refused.Type != "forbiddenToSend" {
		t.Errorf("Type = %q, want %q", refused.Type, "forbiddenToSend")
	}
	if refused.Description == "" {
		t.Error("Description is empty, but the server localised one for the user")
	}
	if submission.OldState != submission.NewState {
		t.Errorf("state moved from %q to %q on a refused send", submission.OldState, submission.NewState)
	}
}

// TestResponseDecodesMethodError covers JT-31. go-jmap registered the
// "error" pseudo-method and no test in its suite ever decoded one.
func TestResponseDecodesMethodError(t *testing.T) {
	raw := []byte(`{"methodResponses":[` +
		`["Mailbox/get",{"accountId":"A13824","state":"78540"},"c1"],` +
		`["error",{"type":"cannotCalculateChanges"},"c2"],` +
		`["error",{"type":"invalidArguments","description":"sinceState is required"},"c3"],` +
		`["error",{"type":"vendorSpecificFailure","retryAfter":30},"c4"]` +
		`],"sessionState":"75128aab4b1b"}`)

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	cannotCalculate := argsOf[*MethodError](t, &resp, "c2")
	if !errors.Is(cannotCalculate, ErrCannotCalculateChanges) {
		t.Errorf("errors.Is(%v, ErrCannotCalculateChanges) = false", cannotCalculate)
	}
	if errors.Is(cannotCalculate, ErrInvalidArguments) {
		t.Error("cannotCalculateChanges matched ErrInvalidArguments")
	}

	invalidArguments := argsOf[*MethodError](t, &resp, "c3")
	if !errors.Is(invalidArguments, ErrInvalidArguments) {
		t.Errorf("errors.Is(%v, ErrInvalidArguments) = false", invalidArguments)
	}
	if want := "invalidArguments: sinceState is required"; invalidArguments.Error() != want {
		t.Errorf("Error() = %q, want %q", invalidArguments.Error(), want)
	}

	unknown := argsOf[*MethodError](t, &resp, "c4")
	if unknown.Type != "vendorSpecificFailure" {
		t.Errorf("Type = %q, want the server's own", unknown.Type)
	}
	// RFC 8620 section 3.6.2 requires a client to treat an error type
	// it does not understand as serverFail. The type it carries stays
	// the server's own, so a caller that wants the real name still
	// reads it.
	if !errors.Is(unknown, ErrServerFail) {
		t.Error("an error type this package does not name did not match ErrServerFail")
	}
	for _, sentinel := range []error{ErrInvalidArguments, ErrCannotCalculateChanges} {
		if errors.Is(unknown, sentinel) {
			t.Errorf("an unregistered error type matched %v", sentinel)
		}
	}
	// A type this package does name is not swept into serverFail.
	if errors.Is(invalidArguments, ErrServerFail) {
		t.Error("invalidArguments matched ErrServerFail")
	}
	if want := `{"type":"vendorSpecificFailure","retryAfter":30}`; string(unknown.Raw) != want {
		t.Errorf("Raw = %s, want the whole payload %s", unknown.Raw, want)
	}
}

// TestResponseOutOfOrderResolvesByCallID proves a response resolves
// by call id rather than by array position, which is what RFC 8620
// section 3.4 promises and what a chained call depends on.
func TestResponseOutOfOrderResolvesByCallID(t *testing.T) {
	raw := []byte(`{"methodResponses":[` +
		`["Email/get",{"accountId":"A13824","state":"1"},"c9"],` +
		`["Mailbox/get",{"accountId":"A13824","state":"2"},"c1"]` +
		`],"sessionState":"75128aab4b1b"}`)

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := resp.Invocations("c1")[0].Args.(*MailboxGetResponse); !ok {
		t.Errorf("call c1 resolved to %T, want *MailboxGetResponse", resp.Invocations("c1")[0].Args)
	}
	if _, ok := resp.Invocations("c9")[0].Args.(*EmailGetResponse); !ok {
		t.Errorf("call c9 resolved to %T, want *EmailGetResponse", resp.Invocations("c9")[0].Args)
	}
}

// TestResponseStateIsOpaque covers JT-18 for the response envelope,
// carrying Fastmail's own captured value. Nothing in the package
// splits, parses, or orders it.
func TestResponseStateIsOpaque(t *testing.T) {
	const state = "cyrus-77;j-1;p-30c616ea00;s-69951158a7dcb38d"

	resp := decodeResponse(t, "set-partial-failure.json")
	if resp.SessionState != state {
		t.Fatalf("SessionState = %q, want %q", resp.SessionState, state)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var round Response
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if round.SessionState != state {
		t.Errorf("SessionState after a round trip = %q, want %q", round.SessionState, state)
	}
}

// TestResponseCreatedIDsRoundTrip covers JT-09's wire half: the
// creation map a request seeds comes back with every creation the
// server made.
func TestResponseCreatedIDsRoundTrip(t *testing.T) {
	raw := []byte(`{"methodResponses":[["Email/set",{"accountId":"A13824"},"0"]],` +
		`"createdIds":{"k1":"M1","k2":"M2"},"sessionState":"75128aab4b1b"}`)

	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(resp.CreatedIDs) != 2 || resp.CreatedIDs["k2"] != "M2" {
		t.Fatalf("CreatedIDs = %v, want both creations", resp.CreatedIDs)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `"createdIds":{"k1":"M1","k2":"M2"}`; !strings.Contains(string(data), want) {
		t.Errorf("Marshal = %s, want it to carry %s", data, want)
	}
}
