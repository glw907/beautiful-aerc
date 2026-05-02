// SPDX-License-Identifier: MIT

package mailimap

import (
	"fmt"
	"os"
	"path/filepath"
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

func TestResolvedPassword_XOAUTH2_BypassesCache(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "n")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Each invocation increments the counter file and prints the new value.
	cmd := fmt.Sprintf(
		`n=$(cat %q); n=$((n+1)); printf %%s "$n" > %q; printf %%s "$n"`,
		counter, counter)

	b := New(config.AccountConfig{
		Name:        "g",
		Auth:        "xoauth2",
		PasswordCmd: cmd,
	})

	first, err := b.resolvedPassword()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := b.resolvedPassword()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first == second {
		t.Errorf("xoauth2 should not cache: first=%q second=%q (want different)", first, second)
	}
}

func TestResolvedPassword_NonXOAUTH2_Caches(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "n")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := fmt.Sprintf(
		`n=$(cat %q); n=$((n+1)); printf %%s "$n" > %q; printf %%s "$n"`,
		counter, counter)

	b := New(config.AccountConfig{
		Name:        "f",
		Auth:        "plain",
		PasswordCmd: cmd,
	})

	first, _ := b.resolvedPassword()
	second, _ := b.resolvedPassword()
	if first != second {
		t.Errorf("plain auth should cache: first=%q second=%q (want equal)", first, second)
	}
}
