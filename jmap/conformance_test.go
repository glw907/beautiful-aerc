//go:build conformance

// The conformance suite runs the package against a second, real JMAP
// server. Fixtures prove poplar reads the RFC the way its authors
// wrote it; only a server that shares no lineage with Fastmail proves
// poplar and a real implementation agree. `make conformance` starts
// Stalwart, provisions it, runs this suite, and tears the container
// down. It is deliberately outside `make check`, which must pass on a
// machine with no container runtime at all.
//
// Two rules hold for every test here. Each one asserts what the server
// actually holds afterwards, never merely that no error came back: a
// call that talks to a real server and asserts only the absence of a
// complaint is the defect this suite exists to catch. And each one
// builds and destroys its own records, so two tests never collide and
// a rerun starts where the last one did.
package jmap_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"maps"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glw907/poplar/jmap"
)

// blobURI is RFC 9404's blob capability. This package models nothing
// under it; the suite reads it off the session to know whether a
// server puts RFC 8620 section 6.3's Blob/copy behind it.
const blobURI jmap.URI = "urn:ietf:params:jmap:blob"

// A target is one server the suite runs against, named by the four
// POPLAR_JMAP_* environment variables of the test inventory's section
// 3. The same test bodies run against Stalwart, against Fastmail, or
// against any server added later.
type target struct {
	client  *jmap.Client
	session *jmap.Session
	account jmap.ID
	profile profile

	// httpClient carries the credentials, so a raw request made
	// beside the Client authenticates the same way.
	httpClient *http.Client
}

// A profile is what one server is known to do where the RFC leaves a
// choice open. A server with no profile of its own gets the zero
// value, which asserts only what the RFC requires.
type profile struct {
	name string

	// uploadStatus is the status this server answers a blob upload
	// with. RFC 8620 section 6.1 mandates the response body and says
	// nothing about the status, so servers disagree (DV-01). Zero
	// accepts any 2xx.
	uploadStatus int
}

var profiles = map[string]profile{
	"stalwart": {name: "stalwart", uploadStatus: http.StatusOK},
	"fastmail": {name: "fastmail", uploadStatus: http.StatusOK},
	"cyrus":    {name: "cyrus", uploadStatus: http.StatusCreated},
}

// basicAuth adds the credentials to every request a Client makes,
// which is where RFC 8620 section 8 leaves authentication and where
// [jmap.Client] expects to find it.
type basicAuth struct {
	user, password string
}

func (a basicAuth) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.SetBasicAuth(a.user, a.password)
	return http.DefaultTransport.RoundTrip(clone)
}

// dial connects to the server the environment names, skipping when it
// names none. The skip is the same idiom the live suite uses for a
// missing token: a checkout with no server running still runs `go
// test` clean.
func dial(t *testing.T) *target {
	t.Helper()

	sessionURL := os.Getenv("POPLAR_JMAP_SESSION_URL")
	if sessionURL == "" {
		t.Skip("POPLAR_JMAP_SESSION_URL is unset; run make conformance")
	}

	// No client-wide timeout. http.Client.Timeout covers the body as
	// well as the headers, so one would cut the push stream off mid
	// connection, and [jmap.Client.Listen] holds that stream open for
	// as long as the caller's context allows. Every call here carries
	// a context instead, which is the bound that belongs per request.
	httpClient := &http.Client{
		Transport: basicAuth{
			user:     os.Getenv("POPLAR_JMAP_USER"),
			password: os.Getenv("POPLAR_JMAP_PASSWORD"),
		},
	}

	client := jmap.NewClient(sessionURL, httpClient)
	session, err := client.FetchSession(t.Context())
	if err != nil {
		t.Fatalf("FetchSession %s: %v", sessionURL, err)
	}

	account := session.PrimaryAccounts[jmap.MailURI]
	if account == "" {
		t.Fatalf("session names no primary mail account; accounts are %v", session.PrimaryAccounts)
	}

	return &target{
		client:     client,
		session:    session,
		account:    account,
		profile:    profiles[os.Getenv("POPLAR_JMAP_SERVER")],
		httpClient: httpClient,
	}
}

// do sends one request and fails the test when the server refuses the
// whole thing. A method that failed on its own is still in the
// response, because that is a per-call outcome and most tests here are
// about exactly those.
func (tg *target) do(t *testing.T, req *jmap.Request) *jmap.Response {
	t.Helper()

	resp, err := tg.client.Do(t.Context(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

// call runs one method and decodes its response into want. A method
// error fails the test, so a caller that wants to inspect one builds
// the request itself.
func (tg *target) call(t *testing.T, method jmap.Method, want any) {
	t.Helper()

	req := &jmap.Request{}
	callID := req.Invoke(method)
	answer := only(t, tg.do(t, req), callID)
	if failed, ok := answer.(*jmap.MethodError); ok {
		t.Fatalf("%s: %v", method.Name(), failed)
	}
	assign(t, answer, want)
}

// only returns the arguments of the single response under callID. Two
// responses under one id is legal (RFC 8620 section 3.4.1) and none is
// a failure, so both end the test rather than silently picking one.
func only(t *testing.T, resp *jmap.Response, callID string) any {
	t.Helper()

	found := resp.Invocations(callID)
	if len(found) != 1 {
		t.Fatalf("call %s answered %d times, want once", callID, len(found))
	}
	return found[0].Args
}

// assign copies one decoded response into the caller's own variable.
// The registry hands back an any holding a pointer to the response
// type, and every caller wants it typed.
func assign(t *testing.T, from, to any) {
	t.Helper()

	data, err := json.Marshal(from)
	if err != nil {
		t.Fatalf("re-marshal %T: %v", from, err)
	}
	if err := json.Unmarshal(data, to); err != nil {
		t.Fatalf("decode %T into %T: %v", from, to, err)
	}
}

// raw posts a request body as bytes and returns the response body as
// bytes, bypassing every type in the package. It is how a test asserts
// that what poplar exposes is what the server actually sent: comparing
// a decoded value against another decoded value would agree with
// itself.
func (tg *target) raw(t *testing.T, body string) json.RawMessage {
	t.Helper()

	resp, decoded := tg.post(t, tg.session.APIURL, "application/json", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw request answered %d: %s", resp.StatusCode, decoded)
	}
	return decoded
}

// post sends one body and returns the response alongside its bytes.
// The body is already read and closed, so only the header and the
// status survive on the response.
func (tg *target) post(t *testing.T, url, contentType, body string) (*http.Response, json.RawMessage) {
	t.Helper()

	//nolint:gosec // G704: the URL is the session's own or the operator's; pointing at a named server is what this suite does
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)

	//nolint:gosec // G704: same request, same reason
	resp, err := tg.httpClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return resp, json.RawMessage(data)
}

// rawSession fetches the session resource as bytes, so a test can
// compare what the package exposes against what the server sent.
func (tg *target) rawSession(t *testing.T) json.RawMessage {
	t.Helper()

	//nolint:gosec // G704: the session URL is the one the operator named in the environment
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		os.Getenv("POPLAR_JMAP_SESSION_URL"), nil)
	if err != nil {
		t.Fatalf("build session request: %v", err)
	}
	//nolint:gosec // G704: same request, same reason
	resp, err := tg.httpClient.Do(req)
	if err != nil {
		t.Fatalf("fetch the session as bytes: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the session: %v", err)
	}
	return data
}

// scratch names one test's records so two tests never collide and a
// leftover from an aborted run is recognisable.
var scratchCounter atomic.Uint64

func scratch(prefix string) string {
	return fmt.Sprintf("poplar-%s-%d-%d", prefix, time.Now().UnixNano()%1e9, scratchCounter.Add(1))
}

// newMailbox creates one mailbox and destroys it when the test ends,
// taking any message still in it along.
//
// A mailbox whose id is all digits is thrown back and another asked
// for. Such an id cannot be spelled in a patch pointer: RFC 8620
// section 5.3 forbids an array index there, and [jmap.Pointer] can
// only see the token's shape. RFC 8620 section 1.2 recommends servers
// avoid allocating those ids and calls the recommendation optional,
// and Stalwart's alphabet runs into the digits after 26 records, so
// the case is reachable rather than theoretical. Working around it
// here keeps the suite deterministic; the divergence itself is pinned
// by TestConformanceDigitOnlyIDsCannotBePatched.
func (tg *target) newMailbox(t *testing.T, name string) jmap.ID {
	t.Helper()

	// Twelve attempts, because Stalwart's alphabet runs the ten digits
	// consecutively and a run of them all lands in the unusable shape.
	for attempt := range 12 {
		// A retry needs its own name: the first mailbox is still there
		// until the test ends, and a server that forbids duplicates
		// would refuse the second.
		attempted := fmt.Sprintf("%s-%d", name, attempt)
		resp := &jmap.MailboxSetResponse{}
		tg.call(t, &jmap.MailboxSet{
			Account: tg.account,
			Create:  map[jmap.ID]*jmap.Mailbox{"m": {Name: attempted}},
		}, resp)

		created := resp.Created["m"]
		if created == nil {
			t.Fatalf("create mailbox %q: %v", attempted, resp.NotCreated["m"])
		}
		t.Cleanup(func() { tg.destroyMailbox(created.ID) })

		if !digitsOnly(string(created.ID)) {
			return created.ID
		}
		t.Logf("%s allocated mailbox id %q, which no patch pointer can address; asking for another (attempt %d)",
			tg.profile.name, created.ID, attempt+1)
	}
	t.Fatalf("%s allocated twelve digit-only mailbox ids in a row", tg.profile.name)
	return ""
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// destroyMailbox is cleanup, so it reports nothing: a test that has
// already failed leaves records behind either way, and the container
// goes when the run does.
func (tg *target) destroyMailbox(id jmap.ID) {
	req := &jmap.Request{}
	req.Invoke(&jmap.MailboxSet{
		Account:               tg.account,
		Destroy:               []jmap.ID{id},
		OnDestroyRemoveEmails: true,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = tg.client.Do(ctx, req)
}

// newEmail files one message into mailbox and returns its id. The
// message is built through Email/set create rather than uploaded,
// because every server has to accept the structured form.
func (tg *target) newEmail(t *testing.T, mailbox jmap.ID, subject string, keywords ...string) jmap.ID {
	t.Helper()

	keyed := make(map[string]bool, len(keywords))
	for _, k := range keywords {
		keyed[k] = true
	}

	resp := &jmap.EmailSetResponse{}
	tg.call(t, &jmap.EmailSet{
		Account: tg.account,
		Create:  map[jmap.ID]*jmap.Email{"e": tg.draft(mailbox, subject, keyed)},
	}, resp)

	created := resp.Created["e"]
	if created == nil {
		t.Fatalf("create email %q: %v", subject, resp.NotCreated["e"])
	}
	return created.ID
}

// draft builds the smallest message RFC 8621 section 4.6 accepts: one
// mailbox, one text part, and the value that part holds.
func (tg *target) draft(mailbox jmap.ID, subject string, keywords map[string]bool) *jmap.Email {
	return &jmap.Email{
		MailboxIDs: map[jmap.ID]bool{mailbox: true},
		Keywords:   keywords,
		Subject:    subject,
		From:       []*jmap.Address{{Name: "Conformance", Email: "user1@conformance.test"}},
		To:         []*jmap.Address{{Name: "Conformance", Email: "user1@conformance.test"}},
		BodyValues: map[string]*jmap.BodyValue{"b": {Value: "body of " + subject + "\n"}},
		TextBody:   []*jmap.BodyPart{{PartID: "b", Type: "text/plain"}},
	}
}

// update patches one message and fails unless the server applied it.
func (tg *target) update(t *testing.T, id jmap.ID, patch jmap.Patch) {
	t.Helper()

	resp := &jmap.EmailSetResponse{}
	tg.call(t, &jmap.EmailSet{
		Account: tg.account,
		Update:  map[jmap.ID]jmap.Patch{id: patch},
	}, resp)
	if refused, ok := resp.NotUpdated[id]; ok {
		t.Fatalf("update %s with %v: %v", id, patch, refused)
	}
	if _, ok := resp.Updated[id]; !ok {
		t.Fatalf("update %s with %v: the server neither applied it nor said why", id, patch)
	}
}

func (tg *target) destroyEmail(t *testing.T, id jmap.ID) {
	t.Helper()

	resp := &jmap.EmailSetResponse{}
	tg.call(t, &jmap.EmailSet{Account: tg.account, Destroy: []jmap.ID{id}}, resp)
	if refused, ok := resp.NotDestroyed[id]; ok {
		t.Fatalf("destroy %s: %v", id, refused)
	}
}

// destroyEmailQuietly is cleanup for a record a test did not expect
// the server to create.
func (tg *target) destroyEmailQuietly(id jmap.ID) {
	req := &jmap.Request{}
	req.Invoke(&jmap.EmailSet{Account: tg.account, Destroy: []jmap.ID{id}})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = tg.client.Do(ctx, req)
}

// emails fetches messages by id, which is how a test reads back what
// the server actually holds.
func (tg *target) emails(t *testing.T, ids ...jmap.ID) *jmap.EmailGetResponse {
	t.Helper()

	resp := &jmap.EmailGetResponse{}
	tg.call(t, &jmap.EmailGet{
		Account:    tg.account,
		IDs:        ids,
		Properties: []string{"id", "threadId", "mailboxIds", "keywords", "subject", "receivedAt"},
	}, resp)
	return resp
}

// emailState returns the Email state to page a later /changes from.
func (tg *target) emailState(t *testing.T) string {
	t.Helper()

	resp := &jmap.EmailGetResponse{}
	tg.call(t, &jmap.EmailGet{Account: tg.account, IDs: []jmap.ID{}}, resp)
	if resp.State == "" {
		t.Fatal("Email/get returned no state, so no /changes call has a starting point")
	}
	return resp.State
}

// mailbox returns one mailbox, or nil when the account no longer holds
// it.
func (tg *target) mailbox(t *testing.T, id jmap.ID) *jmap.Mailbox {
	t.Helper()

	resp := &jmap.MailboxGetResponse{}
	tg.call(t, &jmap.MailboxGet{Account: tg.account, IDs: []jmap.ID{id}}, resp)
	if len(resp.List) == 0 {
		return nil
	}
	return resp.List[0]
}

// inbox returns an id every account has, for a test that needs a
// second existing record to point at.
func (tg *target) inbox(t *testing.T) jmap.ID {
	t.Helper()

	resp := &jmap.MailboxQueryResponse{}
	tg.call(t, &jmap.MailboxQuery{
		Account: tg.account,
		Filter:  &jmap.MailboxFilterCondition{Role: jmap.RoleInbox},
	}, resp)
	if len(resp.IDs) == 0 {
		t.Fatal("the account has no inbox")
	}
	return resp.IDs[0]
}

// upload stores a blob and fails unless the server recorded one.
func (tg *target) upload(t *testing.T, contentType, payload string) *jmap.UploadResponse {
	t.Helper()

	resp, err := tg.client.Upload(t.Context(), tg.account, contentType, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if resp.BlobID == "" {
		t.Fatal("the server recorded no blob id")
	}
	return resp
}

func (tg *target) download(t *testing.T, blob jmap.ID) string {
	t.Helper()

	body, err := tg.client.Download(t.Context(), tg.account, blob, "application/octet-stream", "blob")
	if err != nil {
		t.Fatalf("Download %s: %v", blob, err)
	}
	defer func() { _ = body.Close() }()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read %s: %v", blob, err)
	}
	return string(data)
}

// provoke files a message into mailbox from another goroutine, so a
// test can read the push stream that the change lands on. The returned
// function waits for it to finish. Nothing is asserted here, because
// the assertion belongs to whatever the stream delivered.
func (tg *target) provoke(t *testing.T, mailbox jmap.ID) func() {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return
		}
		req := &jmap.Request{}
		req.Invoke(&jmap.EmailSet{
			Account: tg.account,
			Create:  map[jmap.ID]*jmap.Email{"e": tg.draft(mailbox, scratch("provoked"), nil)},
		})
		_, _ = tg.client.Do(ctx, req)
	}()
	return func() {
		cancel()
		<-done
	}
}

// A serverEvent is one server-sent event read off the wire, before
// any type in this package sees it.
type serverEvent struct {
	name string
	data string
	id   string
}

// readEventStream opens the session's event source with the query
// string given and returns the first count events, framed by the
// WHATWG rules rather than by this package's own reader. Reading the
// bytes directly is what lets a test assert whether a server sends an
// id field at all, which decides whether Last-Event-ID resumption is
// available (DV-09).
func (tg *target) readEventStream(t *testing.T, query string, count int) []serverEvent {
	t.Helper()

	url, _, _ := strings.Cut(tg.session.EventSourceURL, "?")
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"?"+query, nil)
	if err != nil {
		t.Fatalf("build the event source request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := tg.httpClient.Do(req)
	if err != nil {
		t.Fatalf("open the event source: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the event source answered %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("the event source answered content type %q", got)
	}

	var events []serverEvent
	var current serverEvent
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			if current.data != "" || current.name != "" {
				events = append(events, current)
			}
			current = serverEvent{}
			if len(events) >= count {
				return events
			}
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			current.name = value
		case "data":
			current.data += value
		case "id":
			current.id = value
		}
	}
	return events
}

// argumentsOf pulls the arguments object out of one invocation of a
// raw response body, without any type in this package touching it.
func argumentsOf(t *testing.T, body json.RawMessage, index int) json.RawMessage {
	t.Helper()

	var envelope struct {
		MethodResponses [][]json.RawMessage `json:"methodResponses"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode the response envelope: %v", err)
	}
	if index >= len(envelope.MethodResponses) {
		t.Fatalf("the response holds %d invocations, wanted number %d", len(envelope.MethodResponses), index)
	}
	invocation := envelope.MethodResponses[index]
	if len(invocation) != 3 {
		t.Fatalf("invocation %d has %d elements, want the name, arguments, call id triple", index, len(invocation))
	}
	return invocation[1]
}

// mapKeys is the ordering-free view of a JMAP set-valued property,
// which every one of them is: mailboxIds and keywords are objects
// mapping an id or a keyword to true.
func mapKeys[K comparable, V any](m map[K]V) iter.Seq[K] { return maps.Keys(m) }
