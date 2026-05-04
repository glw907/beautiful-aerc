// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"
)

func TestResolvePassword_InlineWins(t *testing.T) {
	cfg := &AccountConfig{
		Password:    "inline",
		PasswordCmd: `printf %s shouldnotrun`,
	}
	got, err := cfg.ResolvePassword()
	if err != nil {
		t.Fatalf("ResolvePassword: %v", err)
	}
	if got != "inline" {
		t.Errorf("got %q, want %q", got, "inline")
	}
}

func TestResolvePassword_RunsCmd(t *testing.T) {
	cfg := &AccountConfig{PasswordCmd: `printf %s shellsecret`}
	got, err := cfg.ResolvePassword()
	if err != nil {
		t.Fatalf("ResolvePassword: %v", err)
	}
	if got != "shellsecret" {
		t.Errorf("got %q, want %q", got, "shellsecret")
	}
}

func TestResolvePassword_CmdFailure(t *testing.T) {
	cfg := &AccountConfig{PasswordCmd: "false"}
	_, err := cfg.ResolvePassword()
	if err == nil {
		t.Fatal("want error from failing command")
	}
	if !strings.Contains(err.Error(), "password-cmd") {
		t.Errorf("error %q omits password-cmd context", err.Error())
	}
}

func TestResolvePassword_Empty(t *testing.T) {
	if _, err := (&AccountConfig{}).ResolvePassword(); err == nil {
		t.Fatal("want error for empty config")
	}
}
