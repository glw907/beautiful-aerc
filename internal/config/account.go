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

	// Outgoing
	Outgoing        string
	OutgoingCredCmd string
}
