// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitWritesTemplate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POPLAR_CONFIG", filepath.Join(dir, "config.toml"))

	cmd := newConfigInitTemplateCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("config.toml not written: %v", err)
	}
	if !strings.HasPrefix(out.String(), "wrote ") {
		t.Errorf("output = %q, want 'wrote ...'", out.String())
	}
}

func TestConfigInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("POPLAR_CONFIG", path)
	if err := os.WriteFile(path, []byte("# pre-existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newConfigInitTemplateCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !strings.Contains(err.Error(), "use --force") {
		t.Errorf("error %q missing --force hint", err)
	}
}

func TestConfigInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("POPLAR_CONFIG", path)
	if err := os.WriteFile(path, []byte("# pre-existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newConfigInitTemplateCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	if !strings.HasPrefix(out.String(), "wrote ") {
		t.Errorf("output = %q, want 'wrote ...'", out.String())
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "pre-existing") {
		t.Error("file was not overwritten")
	}
}

func TestConfigPathPrintsResolved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("POPLAR_CONFIG", filepath.Join(dir, "config.toml"))

	cmd := newConfigPathCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("path: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got != filepath.Join(dir, "config.toml") {
		t.Errorf("path output = %q", got)
	}
}
