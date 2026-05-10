package mailauth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/glw907/poplar/internal/mail"
	"golang.org/x/oauth2"
)

// Config holds the OAuth2 client parameters for a single provider.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	Scopes       []string
	// RedirectPortRange is reserved for the loopback PKCE server (Task 5).
	RedirectPortRange [2]int
}

// Client obtains and refreshes access tokens via a stored refresh token.
// It is safe for concurrent use.
type Client struct {
	cfg   Config
	store TokenStore
	slug  string

	mu        sync.Mutex
	cachedTok string
	expires   time.Time
}

// NewClient returns a Client backed by store, keyed on slug.
func NewClient(cfg Config, store TokenStore, slug string) *Client {
	return &Client{cfg: cfg, store: store, slug: slug}
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
