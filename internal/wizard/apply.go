package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glw907/poplar/internal/config"
)

// Apply converts wizard state into a config.AccountConfig ready for
// rendering + writing. It does not touch disk; the orchestrator does
// that after the user accepts the confirm screen.
//
// An identity block is synthesized when IdentityName or Signature is
// set. When both are empty Identities is left nil and the config
// decoder fills in a bare-email identity at load time (ADR-0177).
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
		// this the round-trip fails the missing-smtp.host validator.
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
		config.ResolvePreset(&cfg)
		switch preset.CredentialStrategy {
		case config.StrategyAppPassword:
			cfg.Password = m.Password
		case config.StrategyAPIToken:
			cfg.Password = m.Token
		case config.StrategyOAuth:
			if !m.OAuthDone {
				return config.AccountConfig{}, fmt.Errorf("oauth consent not completed for %q", m.Preset)
			}
			cfg.Auth = "xoauth2"
			cfg.OAuth = &config.OAuthConfig{
				ClientID:     m.OAuthCID,
				ClientSecret: m.OAuthSecret,
			}
			cfg.OAuthStore = m.OAuthStore
			cfg.OAuthMode = m.OAuthMode
		default:
			return config.AccountConfig{}, fmt.Errorf("provider %q has no credential strategy", m.Preset)
		}
	}

	if m.IdentityName != "" || m.Signature != "" {
		id := config.Identity{Name: m.IdentityName, Email: m.Email}
		if m.Signature != "" {
			id.Signatures = []config.Signature{
				{Name: "default", Text: m.Signature},
			}
		}
		cfg.Identities = []config.Identity{id}
	}

	return cfg, nil
}

// FromAccount reverses Apply: it seeds a Model from an existing
// AccountConfig so --repair can drop the user back into the wizard's
// account section with the broken block's known-good fields filled
// in. Credentials never round-trip; repair re-collects them.
func FromAccount(cfg config.AccountConfig) Model {
	m := Model{
		Email:        cfg.Email,
		AccountLabel: cfg.Name,
	}
	if len(cfg.Identities) > 0 {
		m.IdentityName = cfg.Identities[0].Name
		if sigs := cfg.Identities[0].Signatures; len(sigs) > 0 {
			m.Signature = strings.TrimPrefix(sigs[0].Text, "-- \n")
		}
	}
	switch {
	case cfg.Preset != "":
		m.Preset = cfg.Preset
	case cfg.Backend == "imap":
		m.Preset = "imap"
		m.Host = cfg.Host
		if cfg.Port > 0 {
			m.Port = strconv.Itoa(cfg.Port)
		}
		m.InsecureTLS = cfg.InsecureTLS
	case cfg.Backend == "jmap":
		m.Preset = "jmap"
		m.SessionURL = cfg.Source
		m.InsecureTLS = cfg.InsecureTLS
	}
	return m
}
