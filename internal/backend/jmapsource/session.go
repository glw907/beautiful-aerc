// Package jmapsource is the mail source's JMAP transport against
// Fastmail (ADR-0004): a Session authenticates once and probes the
// server's live capabilities, and Session.Mail returns the
// backend.Mail this package implements against it. No other poplar
// package speaks JMAP; internal/sync and internal/outbox drive this
// package only through the backend.Mail and backend.Credentials
// interfaces.
package jmapsource

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/glw907/poplar/internal/backend"
	"github.com/glw907/poplar/jmap"
)

// Session is one authenticated JMAP session: the credential-backed
// HTTP transport every method call in this package shares, the
// account id the session assigned mail, the capabilities the live
// session reported (ADR-0004 revision 2), and do()'s own dedup state
// for a repeated ClassAuth failure (authState's doc comment) and for
// the session refetch a moved sessionState asks for (refetchState's).
type Session struct {
	client    *jmap.Client
	accountID jmap.ID
	caps      backend.Capabilities
	auth      authState
	refetch   refetchState
}

// Dial authenticates against sessionURL, sourcing the bearer token
// from creds on every request, and probes the resulting session's
// capabilities. jmap.Client.FetchSession already threads ctx all the
// way through, so a caller's deadline or cancellation bounds session
// discovery against a hung server.
func Dial(ctx context.Context, sessionURL string, creds backend.Credentials) (*Session, error) {
	httpClient := &http.Client{Transport: &authTransport{creds: creds}}
	client := jmap.NewClient(sessionURL, httpClient)

	session, err := client.FetchSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("jmap: dial: %w", classifyDial(err))
	}

	s := &Session{
		client:    client,
		accountID: session.PrimaryAccounts[jmap.MailURI],
		caps:      probeCapabilities(session),
	}
	s.refetch.last = session.State
	return s, nil
}

// Capabilities returns the facts s's live session reported.
func (s *Session) Capabilities() backend.Capabilities { return s.caps }

// Mail returns s's mail source.
func (s *Session) Mail() backend.Mail { return &mailSource{session: s} }

func (s *Session) do(ctx context.Context, req *jmap.Request) (*jmap.Response, error) {
	resp, err := s.client.Do(ctx, req)
	if err != nil {
		return resp, classify("jmap.do", err, &s.auth)
	}
	s.auth.clear()
	s.refetch.follow(ctx, s.client, resp.SessionState)
	return resp, nil
}

// refetchState holds the sessionState a refetch has already run for,
// so the session resource is fetched once per state the server moves
// to rather than once per response reporting the move. Every response
// carries the state (RFC 8620 section 3.4), and section 2 recommends
// refetching the session when it differs from the one in hand. A busy
// account is a steady run of responses all reporting the same new
// state, so the undeduped form is a request storm against the session
// resource, on exactly the account least able to absorb one.
type refetchState struct {
	mu   sync.Mutex
	last string
}

// follow fetches the session again when state names one no refetch has
// run for, which installs the server's current API, upload, download,
// and event-source URLs in place of the ones the dial resolved.
//
// It runs on the calling goroutine, so a call that overlaps it waits
// rather than reaching the wire against a session poplar already knows
// is stale. A failed fetch still counts as run: the session in hand
// goes on working, and retrying on every response is the storm this
// dedup exists to prevent. The next state the server reports tries
// again.
func (r *refetchState) follow(ctx context.Context, client *jmap.Client, state string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state == "" || state == r.last {
		return
	}
	r.last = state
	if _, err := client.FetchSession(ctx); err != nil {
		slog.Warn("jmap: session refetch failed, continuing on the session in hand", "error", err)
	}
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
	if c, ok := session.Capabilities[jmap.CoreURI].(*jmap.Core); ok {
		caps.Limits = backend.ServerLimits{
			MaxObjectsInGet:       toInt(c.MaxObjectsInGet),
			MaxObjectsInSet:       toInt(c.MaxObjectsInSet),
			MaxCallsInRequest:     toInt(c.MaxCallsInRequest),
			MaxConcurrentRequests: toInt(c.MaxConcurrentRequests),
			MaxSizeUpload:         toInt64(c.MaxSizeUpload),
		}
	}
	if sub, ok := session.Capabilities[jmap.SubmissionURI].(*jmap.Submission); ok {
		caps.ScheduledSend = sub.MaxDelayedSend > 0
	}
	if id, ok := session.PrimaryAccounts[jmap.MailURI]; ok {
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
	for _, inv := range resp.MethodResponses {
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
