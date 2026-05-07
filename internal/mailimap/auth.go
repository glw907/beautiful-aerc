package mailimap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	imapclient "github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"

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

// dial opens one IMAP connection for role ("command" or "idle"),
// applies keepalives, performs TLS or STARTTLS, then authenticates
// with pw (resolved cleartext password or bearer token).
func dial(cfg config.AccountConfig, pw string, role string) (imapClient, error) {
	if cfg.Host == "" {
		return nil, errors.New("imap: host is required")
	}
	port := cfg.Port
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	tlsCfg := &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: cfg.InsecureTLS} //nolint:gosec // InsecureTLS is opt-in for self-hosted dev servers

	raw, err := dialRawTCP(addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s (%s): %w", addr, role, err)
	}

	// Pre-allocate the realClient so its dispatch method can be wired
	// into the UnilateralDataHandler before the imapclient.Client is
	// constructed. The c field is set once the client is ready.
	// Pre-allocate so dispatch can wire into UnilateralDataHandler
	// before c is set.
	rc := &realClient{}

	opts := &imapclient.Options{
		TLSConfig: tlsCfg,
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					rc.dispatch(mail.Update{Type: mail.UpdateNewMail})
				}
			},
			Expunge: func(_ uint32) {
				rc.dispatch(mail.Update{Type: mail.UpdateExpunge})
			},
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
		cli, err = imapclient.NewStartTLS(raw, opts)
		if err != nil {
			return nil, fmt.Errorf("starttls %s (%s): %w", addr, role, err)
		}
	} else {
		tlsConn := tls.Client(raw, tlsCfg)
		if err := tlsConn.Handshake(); err != nil {
			_ = raw.Close()
			if !cfg.InsecureTLS && looksSelfHosted(cfg.Host) {
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

// dialRawTCP opens a TCP connection with dial timeout and keepalive
// tuning applied. Callers layer TLS or STARTTLS on top.
func dialRawTCP(addr string) (net.Conn, error) {
	d := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: time.Duration(keepAliveInterval) * time.Second,
	}
	raw, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tcp, ok := raw.(*net.TCPConn); ok {
		applyKeepalive(tcp)
	}
	return raw, nil
}

// applyKeepalive tunes kernel TCP keepalive probes and interval. The
// Dialer's KeepAlive already provides basic keepalive. Syscall tuning
// is advisory and failures are silently ignored.
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

// resolvedPassword returns the cached password, resolving on first
// call so reconnects reuse the credential without re-running
// password-cmd. XOAUTH2 access tokens bypass the cache: every dial
// re-runs password-cmd because tokens are short-lived (~1h on Gmail).
func (b *Backend) resolvedPassword() (string, error) {
	if b.cfg.Auth == "xoauth2" {
		return b.cfg.ResolvePassword()
	}
	b.mu.Lock()
	cached := b.password
	b.mu.Unlock()
	if cached != "" {
		return cached, nil
	}
	pw, err := b.cfg.ResolvePassword()
	if err != nil {
		return "", err
	}
	b.mu.Lock()
	b.password = pw
	b.mu.Unlock()
	return pw, nil
}

// authenticate runs the SASL exchange named by cfg.Auth. Supported
// mechanisms: plain (default), login, cram-md5, xoauth2.
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
		// go-sasl v0.0.0-20241020182733 does not ship CRAM-MD5. Reject early.
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

// looksSelfHosted gates the "set insecure-tls = true" TLS-error hint
// to RFC 1918, IPv6 ULA, .local, and loopback.
func looksSelfHosted(host string) bool {
	if strings.HasSuffix(host, ".local") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}
