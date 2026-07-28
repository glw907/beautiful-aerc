// Package keyring resolves poplar's stored credentials. v1 has one:
// the Fastmail JMAP bearer token. Account onboarding and multi-account
// storage are pass 2's.
package keyring

import (
	"errors"
	"os"
)

// EnvFastmailToken is the environment variable poplar falls back to
// for the Fastmail JMAP bearer token when no account config supplies
// one yet.
//
//nolint:gosec // G101: this is an env-var name, not a credential value
const EnvFastmailToken = "FASTMAIL_API_TOKEN"

// Token resolves the Fastmail bearer token: configured, when
// non-empty, otherwise EnvFastmailToken.
func Token(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	if token := os.Getenv(EnvFastmailToken); token != "" {
		return token, nil
	}
	return "", errors.New("keyring: no fastmail token: set account config or " + EnvFastmailToken)
}
