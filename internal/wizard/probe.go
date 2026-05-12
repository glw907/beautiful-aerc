package wizard

import (
	"context"
	"fmt"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/mailimap"
	"github.com/glw907/poplar/internal/mailjmap"
)

// Indirection so tests can substitute fakes without dialing real servers.
var (
	imapProbeFn = func(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
		return mailimap.Probe(ctx, cfg, oauthCli)
	}
	jmapProbeFn = mailjmap.Probe
	smtpProbeFn = func(cfg config.AccountConfig, oauthCli *mailauth.Client) error {
		return mailimap.ProbeSMTP(cfg, oauthCli)
	}
)

// Probe routes to the right backend based on cfg.Backend and returns a
// step-by-step transcript. oauthCli is non-nil for accounts using xoauth2;
// nil otherwise. SMTP is appended for IMAP accounts only; JMAP submission
// rides the JMAP session itself.
func Probe(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
	switch cfg.Backend {
	case "imap":
		r := imapProbeFn(ctx, cfg, oauthCli)
		if !r.OK() {
			return r
		}
		smtpErr := smtpProbeFn(cfg, oauthCli)
		step := mail.ProbeStep{Label: "SMTP submission", Status: mail.ProbeOK}
		if smtpErr != nil {
			step.Status = mail.ProbeFail
			step.Detail = smtpErr.Error()
			r.Err = fmt.Errorf("smtp: %w", smtpErr)
		}
		r.Steps = append(r.Steps, step)
		return r
	case "jmap":
		return jmapProbeFn(ctx, cfg)
	}
	return mail.ProbeResult{Err: fmt.Errorf("unknown backend %q", cfg.Backend)}
}
