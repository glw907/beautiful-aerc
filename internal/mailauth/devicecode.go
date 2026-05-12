package mailauth

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// DeviceDisplay is the user-facing callback for AuthorizeDeviceCode's
// single-shot form: show user_code at verification_uri, wait. Called
// exactly once after the device-auth POST returns.
type DeviceDisplay func(userCode, verificationURI, verificationURIComplete string)

// DeviceAuth carries the RFC 8628 device-auth response. The token-
// polling step needs DeviceCode + the timing fields; the wizard
// needs UserCode + VerificationURI to render. Returned by
// RequestDeviceAuth and consumed by PollDeviceCode.
type DeviceAuth struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int
	Interval                int
}

type deviceAuthResp struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURL         string `json:"verification_url"` // Google's legacy field
	VerificationURI         string `json:"verification_uri"` // RFC 8628 canonical
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type deviceTokenResp struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// AuthorizeDeviceCode is the single-shot convenience over
// RequestDeviceAuth + PollDeviceCode. Suitable for callers that
// don't need to render intermediate state.
//
// Requires the OAuth client to be registered as a public / device-
// flow client at the provider. Google's application type for this
// is "TVs and Limited Input devices"; Microsoft requires a public
// client with device-code enabled.
func (c *Client) AuthorizeDeviceCode(ctx context.Context, display DeviceDisplay) error {
	da, err := c.RequestDeviceAuth(ctx)
	if err != nil {
		return err
	}
	display(da.UserCode, da.VerificationURI, da.VerificationURIComplete)
	return c.PollDeviceCode(ctx, da)
}

// RequestDeviceAuth POSTs to DeviceAuthURL and returns the parsed
// device-auth response. ErrDeviceCodeUnsupported when the preset has
// no device endpoint.
func (c *Client) RequestDeviceAuth(ctx context.Context) (*DeviceAuth, error) {
	if c.cfg.DeviceAuthURL == "" {
		return nil, ErrDeviceCodeUnsupported
	}
	form := url.Values{"client_id": {c.cfg.ClientID}}
	if len(c.cfg.Scopes) > 0 {
		form.Set("scope", strings.Join(c.cfg.Scopes, " "))
	}
	body, status, err := postForm(ctx, c.cfg.DeviceAuthURL, form)
	if err != nil {
		return nil, fmt.Errorf("device auth: %w", err)
	}
	if status/100 != 2 {
		return nil, fmt.Errorf("device auth %d: %s", status, strings.TrimSpace(string(body)))
	}
	var dar deviceAuthResp
	if err := json.Unmarshal(body, &dar); err != nil {
		return nil, fmt.Errorf("device auth decode: %w", err)
	}
	if dar.DeviceCode == "" || dar.UserCode == "" {
		return nil, fmt.Errorf("device auth missing device_code/user_code")
	}
	// RFC 8628 §3.2 leaves expires_in optional; floor a missing or
	// non-positive value to 5 minutes so PollDeviceCode runs at least
	// one iteration rather than tripping ErrConsentTimeout immediately.
	expires := dar.ExpiresIn
	if expires <= 0 {
		expires = 300
	}
	return &DeviceAuth{
		DeviceCode:              dar.DeviceCode,
		UserCode:                dar.UserCode,
		VerificationURI:         cmp.Or(dar.VerificationURI, dar.VerificationURL),
		VerificationURIComplete: dar.VerificationURIComplete,
		ExpiresIn:               expires,
		Interval:                dar.Interval,
	}, nil
}

// defaultDevicePollInterval is the fallback used when the provider
// omits an `interval` field in the device-auth response. RFC 8628
// §3.2 recommends 5 seconds; the field becomes a named site for
// mutation testing to pin against.
var defaultDevicePollInterval = 5 * time.Second

// PollDeviceCode polls TokenURL per da.Interval (with RFC 8628 §3.5
// slow_down handling) until success, denial, or da.ExpiresIn elapses.
// On success the refresh token is persisted via the configured store.
func (c *Client) PollDeviceCode(ctx context.Context, da *DeviceAuth) error {
	interval := time.Duration(da.Interval) * time.Second
	if interval <= 0 {
		interval = defaultDevicePollInterval
	}
	deadline := time.Now().Add(time.Duration(da.ExpiresIn) * time.Second)

	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		if time.Now().After(deadline) {
			return ErrConsentTimeout
		}
		timer.Reset(interval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}

		tok, slowDown, err := pollDeviceToken(ctx, c.cfg, da.DeviceCode)
		if slowDown {
			interval += 5 * time.Second
			continue
		}
		if err != nil {
			return err
		}
		if tok != nil {
			return c.store.Set(c.slug, tok.RefreshToken)
		}
	}
}

// postForm issues a form-encoded POST and returns the response body
// + status. Shared by RequestDeviceAuth and pollDeviceToken.
func postForm(ctx context.Context, endpoint string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// pollDeviceToken returns (token, slowDown, err). slowDown signals
// RFC 8628 §3.5 slow_down.
func pollDeviceToken(ctx context.Context, cfg Config, deviceCode string) (*oauth2.Token, bool, error) {
	form := url.Values{
		"client_id":   {cfg.ClientID},
		"device_code": {deviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	body, status, err := postForm(ctx, cfg.TokenURL, form)
	if err != nil {
		return nil, false, fmt.Errorf("token poll: %w", err)
	}
	var dt deviceTokenResp
	_ = json.Unmarshal(body, &dt)

	if status/100 == 2 && dt.AccessToken != "" {
		tok := &oauth2.Token{
			AccessToken:  dt.AccessToken,
			TokenType:    dt.TokenType,
			RefreshToken: dt.RefreshToken,
		}
		if dt.ExpiresIn > 0 {
			tok.Expiry = time.Now().Add(time.Duration(dt.ExpiresIn) * time.Second)
		}
		return tok, false, nil
	}
	switch dt.Error {
	case "authorization_pending":
		return nil, false, nil
	case "slow_down":
		return nil, true, nil
	case "expired_token":
		return nil, false, ErrConsentTimeout
	case "access_denied":
		return nil, false, ErrDeviceConsentDenied
	}
	return nil, false, fmt.Errorf("token poll %d: %s", status, strings.TrimSpace(string(body)))
}
