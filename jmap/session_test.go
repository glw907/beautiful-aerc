package jmap

import (
	"encoding/json"
	"reflect"
	"testing"
)

// decodeSession reads the RFC 8620 section 2.1 example session.
//
// The RFC's own listing cannot be transcribed byte for byte: it
// carries "..." elisions inside two account capability objects and
// drops a comma after the mail capability. The fixture completes both
// from the section 1.3 property lists of RFC 8621, spells
// maxConcurrentRequests as section 2 defines it rather than as the
// example's own singular typo, and carries Fastmail's captured state
// string for JT-18.
func decodeSession(t *testing.T) *Session {
	t.Helper()
	var s Session
	if err := json.Unmarshal(readFixture(t, "rfc8620-2.1-session.json"), &s); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return &s
}

// TestSessionCapabilitiesDecode covers JT-19. go-jmap proved its
// capability dispatch only through a synthetic test type, so not one
// of its real capability structs was ever decoded, and two of them
// shipped with a mistyped tag that read zero forever.
func TestSessionCapabilitiesDecode(t *testing.T) {
	s := decodeSession(t)

	core, ok := s.Capabilities[CoreURI].(*Core)
	if !ok {
		t.Fatalf("core capability is %T, want *Core", s.Capabilities[CoreURI])
	}
	coreFields := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"MaxSizeUpload", core.MaxSizeUpload, 50000000},
		{"MaxConcurrentUpload", core.MaxConcurrentUpload, 8},
		{"MaxSizeRequest", core.MaxSizeRequest, 10000000},
		{"MaxConcurrentRequests", core.MaxConcurrentRequests, 8},
		{"MaxCallsInRequest", core.MaxCallsInRequest, 32},
		{"MaxObjectsInGet", core.MaxObjectsInGet, 256},
		{"MaxObjectsInSet", core.MaxObjectsInSet, 128},
	}
	for _, f := range coreFields {
		if f.got != f.want {
			t.Errorf("Core.%s = %d, want %d", f.name, f.got, f.want)
		}
	}
	wantCollations := []CollationAlgo{ASCIINumeric, ASCIICasemap, UnicodeCasemap}
	if !reflect.DeepEqual(core.CollationAlgorithms, wantCollations) {
		t.Errorf("Core.CollationAlgorithms = %v, want %v", core.CollationAlgorithms, wantCollations)
	}

	account := s.Accounts["A13824"]
	mail, ok := account.Capabilities[MailURI].(*Mail)
	if !ok {
		t.Fatalf("mail account capability is %T, want *Mail", account.Capabilities[MailURI])
	}
	if mail.MaxMailboxesPerEmail != nil {
		t.Errorf("MaxMailboxesPerEmail = %v, want nil for the RFC's null", *mail.MaxMailboxesPerEmail)
	}
	if mail.MaxMailboxDepth == nil || *mail.MaxMailboxDepth != 10 {
		t.Errorf("MaxMailboxDepth = %v, want 10", mail.MaxMailboxDepth)
	}
	if mail.MaxSizeMailboxName != 490 {
		t.Errorf("MaxSizeMailboxName = %d, want 490", mail.MaxSizeMailboxName)
	}
	if mail.MaxSizeAttachmentsPerEmail != 50000000 {
		t.Errorf("MaxSizeAttachmentsPerEmail = %d, want 50000000", mail.MaxSizeAttachmentsPerEmail)
	}
	if len(mail.EmailQuerySortOptions) != 3 {
		t.Errorf("EmailQuerySortOptions = %v, want three entries", mail.EmailQuerySortOptions)
	}
	if !mail.MayCreateTopLevelMailbox {
		t.Error("MayCreateTopLevelMailbox = false, want true")
	}

	submission, ok := account.Capabilities[SubmissionURI].(*Submission)
	if !ok {
		t.Fatalf("submission account capability is %T, want *Submission", account.Capabilities[SubmissionURI])
	}
	if submission.MaxDelayedSend != 44236800 {
		t.Errorf("MaxDelayedSend = %d, want 44236800", submission.MaxDelayedSend)
	}
	futureRelease := submission.SubmissionExtensions["FUTURERELEASE"]
	if len(futureRelease) != 2 || futureRelease[0] != "86400" {
		t.Errorf("SubmissionExtensions[FUTURERELEASE] = %v, want its two ehlo-args", futureRelease)
	}
}

// TestSessionNullLimitIsNotZero pins the distinction the RFC spells
// as null. A plain uint64 reads "no limit" as "no mailboxes allowed",
// which refuses every message without an error.
func TestSessionNullLimitIsNotZero(t *testing.T) {
	s := decodeSession(t)

	unlimited, ok := s.Accounts["A13824"].Capabilities[MailURI].(*Mail)
	if !ok {
		t.Fatalf("A13824 mail capability is %T, want *Mail", s.Accounts["A13824"].Capabilities[MailURI])
	}
	if unlimited.MaxMailboxesPerEmail != nil {
		t.Errorf("MaxMailboxesPerEmail = %d, want nil", *unlimited.MaxMailboxesPerEmail)
	}

	limited, ok := s.Accounts["A97813"].Capabilities[MailURI].(*Mail)
	if !ok {
		t.Fatalf("A97813 mail capability is %T, want *Mail", s.Accounts["A97813"].Capabilities[MailURI])
	}
	if limited.MaxMailboxesPerEmail == nil || *limited.MaxMailboxesPerEmail != 1 {
		t.Errorf("MaxMailboxesPerEmail = %v, want 1", limited.MaxMailboxesPerEmail)
	}
}

// TestSessionUnknownCapabilitySurvives covers JT-20. go-jmap's own
// fixture carried the RFC's "https://example.com/apis/foobar" and
// never asserted that it was tolerated rather than dropped.
func TestSessionUnknownCapabilitySurvives(t *testing.T) {
	const unknown URI = "https://example.com/apis/foobar"

	s := decodeSession(t)
	if _, ok := s.RawCapabilities[unknown]; !ok {
		t.Fatalf("RawCapabilities = %v, want the unknown URI kept", s.RawCapabilities)
	}
	if _, ok := s.Capabilities[unknown]; ok {
		t.Error("an unmodelled capability appeared in the typed map")
	}
	if _, ok := s.RawCapabilities["urn:ietf:params:jmap:contacts"]; !ok {
		t.Error("the contacts capability was dropped")
	}

	unknownAccount := s.Accounts["A13824"].RawCapabilities["urn:ietf:params:jmap:contacts"]
	if len(unknownAccount) == 0 {
		t.Error("an unmodelled account capability was dropped")
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var original, round map[string]any
	if err := json.Unmarshal(readFixture(t, "rfc8620-2.1-session.json"), &original); err != nil {
		t.Fatalf("Unmarshal fixture: %v", err)
	}
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal re-marshalled session: %v", err)
	}
	if !reflect.DeepEqual(original, round) {
		t.Errorf("session did not survive a round trip:\ngot  %v\nwant %v", round, original)
	}
}

// TestSessionStateIsOpaque covers JT-18. Fastmail's value visibly
// encodes a Cyrus generation number, and reading that structure would
// break the day Fastmail changes it.
func TestSessionStateIsOpaque(t *testing.T) {
	const state = "cyrus-77;j-1;p-30c616ea00;s-69951158a7dcb38d"

	s := decodeSession(t)
	if s.State != state {
		t.Fatalf("State = %q, want %q", s.State, state)
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var round Session
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if round.State != state {
		t.Errorf("State after a round trip = %q, want %q", round.State, state)
	}
}

// TestSessionFields covers the rest of the session resource, so a
// mistyped tag on a URL cannot leave the transport pointed nowhere.
func TestSessionFields(t *testing.T) {
	s := decodeSession(t)

	fields := []struct {
		name string
		got  string
		want string
	}{
		{"Username", s.Username, "john@example.com"},
		{"APIURL", s.APIURL, "https://jmap.example.com/api/"},
		{
			"DownloadURL", s.DownloadURL,
			"https://jmap.example.com/download/{accountId}/{blobId}/{name}?accept={type}",
		},
		{"UploadURL", s.UploadURL, "https://jmap.example.com/upload/{accountId}/"},
		{
			"EventSourceURL", s.EventSourceURL,
			"https://jmap.example.com/eventsource/?types={types}&closeafter={closeafter}&ping={ping}",
		},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("Session.%s = %q, want %q", f.name, f.got, f.want)
		}
	}

	if s.PrimaryAccounts[MailURI] != "A13824" {
		t.Errorf("PrimaryAccounts[mail] = %q, want %q", s.PrimaryAccounts[MailURI], "A13824")
	}

	personal := s.Accounts["A13824"]
	if personal.Name != "john@example.com" || !personal.IsPersonal || personal.IsReadOnly {
		t.Errorf("A13824 = %+v, want the user's own writable account", personal)
	}
	shared := s.Accounts["A97813"]
	if shared.IsPersonal || !shared.IsReadOnly {
		t.Errorf("A97813 = %+v, want a shared read-only account", shared)
	}
}

// TestAccountCarriesNoIDField pins the removal of go-jmap's dead
// Account.ID. No code path there ever filled it, so a caller that
// read it got the empty string and had no way to tell. The id lives
// in the Session.Accounts key, which is the only place a server puts
// it.
func TestAccountCarriesNoIDField(t *testing.T) {
	if _, found := reflect.TypeFor[Account]().FieldByName("ID"); found {
		t.Error("Account has an ID field; the id belongs to the Session.Accounts key alone")
	}

	s := decodeSession(t)
	for id := range s.Accounts {
		if !id.Valid() {
			t.Errorf("account key %q is not a valid id", id)
		}
	}
	if len(s.Accounts) != 2 {
		t.Errorf("Accounts holds %d entries, want 2", len(s.Accounts))
	}
}

// TestSessionRejectsAMalformedCapability proves a capability object
// that does not fit its type is an error rather than a silently empty
// struct.
func TestSessionRejectsAMalformedCapability(t *testing.T) {
	raw := []byte(`{"capabilities":{"urn:ietf:params:jmap:core":{"maxSizeUpload":"lots"}},` +
		`"accounts":{},"primaryAccounts":{},"username":"","apiUrl":"","state":"1"}`)

	var s Session
	if err := json.Unmarshal(raw, &s); err == nil {
		t.Fatal("Unmarshal of a malformed capability returned no error")
	}
}
