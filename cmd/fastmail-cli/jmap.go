package main

import (
	"fmt"
	"os"

	"github.com/glw907/beautiful-aerc/internal/jmap"
)

func newJMAPSession() (*jmap.Session, error) {
	token := os.Getenv("FASTMAIL_API_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("FASTMAIL_API_TOKEN not set")
	}
	return jmap.NewSession(token, jmap.DefaultSessionURL)
}
