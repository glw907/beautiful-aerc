// SPDX-License-Identifier: MIT

package mailjmap

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/config"
)

func TestResolvePasswordPrefersInline(t *testing.T) {
	cfg := &config.AccountConfig{
		Password:    "inline",
		PasswordCmd: `printf %s shouldnotrun`,
	}
	got, err := resolvePassword(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "inline" {
		t.Errorf("resolvePassword = %q, want %q", got, "inline")
	}
}

func TestResolvePasswordRunsCmd(t *testing.T) {
	cfg := &config.AccountConfig{
		PasswordCmd: `printf %s shellsecret`,
	}
	got, err := resolvePassword(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "shellsecret" {
		t.Errorf("resolvePassword = %q, want %q", got, "shellsecret")
	}
}

func TestResolvePasswordCmdFailureSurfaces(t *testing.T) {
	cfg := &config.AccountConfig{
		PasswordCmd: "false",
	}
	_, err := resolvePassword(cfg)
	if err == nil {
		t.Fatal("expected error from failing command, got nil")
	}
	if !strings.Contains(err.Error(), "password-cmd failed") {
		t.Errorf("error %q does not contain %q", err.Error(), "password-cmd failed")
	}
}

func TestResolvePasswordEmpty(t *testing.T) {
	cfg := &config.AccountConfig{}
	_, err := resolvePassword(cfg)
	if err == nil {
		t.Fatal("expected error for empty config, got nil")
	}
}
