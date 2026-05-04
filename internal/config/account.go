// SPDX-License-Identifier: MIT

// Package config holds poplar's configuration types and loaders.
package config

import (
	"github.com/emersion/go-message/mail"
)

// AccountConfig holds the configuration for a single email account.
// This replaces aerc's config.AccountConfig with only the fields
// that the forked workers actually use.
type AccountConfig struct {
	Name           string
	Display        string
	Backend        string
	Source         string
	Params         map[string]string
	Folders        []string
	FoldersExclude []string

	// Identity
	From   *mail.Address
	CopyTo []string

	// Credentials
	// Password is the bearer token or password after env-var substitution.
	// In config.toml use "$VAR_NAME" to pull from the environment.
	Password string

	// PasswordCmd is a shell command whose stdout becomes the
	// password. Deferred to first Connect so secret-manager prompts
	// fire near the action. Mutually exclusive with Password.
	PasswordCmd string

	// Auth — recognized values: "plain", "login", "cram-md5",
	// "xoauth2", "bearer". Empty string defers to backend default.
	Auth string

	// Email is the user's address. May be empty when the backend
	// auto-discovers (e.g. JMAP session).
	Email string

	// IMAP transport (set directly via config.toml or via a
	// provider preset).
	Host     string
	Port     int
	StartTLS bool

	// InsecureTLS skips TLS certificate verification. Intended for
	// self-hosted IMAP servers with self-signed certs and local
	// development (e.g., Dovecot in Docker). Never set for hosted
	// providers.
	InsecureTLS bool

	// GmailQuirks enables Gmail-specific IMAP behavior in mailimap:
	// X-GM-EXT-1 assertion at Connect, and Destroy routed via
	// SELECT [Gmail]/Trash before EXPUNGE so EXPUNGE truly deletes.
	// Set automatically by the gmail preset; never set by hand.
	GmailQuirks bool
}
