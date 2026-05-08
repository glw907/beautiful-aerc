package contacts

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
)

// Sentinels routed on by the drainer's conflict matrix.
var (
	ErrAuth               = errors.New("contacts: auth")
	ErrNotFound           = errors.New("contacts: not found")
	ErrPreconditionFailed = errors.New("contacts: precondition failed")
)

// Client is poplar's CardDAV face. It wraps go-webdav so the rest
// of the package depends on a small, stable surface.
type Client struct {
	cl         *carddav.Client
	httpClient webdav.HTTPClient
	base       string
}

// NewClient builds a CardDAV client for the given server URL with HTTP Basic
// auth. insecureTLS skips certificate verification for self-hosted servers.
func NewClient(serverURL, username, password string, insecureTLS bool) (*Client, error) {
	base := http.DefaultClient
	if insecureTLS {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-opt-in for self-signed certs
		base = &http.Client{Transport: tr}
	}
	hc := webdav.HTTPClientWithBasicAuth(base, username, password)
	cl, err := carddav.NewClient(hc, serverURL)
	if err != nil {
		return nil, fmt.Errorf("carddav client: %w", err)
	}
	return &Client{cl: cl, httpClient: hc, base: serverURL}, nil
}

// HomeSet resolves the principal's addressbook-home-set. Falls back to the
// configured server URL when discovery returns nothing; some self-hosted
// servers expect a direct collection URL.
func (c *Client) HomeSet(ctx context.Context) (string, error) {
	principal, err := c.cl.FindCurrentUserPrincipal(ctx)
	if err != nil {
		return c.base, nil
	}
	home, err := c.cl.FindAddressBookHomeSet(ctx, principal)
	if err != nil || home == "" {
		return c.base, nil
	}
	return home, nil
}

func (c *Client) AddressBooks(ctx context.Context, homeSet string) ([]carddav.AddressBook, error) {
	return c.cl.FindAddressBooks(ctx, homeSet)
}

type (
	SyncQuery    = carddav.SyncQuery
	SyncResponse = carddav.SyncResponse
)

func (c *Client) SyncCollection(ctx context.Context, bookHref string, q *SyncQuery) (*SyncResponse, error) {
	return c.cl.SyncCollection(ctx, bookHref, q)
}

// CTAG fetches the cs:getctag value for bookHref via a depth-0
// PROPFIND. Returns "" when the server does not advertise the property.
func (c *Client) CTAG(ctx context.Context, bookHref string) (string, error) {
	const ctagNS = "http://calendarserver.org/ns/"
	body := `<?xml version="1.0" encoding="utf-8"?>` +
		`<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">` +
		`<D:prop><CS:getctag/></D:prop>` +
		`</D:propfind>`

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", bookHref, bytes.NewBufferString(body))
	if err != nil {
		return "", fmt.Errorf("ctag propfind %s: %w", bookHref, err)
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ctag propfind %s: %w", bookHref, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus {
		return "", nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ctag read response: %w", err)
	}

	dec := xml.NewDecoder(bytes.NewReader(raw))
	var inCTAG bool
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			inCTAG = t.Name.Space == ctagNS && t.Name.Local == "getctag"
		case xml.CharData:
			if inCTAG {
				return string(t), nil
			}
		case xml.EndElement:
			inCTAG = false
		}
	}
	return "", nil
}

func (c *Client) Multiget(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error) {
	return c.cl.MultiGetAddressBook(ctx, bookHref, &carddav.AddressBookMultiGet{Paths: hrefs})
}

// PutAddressObject writes body to href with optional If-Match. Returns
// the server's chosen href (may differ from input) and the new ETag.
// Maps 401/403 → ErrAuth, 404 → ErrNotFound, 412 → ErrPreconditionFailed.
func (c *Client) PutAddressObject(
	ctx context.Context, href, ifMatch string, body []byte,
) (newHref, newETag string, err error) {
	req, err := c.newReq(ctx, "PUT", href, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "text/vcard; charset=utf-8")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("put %s: %w", href, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	if e := mapStatus(resp.StatusCode); e != nil {
		return "", "", fmt.Errorf("put %s: %w", href, e)
	}
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("put %s: status %d", href, resp.StatusCode)
	}
	newHref = href
	if loc := resp.Header.Get("Location"); loc != "" {
		newHref = loc
	}
	return newHref, resp.Header.Get("ETag"), nil
}

// DeleteAddressObject removes href with optional If-Match. Maps the
// same status codes as PutAddressObject. Caller treats ErrNotFound as
// idempotent success.
func (c *Client) DeleteAddressObject(ctx context.Context, href, ifMatch string) error {
	req, err := c.newReq(ctx, "DELETE", href, nil)
	if err != nil {
		return err
	}
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete %s: %w", href, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	if e := mapStatus(resp.StatusCode); e != nil {
		return fmt.Errorf("delete %s: %w", href, e)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("delete %s: status %d", href, resp.StatusCode)
	}
	return nil
}

// newReq resolves href against the configured base URL and builds a
// context-bound request.
func (c *Client) newReq(ctx context.Context, method, href string, body io.Reader) (*http.Request, error) {
	target := href
	if u, err := url.Parse(href); err == nil && !u.IsAbs() {
		base, _ := url.Parse(c.base)
		target = base.ResolveReference(u).String()
	}
	return http.NewRequestWithContext(ctx, method, target, body)
}

func mapStatus(code int) error {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuth
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusPreconditionFailed:
		return ErrPreconditionFailed
	}
	return nil
}
