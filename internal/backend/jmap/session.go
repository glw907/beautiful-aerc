// Package jmap is the mail source's JMAP transport against Fastmail
// (ADR-0004): a Session authenticates once and probes the server's
// live capabilities, and Session.Mail returns the backend.Mail this
// package implements against it. No other poplar package speaks
// JMAP; internal/sync and internal/outbox drive this package only
// through the backend.Mail and backend.Credentials interfaces.
package jmap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/core"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/emailsubmission"

	"github.com/glw907/poplar/internal/backend"
)

// Session is one authenticated JMAP session: the credential-backed
// HTTP transport every method call in this package shares, the
// account id the session assigned mail, and the capabilities the
// live session reported (ADR-0004 revision 2).
type Session struct {
	client    *jmap.Client
	accountID jmap.ID
	caps      backend.Capabilities
}

// Dial authenticates against sessionURL, sourcing the bearer token
// from creds on every request, and probes the resulting session's
// capabilities. Dial fetches the session resource itself rather than
// calling go-jmap's Client.Authenticate, which builds its request
// with http.NewRequest and no context: doing so here threads ctx all
// the way through, so a caller's deadline or cancellation actually
// bounds session discovery against a hung server.
func Dial(ctx context.Context, sessionURL string, creds backend.Credentials) (*Session, error) {
	httpClient := &http.Client{Transport: &authTransport{creds: creds}}
	client := &jmap.Client{SessionEndpoint: sessionURL, HttpClient: httpClient}

	session, err := fetchSession(ctx, httpClient, sessionURL)
	if err != nil {
		return nil, fmt.Errorf("jmap: dial: %w", err)
	}
	client.Session = session

	return &Session{
		client:    client,
		accountID: session.PrimaryAccounts[jmapmail.URI],
		caps:      probeCapabilities(session),
	}, nil
}

// fetchSession GETs sessionURL with httpClient, decoding the JMAP
// session resource ctx-bound.
func fetchSession(ctx context.Context, httpClient *http.Client, sessionURL string) (*jmap.Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, classify("jmap.dial", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("session %s: unexpected status %d", sessionURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	session := &jmap.Session{}
	if err := json.Unmarshal(body, session); err != nil {
		return nil, err
	}
	return session, nil
}

// Capabilities returns the facts s's live session reported.
func (s *Session) Capabilities() backend.Capabilities { return s.caps }

// Mail returns s's mail source.
func (s *Session) Mail() backend.Mail { return &mailSource{session: s} }

func (s *Session) do(req *jmap.Request) (*jmap.Response, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return resp, classify("jmap.do", err)
	}
	return resp, nil
}

// authTransport resolves the bearer token from creds per request
// rather than once at construction, so Credentials.Token's refresh
// (ADR-0004 revision 2) reaches the wire without a 401-retry dance
// at the call site.
type authTransport struct {
	creds backend.Credentials
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.creds.Token(req.Context())
	if err != nil {
		return nil, fmt.Errorf("jmap: credential: %w", err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultTransport.RoundTrip(req)
}

// probeCapabilities reads backend.Capabilities off session rather
// than assuming protocol defaults (ADR-0004 revision 2): the account
// ids per capability, Core's numeric limits, EmailSubmission's
// delayed-send ceiling, and whether the session offers EventSource
// push.
func probeCapabilities(session *jmap.Session) backend.Capabilities {
	caps := backend.Capabilities{
		ThreadIdentity:   backend.ThreadIdentityReferencesDerived,
		DeltaGranularity: backend.DeltaGranularityAccount,
		ServerSearch:     true,
		AccountIDs:       map[string]string{},
	}
	if session.EventSourceURL != "" {
		caps.PushTransport = backend.PushTransportEventSource
	}
	if raw, ok := session.Capabilities[jmap.CoreURI]; ok {
		if c, ok := raw.(*core.Core); ok {
			caps.Limits = backend.ServerLimits{
				MaxObjectsInGet:       toInt(c.MaxObjectsInGet),
				MaxObjectsInSet:       toInt(c.MaxObjectsInSet),
				MaxCallsInRequest:     toInt(c.MaxCallsInRequest),
				MaxConcurrentRequests: toInt(c.MaxConcurrentRequests),
				MaxSizeUpload:         toInt64(c.MaxSizeUpload),
			}
		}
	}
	if raw, ok := session.Capabilities[emailsubmission.URI]; ok {
		if es, ok := raw.(*emailsubmission.Capability); ok {
			caps.ScheduledSend = es.MaxDelayedSend > 0
		}
	}
	if id, ok := session.PrimaryAccounts[jmapmail.URI]; ok {
		caps.AccountIDs["mail"] = string(id)
	}
	return caps
}

// findResponse locates the invocation in resp matching callID and
// asserts its Args to T, the JMAP method's response type. A method
// error in that slot (JMAP routes a per-call failure as a MethodError
// invocation rather than a request-level error) comes back as err, so
// a caller can errors.As it against a MethodError to classify it.
func findResponse[T any](resp *jmap.Response, callID string) (T, error) {
	var zero T
	for _, inv := range resp.Responses {
		if inv.CallID != callID {
			continue
		}
		if me, ok := inv.Args.(*jmap.MethodError); ok {
			return zero, me
		}
		v, ok := inv.Args.(T)
		if !ok {
			return zero, fmt.Errorf("jmap: unexpected response type for call %s", callID)
		}
		return v, nil
	}
	return zero, fmt.Errorf("jmap: no response for call %s", callID)
}
