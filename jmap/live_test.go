//go:build live

// The live suite runs against a real Fastmail account, by hand and
// never in CI. It is deliberately small: the conformance suite covers
// everything a second server can prove, and what is left is the three
// things only Fastmail can answer, per the test inventory's section 3.
// Fastmail's blob upload status and response fields, the shape of its
// session state, and whether its event source honours Last-Event-ID.
//
// Everything here is read-only except one blob upload, which RFC 8620
// section 6.1 lets a server expire after an hour and which no mailbox
// ever sees.
package jmap_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/jmap"
)

// fastmailSessionURL is Fastmail's JMAP session discovery endpoint.
const fastmailSessionURL = "https://api.fastmail.com/jmap/session"

// bearer carries the API token, which is the scheme Fastmail uses
// where RFC 8620 section 8 leaves authentication to the server.
type bearer struct {
	token string
}

func (b bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

// live connects to the account, skipping when the token is not in the
// environment. The skip covers a checkout where the secret was never
// sourced.
func live(t *testing.T) (*jmap.Client, *jmap.Session, *http.Client) {
	t.Helper()

	token := os.Getenv("FASTMAIL_API_TOKEN")
	if token == "" {
		t.Skip("FASTMAIL_API_TOKEN is unset")
	}

	// No client-wide timeout: it would cover the body too, and cut the
	// push stream off mid connection. Every call carries a context.
	httpClient := &http.Client{Transport: bearer{token: token}}
	client := jmap.NewClient(fastmailSessionURL, httpClient)
	session, err := client.FetchSession(t.Context())
	if err != nil {
		t.Fatalf("FetchSession: %v", err)
	}
	return client, session, httpClient
}

// TestLiveUploadStatusAndFields is the first of the three: DV-01
// against the server poplar actually ships for. Only this account can
// say what status Fastmail answers with and which properties it sends
// beyond the four RFC 8620 section 6.1 defines.
func TestLiveUploadStatusAndFields(t *testing.T) {
	client, session, httpClient := live(t)

	account := session.PrimaryAccounts[jmap.MailURI]
	if account == "" {
		t.Fatal("the session names no primary mail account")
	}

	const payload = "poplar live upload probe\n"
	url := strings.ReplaceAll(session.UploadURL, "{accountId}", string(account))
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("build the upload request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read the upload response: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("upload answered %d: %s", resp.StatusCode, body)
	}

	var sent map[string]json.RawMessage
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decode the upload response: %v", err)
	}
	for _, property := range []string{"accountId", "blobId", "type", "size"} {
		if _, ok := sent[property]; !ok {
			t.Errorf("the upload response omits %q, which section 6.1 makes mandatory", property)
		}
	}
	t.Logf("fastmail answers an upload with %d and the properties %v", resp.StatusCode, propertyNames(sent))

	// The same upload through the package, and the bytes back out of
	// it, which is what proves the status handling and the download
	// template agree with the live server.
	uploaded, err := client.Upload(t.Context(), account, "text/plain", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if uploaded.Size != uint64(len(payload)) {
		t.Errorf("Upload reported size %d for %d bytes", uploaded.Size, len(payload))
	}
	if uploaded.Type != "text/plain" {
		t.Errorf("Upload reported type %q, want the one it sent", uploaded.Type)
	}

	stored, err := client.Download(t.Context(), account, uploaded.BlobID, "text/plain", "probe.txt")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer func() { _ = stored.Close() }()
	got, err := io.ReadAll(stored)
	if err != nil {
		t.Fatalf("read the blob: %v", err)
	}
	if string(got) != payload {
		t.Errorf("the stored blob reads %q, want %q", got, payload)
	}
}

func propertyNames(m map[string]json.RawMessage) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

// TestLiveSessionStateIsOpaque is the second: JT-18 against the value
// that made the rule worth having. Fastmail's session state visibly
// encodes a Cyrus generation number, and a client that read that
// structure would break the day Fastmail changed it.
func TestLiveSessionStateIsOpaque(t *testing.T) {
	client, session, httpClient := live(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fastmailSessionURL, nil)
	if err != nil {
		t.Fatalf("build the session request: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("fetch the session as bytes: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var wire struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		t.Fatalf("decode the session: %v", err)
	}
	if wire.State == "" {
		t.Fatal("the live session carries no state")
	}
	if session.State != wire.State {
		t.Errorf("the session exposes state %q while the server sent %q", session.State, wire.State)
	}

	echo := &jmap.Request{}
	echo.Invoke(jmap.Echo{"live": true})
	answered, err := client.Do(t.Context(), echo)
	if err != nil {
		t.Fatalf("Core/echo: %v", err)
	}
	if answered.SessionState != wire.State {
		t.Errorf("a response names session state %q while the session says %q",
			answered.SessionState, wire.State)
	}
	// The structure is the point: it is there, poplar carries it
	// whole, and nothing in the package reads a field out of it.
	t.Logf("fastmail spells its session state %q, in %d semicolon-separated parts",
		wire.State, strings.Count(wire.State, ";")+1)
}

// TestLiveEventSourceResumption is the third: whether Fastmail sends
// an id field, which decides whether Last-Event-ID resumption is
// available at all. RFC 8620 section 7.3 makes replaying missed events
// a SHOULD and defines no error for a server that remembers nothing,
// so poplar treats every new connection as a gap either way.
func TestLiveEventSourceResumption(t *testing.T) {
	client, session, httpClient := live(t)

	if session.EventSourceURL == "" {
		t.Fatal("the live session advertises no event source")
	}

	url, _, _ := strings.Cut(session.EventSourceURL, "?")
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"?types=*&closeafter=no&ping=30", nil)
	if err != nil {
		t.Fatalf("build the event source request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := httpClient.Do(req)
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

	events := readServerEvents(resp.Body, 1)
	if len(events) == 0 {
		t.Fatal("the live event source sent nothing within the window")
	}
	first := events[0]
	if first.id == "" {
		t.Logf("fastmail sent a %q event with no id field; Last-Event-ID has nothing to resume from", first.name)
	} else {
		t.Logf("fastmail sent a %q event carrying id %q, so a reconnect can ask to resume", first.name, first.id)
	}

	// Whatever the answer, the contract poplar rests on is that a
	// connection is announced before any event, so the caller treats
	// the window it was down as a gap and pulls /changes.
	connected := make(chan struct{}, 1)
	listenCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- client.Listen(listenCtx, jmap.EventSource{
			Ping:      30 * time.Second,
			OnConnect: func() { connected <- struct{}{} },
		})
	}()
	select {
	case <-connected:
	case err := <-done:
		t.Fatalf("Listen returned before connecting: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("Listen never announced a connection")
	}
	stop()
	if err := <-done; !errorIsContextCancelled(err) {
		t.Errorf("Listen returned %v, want the cancelled context", err)
	}
}

func errorIsContextCancelled(err error) bool {
	return err != nil && strings.Contains(err.Error(), context.Canceled.Error())
}
