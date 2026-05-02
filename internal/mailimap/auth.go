// SPDX-License-Identifier: MIT

package mailimap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	imapclient "github.com/emersion/go-imap/v2/imapclient"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailauth"
	"github.com/glw907/poplar/internal/mailauth/keepalive"
)

const (
	dialTimeout       = 30 * time.Second
	keepAliveInterval = 30 // seconds, for both net.Dialer and syscall tuning
	keepAliveProbes   = 3
)

// dialCommand opens the synchronous command connection for cfg using pw
// as the resolved password.
func dialCommand(cfg config.AccountConfig, pw string) (imapClient, error) {
	return dial(cfg, pw, "command")
}

// dialIdle opens the dedicated idle connection for cfg using pw as the
// resolved password.
func dialIdle(cfg config.AccountConfig, pw string) (imapClient, error) {
	return dial(cfg, pw, "idle")
}

// dial opens one IMAP connection for the given role ("command" or "idle").
// It applies TCP keepalives, performs TLS or STARTTLS, then authenticates.
// pw is the resolved cleartext password / bearer token.
func dial(cfg config.AccountConfig, pw string, role string) (imapClient, error) {
	if cfg.Host == "" {
		return nil, errors.New("imap: host is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	d := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: time.Duration(keepAliveInterval) * time.Second,
	}
	tlsCfg := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.InsecureTLS} //nolint:gosec // InsecureTLS is opt-in for self-hosted dev servers

	// Dial the raw TCP connection so we can apply kernel keepalive tuning
	// before handing the conn to imapclient.
	raw, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s (%s): %w", addr, role, err)
	}
	if tcp, ok := raw.(*net.TCPConn); ok {
		applyKeepalive(tcp)
	}

	// Pre-allocate the realClient so its dispatch method can be wired
	// into the UnilateralDataHandler before the imapclient.Client is
	// constructed. The c field is set once the client is ready.
	rc := &realClient{}

	opts := &imapclient.Options{
		TLSConfig: tlsCfg,
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			// EXISTS increase: signal new mail arrived.
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					rc.dispatch(mail.Update{Type: mail.UpdateNewMail})
				}
			},
			// EXPUNGE: a message was removed.
			Expunge: func(_ uint32) {
				rc.dispatch(mail.Update{Type: mail.UpdateExpunge})
			},
			// Unilateral FETCH FLAGS: flags changed on one message.
			Fetch: func(msg *imapclient.FetchMessageData) {
				buf, _ := msg.Collect()
				if buf == nil {
					return
				}
				uid := imapUID(buf.UID)
				if uid == "0" {
					return
				}
				rc.dispatch(mail.Update{
					Type: mail.UpdateFlagsChanged,
					UIDs: []mail.UID{uid},
				})
			},
		},
	}

	var cli *imapclient.Client
	if cfg.StartTLS {
		// Plain TCP then STARTTLS upgrade via imapclient.NewStartTLS.
		cli, err = imapclient.NewStartTLS(raw, opts)
		if err != nil {
			return nil, fmt.Errorf("starttls %s (%s): %w", addr, role, err)
		}
	} else {
		// Implicit TLS: wrap raw connection with TLS before handing to imapclient.
		tlsConn := tls.Client(raw, tlsCfg)
		if err := tlsConn.Handshake(); err != nil {
			_ = raw.Close()
			if looksSelfHosted(cfg.Host) {
				return nil, fmt.Errorf("tls handshake %s (%s): %w (set insecure-tls = true if self-signed)", addr, role, err)
			}
			return nil, fmt.Errorf("tls handshake %s (%s): %w", addr, role, err)
		}
		cli = imapclient.New(tlsConn, opts)
	}

	if err := authenticate(cli, cfg, pw); err != nil {
		_ = cli.Logout().Wait()
		return nil, fmt.Errorf("authenticate (%s): %w", role, err)
	}

	rc.c = cli
	return rc, nil
}

// applyKeepalive tunes kernel TCP keepalive probes and interval on c.
// Failures are silently ignored — the OS-level KeepAlive on the Dialer
// already provides basic keepalive; the syscall tuning is advisory.
func applyKeepalive(c *net.TCPConn) {
	_ = c.SetKeepAlive(true)
	f, err := c.File()
	if err != nil {
		return
	}
	defer f.Close()
	fd := int(f.Fd())
	_ = keepalive.SetTcpKeepaliveProbes(fd, keepAliveProbes)
	_ = keepalive.SetTcpKeepaliveInterval(fd, keepAliveInterval)
}

// resolvePassword returns the cleartext password for cfg. Inline
// Password wins; otherwise PasswordCmd is run via /bin/sh -c and
// stdout (trimmed) is the password. Returns an error if neither
// is set or the command fails.
func resolvePassword(cfg *config.AccountConfig) (string, error) {
	if cfg.Password != "" {
		return cfg.Password, nil
	}
	if cfg.PasswordCmd == "" {
		return "", errors.New("account has no password or password-cmd")
	}
	cmd := exec.Command("/bin/sh", "-c", cfg.PasswordCmd)
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = strings.TrimSpace(string(ee.Stderr))
		}
		if stderr != "" {
			return "", fmt.Errorf("password-cmd failed: %s", stderr)
		}
		return "", fmt.Errorf("password-cmd failed: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// resolvedPassword returns the cached password for b, resolving it on
// the first call. The cached value is stored under b.mu so reconnects
// within the session reuse the same credential without re-running the cmd.
func (b *Backend) resolvedPassword() (string, error) {
	b.mu.Lock()
	cached := b.password
	b.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	pw, err := resolvePassword(&b.cfg)
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	b.password = pw
	b.mu.Unlock()
	return pw, nil
}

// authenticate runs the SASL exchange specified by cfg.Auth.
// pw is the resolved cleartext password / bearer token.
// Supported mechanisms: plain (default), login, cram-md5, xoauth2.
func authenticate(cli *imapclient.Client, cfg config.AccountConfig, pw string) error {
	mech := cfg.Auth
	if mech == "" {
		mech = "plain"
	}
	switch mech {
	case "plain":
		return cli.Authenticate(sasl.NewPlainClient("", cfg.Email, pw))
	case "login":
		return cli.Login(cfg.Email, pw).Wait()
	case "cram-md5":
		// go-sasl v0.0.0-20241020182733 does not ship CRAM-MD5; reject early.
		return errors.New("cram-md5: not supported by the bundled go-sasl version")
	case "xoauth2":
		if pw == "" {
			return errors.New("xoauth2: access token (password field) required")
		}
		return cli.Authenticate(mailauth.NewXoauth2Client(cfg.Email, pw))
	default:
		return fmt.Errorf("unsupported auth mechanism %q", mech)
	}
}
