package wizard

import (
	"context"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mailauth"
)

// Authorizer is the test seam for the OAuth client driving Apply.
// In production mailauth.Client implements it.
type Authorizer interface {
	Authorize(ctx context.Context) error
	Backend() mailauth.Backend
}

type oauthStrategy struct {
	cli    Authorizer
	cid    string
	secret string
}

// NewOAuthStrategy returns a credential strategy that runs the PKCE
// authorization flow through cli and returns an OAuth-shaped AccountConfig.
func NewOAuthStrategy(cli Authorizer, preset, email, cid, secret string) *oauthStrategy {
	return &oauthStrategy{cli: cli, cid: cid, secret: secret}
}

// Apply runs the consent flow and returns a config with OAuth fields
// populated. PasswordCmd is explicitly empty; the runtime uses the stored
// refresh token instead.
func (s *oauthStrategy) Apply(ctx context.Context) (config.AccountConfig, error) {
	if err := s.cli.Authorize(ctx); err != nil {
		return config.AccountConfig{}, err
	}
	return config.AccountConfig{
		Auth: "xoauth2",
		OAuth: &config.OAuthConfig{
			ClientID:     s.cid,
			ClientSecret: s.secret,
			// AuthURL / TokenURL / Scopes fill from the preset at Load time
			// when the provider's OAuth defaults are non-nil.
		},
		OAuthStore: string(s.cli.Backend()),
	}, nil
}
