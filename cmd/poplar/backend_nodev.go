//go:build !dev

package main

import (
	"fmt"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func openMockBackend(acct config.AccountConfig) (mail.Backend, error) {
	return nil, fmt.Errorf("mock backend for %q: not available in release builds (rebuild with -tags dev)", acct.Name)
}
