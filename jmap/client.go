package jmap

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
)

// A Client speaks JMAP to one server: the session resource, the API
// endpoint the session names, and the blob upload and download
// endpoints beside it.
//
// Authentication belongs to the http.Client. Pass one whose Transport
// sets the credential header, and every request a Client makes carries
// it, including a session refetch, which lets a token refresh reach the
// wire without the Client knowing a token exists.
//
// A Client returns errors and does nothing else with them. It does not
// retry, does not back off, and does not log, because a retry that
// looks right for a sync poll is wrong for a user waiting on a send,
// and only the caller knows which it is.
//
// # Session state
//
// The session is the only thing a Client mutates, [Client.Session] is
// the only way to read it, and [Client.FetchSession] is the only way to
// replace it. Every call takes one snapshot at the top and works from
// it for its whole run, so a refetch either lands entirely before a
// call or entirely after it, and no call mixes one session's API URL
// with another's account. A decoded session is never written to again,
// which is what makes a snapshot safe to hold. Reading two fields off a
// Client on two separate lines is the shape that rule exists to
// forbid.
type Client struct {
	httpClient *http.Client
	sessionURL string

	mu      sync.RWMutex
	session *Session
}

// NewClient returns a Client for the session resource at sessionURL,
// issuing every request through httpClient. A nil httpClient uses
// http.DefaultClient, which sends no credentials.
func NewClient(sessionURL string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: cmp.Or(httpClient, http.DefaultClient),
		sessionURL: sessionURL,
	}
}

// ErrNoSession reports a call made before a session was fetched. The
// API URL, the upload URL, and the download URL all come out of the
// session resource, so there is nowhere to send anything until it is in
// hand.
var ErrNoSession = errors.New("no session; call FetchSession first")

// Session returns the session in hand, or nil before the first
// [Client.FetchSession] succeeds. The value is shared rather than
// copied, and every caller must treat it as read-only.
func (c *Client) Session() *Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

// FetchSession reads the session resource and installs it, replacing
// whatever was in hand. Calls already in flight finish against the
// session they started with.
func (c *Client) FetchSession(ctx context.Context) (*Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sessionURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := refusal(resp); err != nil {
		return nil, err
	}

	session := &Session{}
	if err := json.NewDecoder(resp.Body).Decode(session); err != nil {
		return nil, fmt.Errorf("decode session: %v", err)
	}

	c.mu.Lock()
	c.session = session
	c.mu.Unlock()
	return session, nil
}

// Do posts req to the session's API URL and decodes the response.
//
// JMAP's three refusals reach a caller by three routes, and only the
// first is Do's error. A request the server would not run at all comes
// back as a [RequestError], or as an [HTTPError] when the body was not
// problem details. A method that failed is a [MethodError] in the
// response under that method's call id. A record a /set would not
// touch is a [SetError] in the method response's notCreated,
// notUpdated, or notDestroyed map, alongside a state that advanced for
// the records that did land.
//
// The response body is decoded off the stream, so a large one is never
// held twice.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	session := c.Session()
	if session == nil {
		return nil, ErrNoSession
	}

	// RFC 8620 section 3.3 applies the core capability to every
	// request. Request.Invoke already merges it, but a Request
	// assembled by hand need not have gone through Invoke. Nothing
	// here writes to req: the merged list lands on a struct copy, and
	// with no extra capabilities to merge mergeURIs either hands back
	// the caller's slice untouched or builds a new one, never
	// appending into the array behind it. go-jmap appended in place,
	// which surprises a caller that reuses a Request and races two
	// goroutines that share one.
	out := *req
	out.Using = mergeURIs(req.Using, nil)

	body, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %v", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, session.APIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := refusal(resp); err != nil {
		return nil, err
	}

	decoded := &Response{}
	if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
		return nil, fmt.Errorf("decode response: %v", err)
	}
	return decoded, nil
}

// Upload posts blob to the session's upload endpoint for account and
// returns what the server recorded (RFC 8620 section 6.1).
//
// contentType becomes the blob's media type, because the server takes
// it from this request's Content-Type header and has nothing else to
// go on. A caller that sends the wrong one mislabels the blob for
// every later download.
//
// Any 2xx carrying a decodable body succeeds. Section 6.1 mandates the
// body and says nothing about the status, and servers disagree: Cyrus
// answers 201, Fastmail and Stalwart answer 200.
func (c *Client) Upload(ctx context.Context, account ID, contentType string, blob io.Reader) (*UploadResponse, error) {
	session := c.Session()
	if session == nil {
		return nil, ErrNoSession
	}

	url := expandTemplate(session.UploadURL, "{accountId}", string(account))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, blob)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := refusal(resp); err != nil {
		return nil, err
	}

	uploaded := &UploadResponse{}
	if err := json.NewDecoder(resp.Body).Decode(uploaded); err != nil {
		return nil, fmt.Errorf("decode upload response: %v", err)
	}
	return uploaded, nil
}

// Download opens the data behind a blob id (RFC 8620 section 6.2). The
// caller closes the reader, and the bytes arrive as the server sends
// them: an attachment is arbitrarily large, and buffering one before
// the caller sees a byte trades a bounded cost for an unbounded one.
//
// contentType asks the server for the Content-Type it labels the
// response with, and name for the file name in its Content-Disposition.
// Neither changes the bytes.
func (c *Client) Download(ctx context.Context, account, blob ID, contentType, name string) (io.ReadCloser, error) {
	session := c.Session()
	if session == nil {
		return nil, ErrNoSession
	}

	url := expandTemplate(session.DownloadURL,
		"{accountId}", string(account),
		"{blobId}", string(blob),
		"{type}", contentType,
		"{name}", name,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if err := refusal(resp); err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

// An UploadResponse is what a server recorded for an uploaded blob
// (RFC 8620 section 6.1). A server is free to send more than the four
// properties the section names, and Cyrus does; the extras decode away
// without complaint.
type UploadResponse struct {
	Account ID `json:"accountId"`

	// BlobID names the stored data. It refers to the octets alone,
	// carries no metadata, and a server may hand back the id of an
	// identical blob it already held.
	BlobID ID `json:"blobId"`

	// Type is the media type the server recorded, taken from the
	// upload request's Content-Type header.
	Type string `json:"type"`

	// Size is the stored length in octets.
	Size uint64 `json:"size"`
}

// An HTTPError is a response the server did not explain with an RFC
// 7807 problem-details body. Only the status survives, because
// whatever the body held was not JMAP's to read.
type HTTPError struct {
	Status int
}

// Error implements error.
func (e *HTTPError) Error() string {
	return strings.TrimSpace(fmt.Sprintf("http %d %s", e.Status, http.StatusText(e.Status)))
}

// refusal reports what a non-success response means, and nil for a 2xx.
//
// A problem-details body (RFC 8620 section 3.6.1) comes back as a
// *RequestError carrying what the server said, whether it arrived as
// the application/problem+json RFC 7807 specifies or the
// application/json servers also send, with or without a charset
// parameter. Anything else comes back as an *HTTPError. A body that
// omits its own status takes the response's, so a caller always has
// one to classify on.
func refusal(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if isProblemDetails(resp.Header.Get("Content-Type")) {
		reqErr := &RequestError{}
		if err := json.NewDecoder(resp.Body).Decode(reqErr); err == nil {
			reqErr.Status = cmp.Or(reqErr.Status, resp.StatusCode)
			return reqErr
		}
	}
	return &HTTPError{Status: resp.StatusCode}
}

func isProblemDetails(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || mediaType == "application/problem+json"
}

// expandTemplate substitutes the variables of an RFC 6570 level 1 URI
// template, which is the form RFC 8620 sections 6.1 and 6.2 give the
// upload and download URLs in. pairs alternate a braced variable and
// its value.
func expandTemplate(template string, pairs ...string) string {
	substitutions := make([]string, len(pairs))
	for i, s := range pairs {
		if i%2 == 0 {
			substitutions[i] = s
			continue
		}
		substitutions[i] = escapeTemplateVar(s)
	}
	return strings.NewReplacer(substitutions...).Replace(template)
}

// escapeTemplateVar percent-encodes a value for RFC 6570 simple string
// expansion, which reserves every octet outside RFC 3986 section 2.3's
// unreserved set. net/url has no function for this: PathEscape leaves
// several reserved characters alone and QueryEscape spells a space as
// a plus. A media type carries a slash and a file name carries
// anything at all, so leaving either raw builds a different URL than
// the one asked for.
func escapeTemplateVar(s string) string {
	var escaped strings.Builder
	for i := range len(s) {
		c := s[i]
		if isUnreserved(c) {
			escaped.WriteByte(c)
			continue
		}
		fmt.Fprintf(&escaped, "%%%02X", c)
	}
	return escaped.String()
}

func isUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	return strings.IndexByte("-._~", c) >= 0
}
