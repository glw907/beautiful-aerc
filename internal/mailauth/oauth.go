package mailauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/mail"
	"golang.org/x/oauth2"
)

// Config holds the OAuth2 client parameters for a single provider.
type Config struct {
	ClientID      string
	ClientSecret  string
	AuthURL       string
	TokenURL      string
	DeviceAuthURL string // RFC 8628 endpoint; required for AuthorizeDeviceCode.
	Scopes        []string
	// RedirectPortRange bounds the loopback port the consent server listens on.
	RedirectPortRange [2]int
}

var (
	ErrConsentTimeout = errors.New("mailauth: consent timeout")
	ErrStateMismatch  = errors.New("mailauth: oauth state mismatch")
	// ErrDeviceCodeUnsupported fires when AuthorizeDeviceCode is called
	// against a preset that lacks DeviceAuthURL.
	ErrDeviceCodeUnsupported = errors.New("mailauth: device-code not supported by provider")
	// ErrDeviceConsentDenied fires when the user denies the device-code
	// prompt or the user_code times out.
	ErrDeviceConsentDenied = errors.New("mailauth: device-code consent denied")
)

// Client obtains and refreshes access tokens via a stored refresh token.
// It is safe for concurrent use.
type Client struct {
	cfg     Config
	store   TokenStore
	slug    string
	backend Backend

	openBrowser    func(string) error
	consentTimeout time.Duration

	mu        sync.Mutex
	cachedTok string
	expires   time.Time
}

// NewClient returns a Client backed by store, keyed on slug.
// backend records which token-store implementation is in use so
// callers can persist the choice in config.
func NewClient(cfg Config, store TokenStore, slug string, backend Backend) *Client {
	return &Client{
		cfg:            cfg,
		store:          store,
		slug:           slug,
		backend:        backend,
		openBrowser:    platformOpen,
		consentTimeout: 2 * time.Minute,
	}
}

func (c *Client) Backend() Backend { return c.backend }

// SetOpenBrowser replaces the browser-launch function for testing.
func SetOpenBrowser(c *Client, fn func(string) error) { c.openBrowser = fn }

// SetConsentTimeout overrides the default consent wait for testing.
func SetConsentTimeout(c *Client, d time.Duration) { c.consentTimeout = d }

func platformOpen(authURL string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		return exec.Command("cmd", "/c", "start", authURL).Start()
	default:
		cmd = "xdg-open"
	}
	return exec.Command(cmd, authURL).Start()
}

// Authorize runs the full PKCE authorization code flow: starts a loopback
// consent server, opens the browser, waits for the callback, exchanges the
// code, and stores the resulting refresh token.
func (c *Client) Authorize(ctx context.Context) error {
	verifier, err := generatePKCEVerifier()
	if err != nil {
		return fmt.Errorf("oauth verifier: %w", err)
	}
	state, err := generateState()
	if err != nil {
		return fmt.Errorf("oauth state: %w", err)
	}

	srv, redirect, done, err := runConsentServer(c.cfg.RedirectPortRange)
	if err != nil {
		return fmt.Errorf("oauth consent server: %w", err)
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	authURL := c.buildAuthURL(state, codeChallengeS256(verifier), redirect)
	if err := c.openBrowser(authURL); err != nil {
		return fmt.Errorf("oauth open browser: %w", err)
	}

	select {
	case res := <-done:
		if res.err != nil {
			return res.err
		}
		if res.state != state {
			return ErrStateMismatch
		}
		tok, err := c.exchangeCode(ctx, res.code, verifier, redirect)
		if err != nil {
			return err
		}
		return c.store.Set(c.slug, tok.RefreshToken)
	case <-time.After(c.consentTimeout):
		return ErrConsentTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) buildAuthURL(state, challenge, redirect string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.cfg.ClientID},
		"redirect_uri":          {redirect},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"access_type":           {"offline"},
		"prompt":                {"consent"},
	}
	if len(c.cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(c.cfg.Scopes, " "))
	}
	return c.cfg.AuthURL + "?" + params.Encode()
}

func (c *Client) exchangeCode(ctx context.Context, code, verifier, redirect string) (*oauth2.Token, error) {
	oc := &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.cfg.AuthURL,
			TokenURL: c.cfg.TokenURL,
		},
		RedirectURL: redirect,
		Scopes:      c.cfg.Scopes,
	}
	tok, err := oc.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return nil, classifyOAuthErr(err)
	}
	return tok, nil
}

func generatePKCEVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func codeChallengeS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ForceRefresh drops the cached access token and fetches a new one.
// Used by callers that just saw an upstream auth failure to distinguish
// a stale-cache miss from a truly bad refresh token.
func (c *Client) ForceRefresh(ctx context.Context) (string, error) {
	c.mu.Lock()
	c.cachedTok = ""
	c.expires = time.Time{}
	c.mu.Unlock()
	return c.Token(ctx)
}

// Token returns a valid access token, refreshing from the stored refresh token
// when the cached token is absent or within 5 minutes of expiry.
func (c *Client) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedTok != "" && time.Until(c.expires) > 5*time.Minute {
		return c.cachedTok, nil
	}

	refresh, err := c.store.Get(c.slug)
	if err != nil {
		return "", fmt.Errorf("oauth refresh: %w", err)
	}
	if refresh == "" {
		return "", fmt.Errorf("oauth refresh missing: %w", mail.ErrAuth)
	}

	oc := &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  c.cfg.AuthURL,
			TokenURL: c.cfg.TokenURL,
		},
		Scopes: c.cfg.Scopes,
	}
	src := oc.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh})
	tok, err := src.Token()
	if err != nil {
		return "", classifyOAuthErr(err)
	}

	c.cachedTok = tok.AccessToken
	c.expires = tok.Expiry

	if tok.RefreshToken != "" && tok.RefreshToken != refresh {
		_ = c.store.Set(c.slug, tok.RefreshToken)
	}

	return c.cachedTok, nil
}

func classifyOAuthErr(err error) error {
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) {
		return err
	}
	sc := re.Response.StatusCode
	if (sc == 400 || sc == 401) &&
		(bytes.Contains(re.Body, []byte("invalid_grant")) ||
			bytes.Contains(re.Body, []byte("invalid_token"))) {
		return fmt.Errorf("%s: %w", re.ErrorCode, mail.ErrAuth)
	}
	return err
}
