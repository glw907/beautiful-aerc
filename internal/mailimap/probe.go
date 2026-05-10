package mailimap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"

	imapclient "github.com/emersion/go-imap/v2/imapclient"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
)

// Probe runs a connect-test against an IMAP account and returns a
// step-by-step transcript. When oauthCli is non-nil an oauth-token step
// runs first; failure there stops the transcript. On any failure remaining
// steps are dropped and Err is set. SMTP probing is separate (ProbeSMTP);
// callers run both.
func Probe(ctx context.Context, cfg config.AccountConfig, oauthCli *mailauth.Client) mail.ProbeResult {
	var r mail.ProbeResult
	if oauthCli != nil {
		step := mail.ProbeStep{Label: "oauth-token"}
		if _, err := oauthCli.Token(ctx); err != nil {
			step.Status = mail.ProbeFail
			step.Detail = err.Error()
			r.Steps = append(r.Steps, step)
			r.Err = err
			return r
		}
		step.Status = mail.ProbeOK
		r.Steps = append(r.Steps, step)
	}

	cli, steps, err := probeDial(cfg)
	r.Steps = append(r.Steps, steps...)
	if err != nil {
		r.Err = err
		return r
	}
	defer func() { _ = cli.Logout() }()

	// CAPABILITY (UIDPLUS asserted)
	caps, err := cli.Capabilities()
	switch {
	case err != nil:
		r.Steps = append(r.Steps, mail.ProbeStep{
			Label: "CAPABILITY (UIDPLUS)", Status: mail.ProbeFail, Detail: err.Error(),
		})
		r.Err = fmt.Errorf("capability: %v", err)
		return r
	case !caps["UIDPLUS"]:
		r.Steps = append(r.Steps, mail.ProbeStep{
			Label: "CAPABILITY (UIDPLUS)", Status: mail.ProbeFail, Detail: "server lacks UIDPLUS",
		})
		r.Err = errors.New("capability: server lacks UIDPLUS")
		return r
	}
	r.Steps = append(r.Steps, mail.ProbeStep{
		Label: "CAPABILITY (UIDPLUS)", Status: mail.ProbeOK,
	})

	// STATUS INBOX. Read-only SELECT returns the same EXISTS count as
	// STATUS and is already on the imapClient interface.
	folder, err := cli.Select("INBOX", true)
	if err != nil {
		r.Steps = append(r.Steps, mail.ProbeStep{
			Label: "STATUS INBOX", Status: mail.ProbeFail, Detail: err.Error(),
		})
		r.Err = fmt.Errorf("status inbox: %v", err)
		return r
	}
	r.Steps = append(r.Steps, mail.ProbeStep{
		Label:  "STATUS INBOX",
		Status: mail.ProbeOK,
		Detail: fmt.Sprintf("%d messages", folder.Exists),
	})
	return r
}

// probeDial is the test seam for the dial+TLS+auth phase.
var probeDial probeDialFn = realProbeDial

type probeDialFn func(cfg config.AccountConfig) (imapClient, []mail.ProbeStep, error)

func realProbeDial(cfg config.AccountConfig) (imapClient, []mail.ProbeStep, error) {
	if cfg.Host == "" {
		return nil, nil, errors.New("imap: host is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	steps := make([]mail.ProbeStep, 0, 3)

	raw, err := dialRawTCP(addr)
	if err != nil {
		steps = append(steps, mail.ProbeStep{
			Label: "Connecting", Status: mail.ProbeFail, Detail: err.Error(),
		})
		return nil, steps, fmt.Errorf("dial %s: %v", addr, err)
	}
	steps = append(steps, mail.ProbeStep{
		Label: "Connecting", Status: mail.ProbeOK, Detail: addr,
	})

	tlsCfg := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.InsecureTLS} //nolint:gosec // InsecureTLS is opt-in for self-hosted dev servers
	cli, err := layerTLS(raw, cfg, tlsCfg, &imapclient.Options{TLSConfig: tlsCfg})
	if err != nil {
		_ = raw.Close()
		steps = append(steps, mail.ProbeStep{
			Label: "TLS handshake", Status: mail.ProbeFail, Detail: err.Error(),
		})
		return nil, steps, fmt.Errorf("tls: %v", err)
	}
	steps = append(steps, mail.ProbeStep{
		Label: "TLS handshake", Status: mail.ProbeOK,
	})

	pw, err := cfg.ResolvePassword()
	if err != nil {
		_ = cli.Logout().Wait()
		steps = append(steps, mail.ProbeStep{
			Label: "AUTHENTICATE", Status: mail.ProbeFail, Detail: err.Error(),
		})
		return nil, steps, fmt.Errorf("password: %v", err)
	}

	if err := authenticate(cli, cfg, pw); err != nil {
		_ = cli.Logout().Wait()
		steps = append(steps, mail.ProbeStep{
			Label: "AUTHENTICATE", Status: mail.ProbeFail, Detail: err.Error(),
		})
		return nil, steps, fmt.Errorf("authenticate: %v", err)
	}
	mech := cfg.Auth
	if mech == "" {
		mech = "plain"
	}
	steps = append(steps, mail.ProbeStep{
		Label: "AUTHENTICATE", Status: mail.ProbeOK, Detail: mech,
	})

	return &realClient{c: cli}, steps, nil
}
