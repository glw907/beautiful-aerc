package wizard

import (
	"fmt"

	"github.com/glw907/poplar/internal/config"
)

// SelectStrategy returns the credential strategy for a provider preset.
// The "imap" and "jmap" sentinels (self-hosted, no preset) route to
// the plainIMAP / plainJMAP strategies directly.
func SelectStrategy(preset string) (config.CredentialStrategy, error) {
	switch preset {
	case "imap":
		return config.StrategyPlainIMAP, nil
	case "jmap":
		return config.StrategyPlainJMAP, nil
	}
	p, ok := config.Providers[preset]
	if !ok {
		return "", fmt.Errorf("unknown provider preset %q", preset)
	}
	if p.CredentialStrategy == "" {
		return "", fmt.Errorf("provider %q has no credential strategy", preset)
	}
	return p.CredentialStrategy, nil
}
