package wizard

import (
	"fmt"
	"strconv"

	"github.com/glw907/poplar/internal/config"
)

// Apply converts wizard state into a config.AccountConfig ready for
// rendering + writing. It does not touch disk. the orchestrator does
// that after the user accepts the confirm screen.
//
// Identity defaults to one entry built from Email + IdentityName when
// IdentityName is set; ADR-0177 requires len(Identities) >= 1, so a
// missing display name falls through to the bare-email identity.
func Apply(m Model) (config.AccountConfig, error) {
	preset, ok := config.Providers[m.Preset]
	plain := m.Preset == "imap" || m.Preset == "jmap"
	if !ok && !plain {
		return config.AccountConfig{}, fmt.Errorf("unknown provider %q", m.Preset)
	}

	cfg := config.AccountConfig{
		Name:  m.AccountLabel,
		Email: m.Email,
	}
	if cfg.Name == "" {
		cfg.Name = m.Email
	}

	switch {
	case plain && m.Preset == "imap":
		cfg.Backend = "imap"
		cfg.Host = m.Host
		port, err := strconv.Atoi(m.Port)
		if err != nil || port < 1 || port > 65535 {
			return config.AccountConfig{}, fmt.Errorf("port %q out of range", m.Port)
		}
		cfg.Port = port
		cfg.InsecureTLS = m.InsecureTLS
		cfg.Password = m.Password
		// SMTP defaults to the same host on submission ports; without
		// this the round-trip is rejected by the missing-smtp.host
		// validator. The user can edit afterwards.
		cfg.SMTP = config.SMTPConfig{
			Host: m.Host,
			Port: 587,
		}
	case plain && m.Preset == "jmap":
		cfg.Backend = "jmap"
		cfg.Source = m.SessionURL
		cfg.InsecureTLS = m.InsecureTLS
		cfg.Password = m.Token
	default:
		cfg.Preset = m.Preset
		cfg.Backend = preset.Backend
		switch preset.CredentialStrategy {
		case config.StrategyAppPassword:
			cfg.Password = m.Password
		case config.StrategyAPIToken:
			cfg.Password = m.Token
		case config.StrategyOAuth:
			// TODO(Pass 14.1): OAuth + keyring flow.
			cfg.PasswordCmd = "echo TODO-pass-14.1-oauth"
		default:
			return config.AccountConfig{}, fmt.Errorf("provider %q has no credential strategy", m.Preset)
		}
	}

	if m.IdentityName != "" {
		cfg.Identities = []config.Identity{
			{Name: m.IdentityName, Email: m.Email},
		}
	}

	return cfg, nil
}
