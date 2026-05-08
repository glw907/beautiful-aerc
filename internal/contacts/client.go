package contacts

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/carddav"
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
	var inner http.Client
	if insecureTLS {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-opt-in for self-signed certs
		inner.Transport = tr
	}
	hc := webdav.HTTPClientWithBasicAuth(&inner, username, password)
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

// AddressBooks lists all collections under homeSet.
func (c *Client) AddressBooks(ctx context.Context, homeSet string) ([]carddav.AddressBook, error) {
	return c.cl.FindAddressBooks(ctx, homeSet)
}

// SyncQuery is a typed alias so callers don't import go-webdav directly.
type SyncQuery = carddav.SyncQuery

// SyncResponse is a typed alias so callers don't import go-webdav directly.
type SyncResponse = carddav.SyncResponse

// SyncCollection runs a sync-collection REPORT against bookHref.
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

	// Walk the multistatus XML looking for CS:getctag chardata.
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

// Multiget fetches the named hrefs as full vCards.
func (c *Client) Multiget(ctx context.Context, bookHref string, hrefs []string) ([]carddav.AddressObject, error) {
	return c.cl.MultiGetAddressBook(ctx, bookHref, &carddav.AddressBookMultiGet{Paths: hrefs})
}
