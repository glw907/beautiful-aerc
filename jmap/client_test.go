package jmap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// sessionTemplate is the session resource the fake server serves,
// with the test server's own base URL substituted in so a Client
// under test reaches the handlers registered beside it.
const sessionTemplate = `{
  "capabilities": {
    "urn:ietf:params:jmap:core": {
      "maxSizeUpload": 50000000,
      "maxConcurrentUpload": 4,
      "maxSizeRequest": 10000000,
      "maxConcurrentRequests": 4,
      "maxCallsInRequest": 16,
      "maxObjectsInGet": 500,
      "maxObjectsInSet": 500,
      "collationAlgorithms": ["i;ascii-numeric", "i;unicode-casemap"]
    },
    "urn:ietf:params:jmap:mail": {}
  },
  "accounts": {
    "A1": {
      "name": "user@example.com",
      "isPersonal": true,
      "isReadOnly": false,
      "accountCapabilities": {
        "urn:ietf:params:jmap:mail": {
          "maxMailboxesPerEmail": null,
          "maxMailboxDepth": 10,
          "maxSizeMailboxName": 490,
          "maxSizeAttachmentsPerEmail": 50000000,
          "emailQuerySortOptions": ["receivedAt"],
          "mayCreateTopLevelMailbox": true
        }
      }
    }
  },
  "primaryAccounts": {"urn:ietf:params:jmap:mail": "A1"},
  "username": "user@example.com",
  "apiUrl": "%[1]s/api",
  "downloadUrl": "%[1]s/download/{accountId}/{blobId}/{name}?accept={type}",
  "uploadUrl": "%[1]s/upload/{accountId}/",
  "eventSourceUrl": "%[1]s/events?types={types}&closeafter={closeafter}&ping={ping}",
  "state": "%[2]s"
}`

// startFake serves the session resource above and returns a Client
// pointed at it, along with the mux the test scripts the API, upload,
// and download endpoints on. Each fetch reports a new session state,
// so a test can tell one session from the next by what it says rather
// than by which pointer it is.
func startFake(t *testing.T) (*Client, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	var fetches atomic.Int64
	mux.HandleFunc("GET /session", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, sessionTemplate, srv.URL, fmt.Sprintf("s%d", fetches.Add(1)-1))
	})
	return NewClient(srv.URL+"/session", srv.Client()), mux
}

func dial(t *testing.T, c *Client) *Session {
	t.Helper()
	session, err := c.FetchSession(t.Context())
	if err != nil {
		t.Fatalf("FetchSession: %v", err)
	}
	return session
}

// serveJSON answers every request with one scripted body.
func serveJSON(status int, contentType, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

const emptyResponse = `{"methodResponses":[],"sessionState":"s0"}`

// echoRequest answers every call with response and keeps the request
// body, so a test can assert on the bytes that reached the wire.
func echoRequest(t *testing.T, response string) (http.HandlerFunc, func() string) {
	t.Helper()
	var mu sync.Mutex
	var seen string
	handler := func(w http.ResponseWriter, r *http.Request) {
		var body strings.Builder
		if _, err := io.Copy(&body, r.Body); err != nil {
			t.Errorf("read request body: %v", err)
		}
		mu.Lock()
		seen = body.String()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}
	return handler, func() string {
		mu.Lock()
		defer mu.Unlock()
		return seen
	}
}

// TestFetchSessionInstallsAtomically covers the half of JT-21 that is
// this package's: a refetched session replaces the cached one whole.
// Nothing reads a URL out of a half-installed session, which is the
// shape of the go-jmap race this transport was written to avoid.
// Deciding when to refetch is policy and belongs to the caller.
func TestFetchSessionInstallsAtomically(t *testing.T) {
	client, _ := startFake(t)

	if got := client.Session(); got != nil {
		t.Fatalf("Session() before a fetch = %v, want nil", got)
	}
	first := dial(t, client)
	if client.Session() != first {
		t.Error("Session() does not return the session FetchSession installed")
	}
	if first.APIURL == "" || first.PrimaryAccounts[MailURI] != "A1" {
		t.Errorf("session decoded as %+v, want an api url and the mail account", first)
	}
	if first.State != "s0" {
		t.Errorf("first session State = %q, want s0", first.State)
	}

	second := dial(t, client)
	if second.State != "s1" {
		t.Errorf("second session State = %q, want s1; the refetch did not read the server", second.State)
	}
	if got := client.Session(); got.State != "s1" {
		t.Errorf("Session() reports State %q, want the replacement's s1", got.State)
	}
}

// TestClientRefusesCallsWithoutASession proves every call that needs a
// URL out of the session says so, rather than posting to an empty URL
// or fetching the session behind the caller's back.
func TestClientRefusesCallsWithoutASession(t *testing.T) {
	client, _ := startFake(t)

	cases := []struct {
		name string
		call func() error
	}{
		{"Do", func() error { _, err := client.Do(t.Context(), &Request{}); return err }},
		{"Upload", func() error {
			_, err := client.Upload(t.Context(), "A1", "text/plain", strings.NewReader("x"))
			return err
		}},
		{"Download", func() error {
			_, err := client.Download(t.Context(), "A1", "B1", "text/plain", "note.txt")
			return err
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); !errors.Is(err, ErrNoSession) {
				t.Errorf("%s error = %v, want ErrNoSession", c.name, err)
			}
		})
	}
}

// TestDoRefusesAnUnmarshalableRequest proves a request this package
// will not build never reaches the wire. RFC 8620 section 5.3's
// pointer restrictions are enforced when a Patch marshals, so the
// failure surfaces here, at the one call in Do whose error is not the
// network's, and it has to say which step it came from.
func TestDoRefusesAnUnmarshalableRequest(t *testing.T) {
	client, mux := startFake(t)
	var calls atomic.Int64
	mux.HandleFunc("POST /api", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, emptyResponse)
	})
	dial(t, client)

	req := &Request{}
	req.Invoke(&EmailSet{Account: "A1", Update: map[ID]Patch{
		"M1": {Pointer("mailboxIds", "0"): nil},
	}})

	resp, err := client.Do(t.Context(), req)
	if err == nil {
		t.Fatalf("Do = %+v, want the patch to be refused", resp)
	}
	if !strings.Contains(err.Error(), "marshal request") {
		t.Errorf("Do error = %q, want it to name the step that failed", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("the server saw %d calls, want 0; the request reached the wire", got)
	}
}

// TestDoStreamsTheResponse pins the streaming decoder. A server that
// sends a complete response and then holds the connection open is
// answered; buffering the whole body first would wait for a close that
// never comes.
func TestDoStreamsTheResponse(t *testing.T) {
	client, mux := startFake(t)
	release := make(chan struct{})
	var stalled sync.WaitGroup
	stalled.Add(1)
	mux.HandleFunc("POST /api", func(w http.ResponseWriter, _ *http.Request) {
		defer stalled.Done()
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("the test server's ResponseWriter cannot flush")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, emptyResponse)
		flusher.Flush()
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
	})
	dial(t, client)

	done := make(chan error, 1)
	go func() {
		_, err := client.Do(t.Context(), &Request{})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Do: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Do did not return while the server held the response body open")
	}
	close(release)
	stalled.Wait()
}

// TestDoDecodesProblemDetails covers JT-29. go-jmap required the
// content type to equal application/json exactly, so the
// application/problem+json that RFC 7807 specifies, and any charset
// parameter, reduced the server's only diagnostic to a bare status.
func TestDoDecodesProblemDetails(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
		wantType    string
		wantLimit   string
	}{
		{
			name:        "problem+json",
			status:      http.StatusBadRequest,
			contentType: "application/problem+json",
			body:        string(readFixture(t, "rfc8620-3.6.1.1-unknowncapability.json")),
			wantType:    "urn:ietf:params:jmap:error:unknownCapability",
		},
		{
			name:        "plain json",
			status:      http.StatusBadRequest,
			contentType: "application/json",
			body:        string(readFixture(t, "rfc8620-3.6.1.1-limit.json")),
			wantType:    "urn:ietf:params:jmap:error:limit",
			wantLimit:   "maxSizeRequest",
		},
		{
			name:        "problem+json with a charset",
			status:      http.StatusBadRequest,
			contentType: "application/problem+json; charset=utf-8",
			body:        string(readFixture(t, "rfc8620-3.6.1.1-limit.json")),
			wantType:    "urn:ietf:params:jmap:error:limit",
			wantLimit:   "maxSizeRequest",
		},
		{
			name:        "json with a charset",
			status:      http.StatusBadRequest,
			contentType: "application/json; charset=utf-8",
			body:        string(readFixture(t, "rfc8620-3.6.1.1-unknowncapability.json")),
			wantType:    "urn:ietf:params:jmap:error:unknownCapability",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, mux := startFake(t)
			mux.HandleFunc("POST /api", serveJSON(c.status, c.contentType, c.body))
			dial(t, client)

			_, err := client.Do(t.Context(), &Request{})
			var reqErr *RequestError
			if !errors.As(err, &reqErr) {
				t.Fatalf("Do error = %v (%T), want a *RequestError", err, err)
			}
			if reqErr.Type != c.wantType {
				t.Errorf("RequestError.Type = %q, want %q", reqErr.Type, c.wantType)
			}
			if reqErr.Limit != c.wantLimit {
				t.Errorf("RequestError.Limit = %q, want %q", reqErr.Limit, c.wantLimit)
			}
		})
	}
}

// TestDoDegradesToAnHTTPError covers JT-29's other half: a body that
// is not problem details still reaches the caller as a typed failure
// carrying the status, never as a success.
func TestDoDegradesToAnHTTPError(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		contentType string
		body        string
	}{
		{"html error page", http.StatusBadGateway, "text/html", "<html>bad gateway</html>"},
		{"empty body", http.StatusServiceUnavailable, "", ""},
		{"json content type over a non-json body", http.StatusInternalServerError, "application/json", "not json"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, mux := startFake(t)
			mux.HandleFunc("POST /api", serveJSON(c.status, c.contentType, c.body))
			dial(t, client)

			_, err := client.Do(t.Context(), &Request{})
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("Do error = %v (%T), want an *HTTPError", err, err)
			}
			if httpErr.Status != c.status {
				t.Errorf("HTTPError.Status = %d, want %d", httpErr.Status, c.status)
			}
			if !strings.Contains(err.Error(), fmt.Sprint(c.status)) {
				t.Errorf("error text %q does not name the status", err)
			}
		})
	}
}

// TestFetchSessionSurfacesTheServerRefusal proves session discovery
// reads the same two error shapes the API endpoint does. An expired
// token is the common case, and the problem details are what say so.
func TestFetchSessionSurfacesTheServerRefusal(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("GET /problem", serveJSON(http.StatusUnauthorized, "application/problem+json",
		`{"type":"about:blank","detail":"token expired"}`))
	mux.HandleFunc("GET /html", serveJSON(http.StatusForbidden, "text/html", "<html>no</html>"))

	t.Run("problem details", func(t *testing.T) {
		_, err := NewClient(srv.URL+"/problem", srv.Client()).FetchSession(t.Context())
		var reqErr *RequestError
		if !errors.As(err, &reqErr) {
			t.Fatalf("FetchSession error = %v (%T), want a *RequestError", err, err)
		}
		if reqErr.Status != http.StatusUnauthorized {
			t.Errorf("RequestError.Status = %d, want %d; a body that omits it leaves the caller nothing to classify on",
				reqErr.Status, http.StatusUnauthorized)
		}
		if reqErr.Detail != "token expired" {
			t.Errorf("RequestError.Detail = %q, want %q", reqErr.Detail, "token expired")
		}
	})

	t.Run("no problem details", func(t *testing.T) {
		_, err := NewClient(srv.URL+"/html", srv.Client()).FetchSession(t.Context())
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) {
			t.Fatalf("FetchSession error = %v (%T), want an *HTTPError", err, err)
		}
		if httpErr.Status != http.StatusForbidden {
			t.Errorf("HTTPError.Status = %d, want %d", httpErr.Status, http.StatusForbidden)
		}
	})
}

// TestCallsHonourTheContext proves every request carries the caller's
// context, so a deadline or a cancellation actually unblocks a call
// against a server that has stopped answering.
func TestCallsHonourTheContext(t *testing.T) {
	client, mux := startFake(t)
	hang := func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}
	mux.HandleFunc("POST /api", hang)
	mux.HandleFunc("/upload/", hang)
	mux.HandleFunc("/download/", hang)
	dial(t, client)

	cases := []struct {
		name string
		call func(context.Context) error
	}{
		{"Do", func(ctx context.Context) error { _, err := client.Do(ctx, &Request{}); return err }},
		{"Upload", func(ctx context.Context) error {
			_, err := client.Upload(ctx, "A1", "text/plain", strings.NewReader("x"))
			return err
		}},
		{"Download", func(ctx context.Context) error {
			_, err := client.Download(ctx, "A1", "B1", "text/plain", "note.txt")
			return err
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			done := make(chan error, 1)
			go func() { done <- c.call(ctx) }()
			cancel()

			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("%s error = %v, want context.Canceled", c.name, err)
				}
			case <-time.After(3 * time.Second):
				t.Errorf("%s did not return after its context was cancelled", c.name)
			}
		})
	}
}

// TestDownloadExpandsTheURLTemplate covers JT-32's template half. RFC
// 8620 section 6.2 gives the download URL as an RFC 6570 level 1
// template, whose simple string expansion percent-encodes everything
// outside the unreserved set. A media type carries a slash and a file
// name carries anything at all, so leaving them raw builds a different
// URL than the one asked for.
func TestDownloadExpandsTheURLTemplate(t *testing.T) {
	client, mux := startFake(t)
	var gotPath, gotQuery string
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.RawQuery
		_, _ = io.WriteString(w, "data")
	})
	dial(t, client)

	body, err := client.Download(t.Context(), "A1", "B/1#2", "text/plain", "réponse note.txt")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Fatalf("close body: %v", err)
	}

	wantPath := "/download/A1/B%2F1%232/r%C3%A9ponse%20note.txt"
	if gotPath != wantPath {
		t.Errorf("download path = %q, want %q", gotPath, wantPath)
	}
	if wantQuery := "accept=text%2Fplain"; gotQuery != wantQuery {
		t.Errorf("download query = %q, want %q", gotQuery, wantQuery)
	}
}

// TestDownloadSurfacesAServerRefusal proves a failed download comes
// back as an error rather than as a reader over the error page.
func TestDownloadSurfacesAServerRefusal(t *testing.T) {
	client, mux := startFake(t)
	mux.HandleFunc("/download/", serveJSON(http.StatusNotFound, "application/problem+json",
		`{"type":"about:blank","status":404,"detail":"no such blob"}`))
	dial(t, client)

	body, err := client.Download(t.Context(), "A1", "B1", "text/plain", "note.txt")
	if err == nil {
		_ = body.Close()
		t.Fatal("Download returned a reader over a 404")
	}
	if body != nil {
		t.Error("Download returned a reader alongside its error")
	}
	var reqErr *RequestError
	if !errors.As(err, &reqErr) || reqErr.Detail != "no such blob" {
		t.Errorf("Download error = %v (%T), want the server's problem details", err, err)
	}
}

// TestUploadSendsTheCallersContentType pins RFC 8620 section 6.1's
// contract that the server records the blob's media type from this
// header. go-jmap sent application/json for every upload, so every
// attachment it stored was mislabelled.
func TestUploadSendsTheCallersContentType(t *testing.T) {
	client, mux := startFake(t)
	var gotType, gotBody string
	mux.HandleFunc("/upload/", func(w http.ResponseWriter, r *http.Request) {
		gotType = r.Header.Get("Content-Type")
		var body strings.Builder
		if _, err := io.Copy(&body, r.Body); err != nil {
			t.Errorf("read upload body: %v", err)
		}
		gotBody = body.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"accountId":"A1","blobId":"B1","type":"image/png","size":4}`)
	})
	dial(t, client)

	if _, err := client.Upload(t.Context(), "A1", "image/png", strings.NewReader("\x89PNG")); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if gotType != "image/png" {
		t.Errorf("upload Content-Type = %q, want image/png", gotType)
	}
	if gotBody != "\x89PNG" {
		t.Errorf("upload body = %q, want the bytes the caller passed", gotBody)
	}
}

// TestClientReadsTheSessionCoherently is JT-21's atomicity claim under
// contention: calls run against a session being replaced underneath
// them. Each takes one snapshot and works from it, so a refetch
// mid-flight cannot leave a call reading one session's API URL and
// another's account. The race detector in CI is what turns a
// regression here into a failure.
func TestClientReadsTheSessionCoherently(t *testing.T) {
	client, mux := startFake(t)
	mux.HandleFunc("POST /api", serveJSON(http.StatusOK, "application/json", emptyResponse))
	dial(t, client)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 20 {
				if _, err := client.FetchSession(t.Context()); err != nil {
					t.Errorf("FetchSession: %v", err)
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			for range 20 {
				if _, err := client.Do(t.Context(), &Request{}); err != nil {
					t.Errorf("Do: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
