//go:build dev

package main

import (
	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/mail"
)

func openMockBackend(_ config.AccountConfig) (mail.Backend, error) {
	return mail.NewMockBackend(), nil
}
