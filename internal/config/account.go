// SPDX-License-Identifier: MIT

// Package config holds poplar's configuration types and loaders.
package config

import (
	"time"

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
	Headers        []string
	HeadersExclude []string
	CheckMail      time.Duration

	// Identity
	From    *mail.Address
	Aliases []*mail.Address
	CopyTo  []string

	// Credentials
	// Password is the bearer token or password after env-var substitution.
	// In accounts.toml use "$VAR_NAME" to pull from the environment.
	Password string

	// Auth — recognized values: "plain", "login", "cram-md5",
	// "xoauth2", "bearer". Empty string defers to backend default.
	Auth string

	// Email is the user's address. May be empty for backends that
	// auto-discover (JMAP session). Used as SASL username for IMAP.
	Email string

	// IMAP transport (set directly via accounts.toml or via a
	// provider preset).
	Host     string
	Port     int
	StartTLS bool

	// InsecureTLS skips TLS certificate verification. Intended for
	// self-hosted IMAP servers with self-signed certs and local
	// development (e.g., Dovecot in Docker). Never set for hosted
	// providers.
	InsecureTLS bool

	// XOAUTH2 inputs. All env-var-substituted via $VAR.
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRefreshToken string

	// Outgoing
	Outgoing        string
	OutgoingCredCmd string
}
