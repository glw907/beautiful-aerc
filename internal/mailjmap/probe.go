package mailjmap

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

// Probe runs a connect-test against a JMAP account and returns a
// step-by-step transcript: session-URL validation, authenticate,
// and mailbox/get. The authenticate step bundles TLS, bearer, and
// Session/get into one library call. See ADR-0190.
func Probe(ctx context.Context, cfg config.AccountConfig) mail.ProbeResult {
	r := mail.ProbeResult{}

	urlStep := mail.ProbeStep{Label: "Resolving session URL", Status: mail.ProbeOK}
	if cfg.Source == "" {
		urlStep.Status = mail.ProbeFail
		urlStep.Detail = "session URL is empty"
		r.Steps = append(r.Steps, urlStep)
		r.Err = errors.New("session URL: empty")
		return r
	}
	parsed, err := url.Parse(cfg.Source)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		urlStep.Status = mail.ProbeFail
		urlStep.Detail = "not a valid URL"
		r.Steps = append(r.Steps, urlStep)
		r.Err = fmt.Errorf("session URL: %v", cfg.Source)
		return r
	}
	urlStep.Detail = parsed.Host
	r.Steps = append(r.Steps, urlStep)

	cli, session, err := probeAuth(ctx, cfg)
	if err != nil {
		r.Steps = append(r.Steps, mail.ProbeStep{
			Label: "Authenticate", Status: mail.ProbeFail, Detail: err.Error(),
		})
		r.Err = fmt.Errorf("authenticate: %v", err)
		return r
	}
	r.Steps = append(r.Steps, mail.ProbeStep{
		Label: "Authenticate", Status: mail.ProbeOK,
	})

	count, err := probeMailboxGet(cli, session)
	if err != nil {
		r.Steps = append(r.Steps, mail.ProbeStep{
			Label: "mailbox/get", Status: mail.ProbeFail, Detail: err.Error(),
		})
		r.Err = fmt.Errorf("mailbox/get: %v", err)
		return r
	}
	r.Steps = append(r.Steps, mail.ProbeStep{
		Label:  "mailbox/get",
		Status: mail.ProbeOK,
		Detail: fmt.Sprintf("%d mailboxes", count),
	})
	return r
}

// probeAuth is the test seam for the authenticate phase.
var probeAuth probeAuthFn = realProbeAuth

type probeAuthFn func(ctx context.Context, cfg config.AccountConfig) (jmapClient, *jmap.Session, error)

func realProbeAuth(_ context.Context, cfg config.AccountConfig) (jmapClient, *jmap.Session, error) {
	pw, err := cfg.ResolvePassword()
	if err != nil {
		return nil, nil, fmt.Errorf("password: %v", err)
	}
	cli := &jmap.Client{SessionEndpoint: cfg.Source}
	cli.WithAccessToken(pw)
	if err := cli.Authenticate(); err != nil {
		return nil, nil, err
	}
	return cli, cli.Session, nil
}

// probeMailboxGet exists so tests can drive the count via
// fakeClient.respond without reimplementing the request shape.
func probeMailboxGet(cli jmapClient, session *jmap.Session) (int, error) {
	if session == nil {
		return 0, errors.New("session is nil")
	}
	accountID := session.PrimaryAccounts[jmapmail.URI]
	req := &jmap.Request{Using: []jmap.URI{jmapmail.URI}}
	req.Invoke(&mailbox.Get{Account: accountID})
	resp, err := cli.Do(req)
	if err != nil {
		return 0, err
	}
	for _, inv := range resp.Responses {
		if gr, ok := inv.Args.(*mailbox.GetResponse); ok {
			return len(gr.List), nil
		}
	}
	return 0, errors.New("no mailbox/get response")
}
