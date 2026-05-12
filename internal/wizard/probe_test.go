package wizard

import (
	"context"
	"errors"
	"testing"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
)

func withFakeProbes(t *testing.T,
	imap func(context.Context, config.AccountConfig, *mailauth.Client) mail.ProbeResult,
	jmap func(context.Context, config.AccountConfig) mail.ProbeResult,
	smtp func(config.AccountConfig, *mailauth.Client) error,
) {
	t.Helper()
	prevI, prevJ, prevS := imapProbeFn, jmapProbeFn, smtpProbeFn
	imapProbeFn, jmapProbeFn, smtpProbeFn = imap, jmap, smtp
	t.Cleanup(func() { imapProbeFn, jmapProbeFn, smtpProbeFn = prevI, prevJ, prevS })
}

func TestProbeRoutesIMAPAndAppendsSMTP(t *testing.T) {
	withFakeProbes(t,
		func(_ context.Context, _ config.AccountConfig, _ *mailauth.Client) mail.ProbeResult {
			return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "imap", Status: mail.ProbeOK}}}
		},
		func(context.Context, config.AccountConfig) mail.ProbeResult { return mail.ProbeResult{} },
		func(config.AccountConfig, *mailauth.Client) error { return nil },
	)

	r := Probe(context.Background(), config.AccountConfig{Backend: "imap"}, nil)
	if len(r.Steps) != 2 || r.Steps[1].Label != "SMTP submission" {
		t.Fatalf("expected imap + SMTP submission, got %+v", r.Steps)
	}
	if !r.OK() {
		t.Fatalf("OK = false, want true")
	}
}

func TestProbeRoutesJMAP(t *testing.T) {
	withFakeProbes(t,
		func(_ context.Context, _ config.AccountConfig, _ *mailauth.Client) mail.ProbeResult {
			return mail.ProbeResult{}
		},
		func(context.Context, config.AccountConfig) mail.ProbeResult {
			return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "jmap", Status: mail.ProbeOK}}}
		},
		func(config.AccountConfig, *mailauth.Client) error { return nil },
	)

	r := Probe(context.Background(), config.AccountConfig{Backend: "jmap"}, nil)
	if len(r.Steps) != 1 || r.Steps[0].Label != "jmap" {
		t.Fatalf("expected jmap-only, got %+v", r.Steps)
	}
}

func TestProbeIMAPSurfacesSMTPFailure(t *testing.T) {
	withFakeProbes(t,
		func(_ context.Context, _ config.AccountConfig, _ *mailauth.Client) mail.ProbeResult {
			return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "imap", Status: mail.ProbeOK}}}
		},
		func(context.Context, config.AccountConfig) mail.ProbeResult { return mail.ProbeResult{} },
		func(config.AccountConfig, *mailauth.Client) error { return errors.New("connection refused") },
	)

	r := Probe(context.Background(), config.AccountConfig{Backend: "imap"}, nil)
	if r.OK() {
		t.Fatal("expected SMTP failure to flip OK to false")
	}
	if r.Steps[1].Status != mail.ProbeFail {
		t.Fatalf("SMTP step status = %v, want ProbeFail", r.Steps[1].Status)
	}
}

func TestProbeThreadsOAuthClientIntoIMAPAndSMTP(t *testing.T) {
	var gotIMAP, gotSMTP bool
	withFakeProbes(t,
		func(_ context.Context, _ config.AccountConfig, c *mailauth.Client) mail.ProbeResult {
			gotIMAP = c != nil
			return mail.ProbeResult{Steps: []mail.ProbeStep{{Label: "imap", Status: mail.ProbeOK}}}
		},
		func(context.Context, config.AccountConfig) mail.ProbeResult { return mail.ProbeResult{} },
		func(_ config.AccountConfig, c *mailauth.Client) error { gotSMTP = c != nil; return nil },
	)

	sentinel := &mailauth.Client{}
	Probe(context.Background(), config.AccountConfig{Backend: "imap"}, sentinel)
	if !gotIMAP || !gotSMTP {
		t.Fatalf("oauth client not threaded: imap=%v smtp=%v", gotIMAP, gotSMTP)
	}
}
