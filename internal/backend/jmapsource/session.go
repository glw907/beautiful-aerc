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
	push      pushSource
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
	s.push.session = s
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

// refetchState remembers which sessionState values a refetch has
// already answered, so the session resource is fetched once per state
// the server moves to rather than once per response reporting the
// move. Every response carries the state (RFC 8620 section 3.4), and
// section 2 recommends refetching the session when it differs from the
// one in hand. A busy account is a steady run of responses all
// reporting the same new state, so the undeduped form is a request
// storm against the session resource, on exactly the account least
// able to absorb one.
//
// It remembers a pair because concurrent calls straddle a move: a
// response already in flight when the state changed reports the state
// it started under, arriving after the newer ones have reported the
// new one. Keyed on the newest state alone, every flip between the two
// reads as fresh news and refetches. The state a refetch superseded is
// not news either.
type refetchState struct {
	mu         sync.Mutex
	last       string
	superseded string
}

// follow fetches the session again when state is one no refetch has
// answered, which installs the server's current API, upload, download,
// and event-source URLs in place of the ones the dial resolved.
//
// The fetch runs on the calling goroutine but outside the lock: do
// follows the state on every successful response, so holding it across
// the wire would park every other call on this Session behind one slow
// or black-holing session resource. Those calls are not left
// unprotected by that, since each reads the session it runs against
// out of the client once, and FetchSession replaces that value rather
// than editing it.
//
// A failed fetch still counts as answered: the session in hand goes on
// working, and retrying on every response is the storm this dedup
// exists to prevent. The next state the server reports tries again.
func (r *refetchState) follow(ctx context.Context, client *jmap.Client, state string) {
	if !r.claim(state) {
		return
	}
	// The session installs itself in the client, which is where every
	// call reads its URLs from, so the returned value has no second
	// consumer here. s.accountID and s.caps keep their dial-time values
	// on purpose: a primary account id or a capability set that moved
	// under one credential is a re-dial, not a URL change, and pass 1b
	// has no re-dial path.
	if _, err := client.FetchSession(ctx); err != nil {
		slog.Warn("jmap: session refetch failed, continuing on the session in hand", "error", err)
	}
}

// claim reports whether state is a move no refetch has answered,
// recording it as answered when it is.
func (r *refetchState) claim(state string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if state == "" || state == r.last || state == r.superseded {
		return false
	}
	r.superseded, r.last = r.last, state
	return true
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
