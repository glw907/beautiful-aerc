// Package wizard is the UI-free domain for the first-run setup wizard.
// It owns the data the wizard collects, the credential-strategy
// dispatch, and the connect-test routing into mailimap/mailjmap. The
// bubbletea + huh surface lives at internal/ui/wizard.
package wizard

import "github.com/glw907/poplar/internal/mail"

// Model is the wizard's collected state. The UI's per-step sub-models
// own focus + cursor state; this struct holds the values that Apply
// hands to config.AccountConfig.
type Model struct {
	// Provider section. Preset is a key in config.Providers, or the
	// raw "imap" / "jmap" sentinels for self-hosted servers.
	Preset string

	// Identity section.
	Email        string
	IdentityName string
	AccountLabel string

	// Credentials, populated by the active strategy. Other fields stay
	// zero-valued.
	Password    string
	Token       string // api-token strategy; written into AccountConfig.Password by Apply.
	Host        string
	Port        string // string so huh.Input can bind directly; parsed in Apply.
	InsecureTLS bool
	SessionURL  string

	// Theme section. Empty == use default.
	Theme string

	// Probe outcome (set by the UI's probe screen).
	Probe mail.ProbeResult
}
