package jmap

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The tests below are the reliance spec of the 2026-07-28 JMAP library
// decision, section 3a: every transport behaviour a caller depends on,
// driven against a scripted server rather than written down in prose.
// A replacement for this package has to pass them.

// TestRelianceCoreLimits reads the RFC 8620 section 2 core capability
// out of a live session. A mistyped tag here reports a limit of zero,
// which a caller reads as "batch nothing" or, worse, ignores.
func TestRelianceCoreLimits(t *testing.T) {
	client, _ := startFake(t)
	session := dial(t, client)

	core, ok := session.Capabilities[CoreURI].(*Core)
	if !ok {
		t.Fatalf("session core capability is %T, want *Core", session.Capabilities[CoreURI])
	}

	limits := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"MaxSizeUpload", core.MaxSizeUpload, 50000000},
		{"MaxConcurrentUpload", core.MaxConcurrentUpload, 4},
		{"MaxSizeRequest", core.MaxSizeRequest, 10000000},
		{"MaxConcurrentRequests", core.MaxConcurrentRequests, 4},
		{"MaxCallsInRequest", core.MaxCallsInRequest, 16},
		{"MaxObjectsInGet", core.MaxObjectsInGet, 500},
		{"MaxObjectsInSet", core.MaxObjectsInSet, 500},
	}
	for _, limit := range limits {
		if limit.got != limit.want {
			t.Errorf("Core.%s = %d, want %d", limit.name, limit.got, limit.want)
		}
	}
	if want := []CollationAlgo{ASCIINumeric, UnicodeCasemap}; !slices.Equal(core.CollationAlgorithms, want) {
		t.Errorf("Core.CollationAlgorithms = %v, want %v", core.CollationAlgorithms, want)
	}
}

// TestRelianceForcedCoreCapability proves every request names the core
// capability, whatever the caller assembled. RFC 8620 section 3.3
// makes a server reject a request whose "using" omits a capability the
// call needs, and forcing it in the copy leaves the caller's own
// Request untouched, so a Request shared between goroutines is not
// written to under one of them.
func TestRelianceForcedCoreCapability(t *testing.T) {
	client, mux := startFake(t)
	handler, body := echoRequest(t, emptyResponse)
	mux.HandleFunc("POST /api", handler)
	dial(t, client)

	req := &Request{Using: []URI{MailURI}}
	if _, err := client.Do(t.Context(), req); err != nil {
		t.Fatalf("Do: %v", err)
	}

	var sent struct {
		Using []URI `json:"using"`
	}
	if err := json.Unmarshal([]byte(body()), &sent); err != nil {
		t.Fatalf("decode sent request: %v", err)
	}
	if want := []URI{CoreURI, MailURI}; !slices.Equal(sent.Using, want) {
		t.Errorf(`sent "using" = %v, want %v`, sent.Using, want)
	}
	if want := []URI{MailURI}; !slices.Equal(req.Using, want) {
		t.Errorf("Do rewrote the caller's Using to %v, want %v left alone", req.Using, want)
	}
}

// TestRelianceBackReferenceBatch drives the changes-plus-get pattern
// poplar's sync leans on hardest: one request whose second call takes
// its ids from the first call's result. The reference has to reach the
// wire in its "#" form, and the ids it names have to be the ones the
// server worked from.
func TestRelianceBackReferenceBatch(t *testing.T) {
	client, mux := startFake(t)
	scripted := `{
	  "methodResponses": [
	    ["Mailbox/changes", {
	      "accountId": "A1", "oldState": "s1", "newState": "s2",
	      "created": ["MA", "MB"], "updated": [], "destroyed": []
	    }, "0"],
	    ["Mailbox/get", {
	      "accountId": "A1", "state": "s2",
	      "list": [{"id": "MA", "name": "Inbox"}, {"id": "MB", "name": "Archive"}],
	      "notFound": []
	    }, "1"]
	  ],
	  "sessionState": "s0"
	}`
	handler, body := echoRequest(t, scripted)
	mux.HandleFunc("POST /api", handler)
	dial(t, client)

	req := &Request{}
	changesID := req.Invoke(&MailboxChanges{Account: "A1", SinceState: "s1"})
	req.Invoke(&MailboxGet{Account: "A1", ReferenceIDs: &ResultReference{
		ResultOf: changesID,
		Name:     "Mailbox/changes",
		Path:     "/created",
	}})

	resp, err := client.Do(t.Context(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	const wantReference = `"#ids":{"resultOf":"0","name":"Mailbox/changes","path":"/created"}`
	if !strings.Contains(body(), wantReference) {
		t.Errorf("sent request %s does not carry %s", body(), wantReference)
	}

	referenced, err := resp.Resolve(ResultReference{
		ResultOf: changesID,
		Name:     "Mailbox/changes",
		Path:     "/created",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	var ids []ID
	if err := json.Unmarshal(referenced, &ids); err != nil {
		t.Fatalf("resolved value is not an id list: %v", err)
	}

	fetched := argsOf[*MailboxGetResponse](t, resp, "1")
	got := make([]ID, len(fetched.List))
	for i, box := range fetched.List {
		got[i] = box.ID
	}
	if !slices.Equal(got, ids) {
		t.Errorf("Mailbox/get returned %v, want the referenced ids %v", got, ids)
	}
}

// TestReliancePatchOnSet proves an update reaches the wire as the
// per-leaf PatchObject of RFC 8620 section 5.3. A whole-property
// replacement in its place clears every keyword but the one being set,
// or files the message out of every mailbox, and the call still reports
// success.
func TestReliancePatchOnSet(t *testing.T) {
	client, mux := startFake(t)
	handler, body := echoRequest(t, emptyResponse)
	mux.HandleFunc("POST /api", handler)
	dial(t, client)

	req := &Request{}
	req.Invoke(&EmailSet{Account: "A1", Update: map[ID]Patch{"M1": {
		Pointer("keywords", "$seen"):  true,
		Pointer("mailboxIds", "MA"):   nil,
		Pointer("mailboxIds", "MB"):   true,
		Pointer("keywords", "$draft"): nil,
	}}})
	if _, err := client.Do(t.Context(), req); err != nil {
		t.Fatalf("Do: %v", err)
	}

	sent := body()
	for _, want := range []string{
		`"keywords/$seen":true`,
		`"keywords/$draft":null`,
		`"mailboxIds/MA":null`,
		`"mailboxIds/MB":true`,
	} {
		if !strings.Contains(sent, want) {
			t.Errorf("sent request %s does not carry %s", sent, want)
		}
	}
	for _, unwanted := range []string{`"keywords":`, `"mailboxIds":`} {
		if strings.Contains(sent, unwanted) {
			t.Errorf("sent request %s replaces a whole property with %s", sent, unwanted)
		}
	}
}

// TestRelianceThreeErrorShapes proves JMAP's three refusals stay
// distinguishable. A request the server never ran is an error from Do;
// a method that failed is an error inside a successful response; a
// record a /set refused is an error inside a successful method
// response, whose state still advanced. Collapsing the second or third
// into "the call worked" is how a message reads as sent when it was
// not.
func TestRelianceThreeErrorShapes(t *testing.T) {
	t.Run("request level", func(t *testing.T) {
		client, mux := startFake(t)
		mux.HandleFunc("POST /api", serveJSON(http.StatusBadRequest, "application/problem+json",
			string(readFixture(t, "rfc8620-3.6.1.1-limit.json"))))
		dial(t, client)

		resp, err := client.Do(t.Context(), &Request{})
		if resp != nil {
			t.Error("Do returned a response alongside a request-level error")
		}
		var reqErr *RequestError
		if !errors.As(err, &reqErr) {
			t.Fatalf("Do error = %v (%T), want a *RequestError", err, err)
		}
		if reqErr.Limit != "maxSizeRequest" {
			t.Errorf("RequestError.Limit = %q, want maxSizeRequest", reqErr.Limit)
		}
	})

	t.Run("method level", func(t *testing.T) {
		client, mux := startFake(t)
		mux.HandleFunc("POST /api", serveJSON(http.StatusOK, "application/json", `{
		  "methodResponses": [
		    ["error", {"type": "cannotCalculateChanges"}, "0"],
		    ["Mailbox/get", {"accountId": "A1", "state": "s2", "list": []}, "1"]
		  ],
		  "sessionState": "s0"
		}`))
		dial(t, client)

		resp, err := client.Do(t.Context(), &Request{})
		if err != nil {
			t.Fatalf("Do: %v; a failed method is not a failed request", err)
		}
		if !errors.Is(argsOf[*MethodError](t, resp, "0"), ErrCannotCalculateChanges) {
			t.Error("the failed call did not decode to cannotCalculateChanges")
		}
		if got := argsOf[*MailboxGetResponse](t, resp, "1").State; got != "s2" {
			t.Errorf("the call after the failure decoded State = %q, want s2", got)
		}
	})

	t.Run("record level", func(t *testing.T) {
		client, mux := startFake(t)
		mux.HandleFunc("POST /api", serveJSON(http.StatusOK, "application/json", `{
		  "methodResponses": [
		    ["Mailbox/set", {
		      "accountId": "A1", "oldState": "s1", "newState": "s2",
		      "created": {"k1": {"id": "MA"}},
		      "notCreated": {
		        "k2": {"type": "invalidProperties", "properties": ["name"],
		               "description": "name is already taken"}
		      }
		    }, "0"]
		  ],
		  "sessionState": "s0"
		}`))
		dial(t, client)

		resp, err := client.Do(t.Context(), &Request{})
		if err != nil {
			t.Fatalf("Do: %v; a refused record is not a failed request", err)
		}
		set := argsOf[*MailboxSetResponse](t, resp, "0")
		if set.NewState != "s2" {
			t.Errorf("NewState = %q, want s2; the state advances even when a record was refused", set.NewState)
		}
		if len(set.Created) != 1 {
			t.Errorf("Created holds %d records, want 1", len(set.Created))
		}
		refused, ok := set.NotCreated["k2"]
		if !ok {
			t.Fatal("NotCreated does not name the refused creation id")
		}
		var asError error = refused
		if !strings.Contains(asError.Error(), "name is already taken") {
			t.Errorf("SetError as an error reads %q, want the server's description", asError)
		}
		if want := []string{"name"}; !slices.Equal(refused.Properties, want) {
			t.Errorf("SetError.Properties = %v, want %v", refused.Properties, want)
		}
	})
}

// TestRelianceUploadStatusHandling covers DV-01 and JT-32. RFC 8620
// section 6.1 mandates the body and says nothing about the status:
// Cyrus answers 201 and adds a fifth property, Fastmail and Stalwart
// answer 200. go-jmap accepted 200 alone, so uploading to Cyrus failed.
func TestRelianceUploadStatusHandling(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"200", http.StatusOK, `{"accountId":"A1","blobId":"B1","type":"text/plain","size":5}`},
		{"201", http.StatusCreated, `{"accountId":"A1","blobId":"B1","type":"text/plain","size":5}`},
		{"202", http.StatusAccepted, `{"accountId":"A1","blobId":"B1","type":"text/plain","size":5}`},
		{
			name:   "200 with a property beyond the four the RFC names",
			status: http.StatusOK,
			body: `{"accountId":"A1","blobId":"B1","type":"text/plain","size":5,` +
				`"expires":"2026-07-29T12:00:00Z"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, mux := startFake(t)
			mux.HandleFunc("/upload/", serveJSON(c.status, "application/json", c.body))
			dial(t, client)

			uploaded, err := client.Upload(t.Context(), "A1", "text/plain", strings.NewReader("hello"))
			if err != nil {
				t.Fatalf("Upload: %v", err)
			}
			if uploaded.Account != "A1" || uploaded.BlobID != "B1" {
				t.Errorf("upload recorded %+v, want account A1 and blob B1", uploaded)
			}
			if uploaded.Type != "text/plain" || uploaded.Size != 5 {
				t.Errorf("upload recorded type %q size %d, want text/plain and 5", uploaded.Type, uploaded.Size)
			}
		})
	}

	t.Run("a refusal is still an error", func(t *testing.T) {
		client, mux := startFake(t)
		mux.HandleFunc("/upload/", serveJSON(http.StatusRequestEntityTooLarge, "application/problem+json",
			`{"type":"urn:ietf:params:jmap:error:limit","limit":"maxSizeUpload","status":413,"detail":"too big"}`))
		dial(t, client)

		uploaded, err := client.Upload(t.Context(), "A1", "text/plain", strings.NewReader("hello"))
		if uploaded != nil {
			t.Error("Upload returned a blob alongside its error")
		}
		var reqErr *RequestError
		if !errors.As(err, &reqErr) || reqErr.Limit != "maxSizeUpload" {
			t.Errorf("Upload error = %v (%T), want the server's limit problem details", err, err)
		}
	})
}

// TestRelianceStreamingDownload proves a download is a stream. An
// attachment is arbitrarily large, and buffering one before the caller
// sees a byte trades a bounded memory cost for an unbounded one.
func TestRelianceStreamingDownload(t *testing.T) {
	client, mux := startFake(t)
	release := make(chan struct{})
	var served sync.WaitGroup
	served.Add(1)
	mux.HandleFunc("/download/", func(w http.ResponseWriter, _ *http.Request) {
		defer served.Done()
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("the test server's ResponseWriter cannot flush")
			return
		}
		_, _ = io.WriteString(w, "head")
		flusher.Flush()
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
		_, _ = io.WriteString(w, "tail")
	})
	dial(t, client)

	head := make(chan string, 1)
	rest := make(chan string, 1)
	failed := make(chan error, 1)
	go func() {
		body, err := client.Download(t.Context(), "A1", "B1", "text/plain", "note.txt")
		if err != nil {
			failed <- err
			return
		}
		defer func() { _ = body.Close() }()

		prefix := make([]byte, 4)
		if _, err := io.ReadFull(body, prefix); err != nil {
			failed <- err
			return
		}
		head <- string(prefix)

		var tail strings.Builder
		if _, err := io.Copy(&tail, body); err != nil {
			failed <- err
			return
		}
		rest <- tail.String()
	}()

	select {
	case got := <-head:
		if got != "head" {
			t.Errorf("first bytes = %q, want head", got)
		}
	case err := <-failed:
		t.Fatalf("Download: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("the first bytes never arrived while the server held the rest back; the body was buffered, not streamed")
	}

	close(release)
	select {
	case got := <-rest:
		if got != "tail" {
			t.Errorf("remaining bytes = %q, want tail", got)
		}
	case err := <-failed:
		t.Fatalf("read the rest of the body: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("the rest of the body never arrived")
	}
	served.Wait()
}
