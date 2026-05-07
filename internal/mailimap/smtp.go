package mailimap

import (
	"bytes"
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

var smtpDial = realSMTPDial

// ProbeSMTP dials, authenticates, and closes. `poplar config check`
// uses it to validate submission credentials alongside IMAP.
func ProbeSMTP(cfg config.AccountConfig) error {
	cli, err := smtpDial(cfg)
	if err != nil {
		return err
	}
	return cli.Close()
}

func realSMTPDial(cfg config.AccountConfig) (smtpClient, error) {
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

	if err := smtpAuth(cli, smtp, cfg.Email); err != nil {
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

func smtpAuth(cli *gosmtp.Client, cfg config.SMTPConfig, email string) error {
	pw, err := cfg.ResolvePassword()
	if err != nil {
		return err
	}
	mech := cfg.Auth
	if mech == "" {
		mech = "plain"
	}
	switch mech {
	case "plain":
		return cli.Auth(sasl.NewPlainClient("", email, pw))
	case "login":
		return cli.Auth(sasl.NewLoginClient(email, pw))
	case "xoauth2":
		if pw == "" {
			return errors.New("xoauth2: access token required")
		}
		return cli.Auth(mailauth.NewXoauth2Client(email, pw))
	default:
		return fmt.Errorf("unsupported smtp auth mechanism %q", mech)
	}
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
	cfg := b.cfg
	b.mu.Unlock()

	cli, err := smtpDial(cfg)
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
