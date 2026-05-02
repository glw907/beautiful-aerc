// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
	"github.com/glw907/poplar/internal/mailimap"
	"github.com/glw907/poplar/internal/mailjmap"
)

// openBackend constructs a mail.Backend for the given account based on
// its backend type. Mock is the default for unconfigured (or test) accounts.
func openBackend(acct config.AccountConfig) (mail.Backend, error) {
	switch acct.Backend {
	case "mock", "":
		return mail.NewMockBackend(), nil
	case "jmap":
		return mailjmap.New(acct), nil
	case "imap":
		return mailimap.New(acct), nil
	default:
		return nil, fmt.Errorf("unknown backend %q for account %q", acct.Backend, acct.Name)
	}
}
