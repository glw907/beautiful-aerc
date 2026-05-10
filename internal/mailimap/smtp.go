package mailimap

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
)

// smtpClient is the subset of *gosmtp.Client mailimap uses. Tests
// substitute a fake.
type smtpClient interface {
	SendMail(from string, to []string, body []byte) error
	Close() error
}

var smtpDial = func(b *Backend) (smtpClient, error) {
	return realSMTPDial(context.Background(), b)
}

// ProbeSMTP dials, authenticates, and closes. `poplar config check`
// uses it to validate submission credentials alongside IMAP.
func ProbeSMTP(cfg config.AccountConfig) error {
	b := New(cfg)
	cli, err := smtpDial(b)
	if err != nil {
		return err
	}
	return cli.Close()
}

func realSMTPDial(ctx context.Context, b *Backend) (smtpClient, error) {
	cfg := b.cfg
	smtp := cfg.SMTP
	if smtp.Host == "" {
		return nil, errors.New("smtp: host is required")
	}
	port := smtp.Port
	if port == 0 {
		if smtp.StartTLS {
			port = 587
		} else {
			port = 465
		}
	}
	addr := net.JoinHostPort(smtp.Host, strconv.Itoa(port))
	tlsCfg := &tls.Config{ServerName: smtp.Host, InsecureSkipVerify: smtp.InsecureTLS} //nolint:gosec // InsecureTLS is opt-in for self-hosted dev servers

	raw, err := dialRawTCP(addr)
	if err != nil {
		return nil, fmt.Errorf("smtp dial %s: %w", addr, err)
	}

	var cli *gosmtp.Client
	if smtp.StartTLS {
		cli, err = gosmtp.NewClientStartTLS(raw, tlsCfg)
		if err != nil {
			_ = raw.Close()
			return nil, fmt.Errorf("smtp starttls %s: %w", addr, err)
		}
	} else {
		tlsConn := tls.Client(raw, tlsCfg)
		if err := tlsConn.Handshake(); err != nil {
			_ = raw.Close()
			return nil, fmt.Errorf("smtp tls handshake %s: %w", addr, err)
		}
		cli = gosmtp.NewClient(tlsConn)
	}

	if err := smtpAuth(ctx, cli, b); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("smtp authenticate: %w", err)
	}
	return &smtpClientAdapter{c: cli}, nil
}

type smtpClientAdapter struct{ c *gosmtp.Client }

func (a *smtpClientAdapter) SendMail(from string, to []string, body []byte) error {
	return a.c.SendMail(from, to, bytes.NewReader(body))
}

func (a *smtpClientAdapter) Close() error { return a.c.Close() }

// smtpAuth authenticates cli using the mechanism from b.cfg.SMTP. For
// xoauth2 accounts, the access token comes from b.oauth when set;
// otherwise from cfg.SMTP.ResolvePassword. Non-xoauth2 mechanisms
// always resolve via cfg.SMTP.ResolvePassword.
func smtpAuth(ctx context.Context, cli *gosmtp.Client, b *Backend) error {
	cfg := b.cfg
	smtp := cfg.SMTP
	mech := smtp.Auth
	if mech == "" {
		mech = "plain"
	}

	if mech == "xoauth2" {
		pw, err := resolveXOAUTH2SMTPToken(ctx, b)
		if err != nil {
			return err
		}
		if pw == "" {
			return errors.New("xoauth2: access token required")
		}
		return cli.Auth(mailauth.NewXoauth2Client(cfg.Email, pw))
	}

	pw, err := smtp.ResolvePassword()
	if err != nil {
		return err
	}
	switch mech {
	case "plain":
		return cli.Auth(sasl.NewPlainClient("", cfg.Email, pw))
	case "login":
		return cli.Auth(sasl.NewLoginClient(cfg.Email, pw))
	default:
		return fmt.Errorf("unsupported smtp auth mechanism %q", mech)
	}
}

// resolveXOAUTH2SMTPToken returns an access token via b.oauth when set,
// or falls back to the SMTP config's resolved password for non-OAuth accounts.
func resolveXOAUTH2SMTPToken(ctx context.Context, b *Backend) (string, error) {
	if b.oauth != nil {
		return b.oauth.Token(ctx)
	}
	return b.cfg.SMTP.ResolvePassword()
}

// Send transmits mime via SMTP. The client is dialed lazily and
// cached. Any send error drops the cached client so the next call
// redials.
func (b *Backend) Send(env mail.Envelope, mime []byte) error {
	cli, err := b.smtpClientLocked()
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	if err := cli.SendMail(env.From, env.Rcpts, mime); err != nil {
		b.dropSMTP(cli)
		return fmt.Errorf("send: %w", classifyErr(err))
	}
	return nil
}

func (b *Backend) Append(folder string, mime []byte, flags mail.Flag) error {
	if folder == "" {
		return errors.New("append: empty folder")
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()
	if _, err := cmd.Append(folder, mime, imapFlagsFor(flags)); err != nil {
		return fmt.Errorf("append: %w", classifyErr(err))
	}
	return nil
}

// PushDraft APPENDs mime as \Draft and best-effort expunges the prior
// image at prevUID. APPEND failure aborts. EXPUNGE failure orphans
// the prior image but the new draft is good (ADR-0164).
func (b *Backend) PushDraft(folder string, mime []byte, prevUID mail.UID) (mail.UID, error) {
	if folder == "" {
		return "", errors.New("push-draft: empty folder")
	}
	b.mu.Lock()
	cmd := b.cmd
	b.mu.Unlock()

	newUID, err := cmd.Append(folder, mime, []string{"\\Draft"})
	if err != nil {
		return "", fmt.Errorf("push-draft: append: %w", classifyErr(err))
	}
	if prevUID == "" {
		return newUID, nil
	}
	if _, err := cmd.Select(folder, false); err != nil {
		return newUID, nil
	}
	if err := cmd.Store([]mail.UID{prevUID}, "+FLAGS.SILENT", []string{"\\Deleted"}); err != nil {
		return newUID, nil
	}
	_ = cmd.UIDExpunge([]mail.UID{prevUID})
	return newUID, nil
}

// smtpClientLocked returns the cached SMTP client, dialing on first
// use.
func (b *Backend) smtpClientLocked() (smtpClient, error) {
	b.mu.Lock()
	if b.smtp != nil {
		c := b.smtp
		b.mu.Unlock()
		return c, nil
	}
	b.mu.Unlock()

	cli, err := smtpDial(b)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.smtp != nil {
		// Lost a race. Close the duplicate.
		old := cli
		cli = b.smtp
		b.mu.Unlock()
		_ = old.Close()
		return cli, nil
	}
	b.smtp = cli
	b.mu.Unlock()
	return cli, nil
}

// dropSMTP closes cli and clears the cache if cli is still the
// cached client.
func (b *Backend) dropSMTP(cli smtpClient) {
	b.mu.Lock()
	if b.smtp == cli {
		b.smtp = nil
	}
	b.mu.Unlock()
	_ = cli.Close()
}
