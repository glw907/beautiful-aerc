package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddRule(t *testing.T) {
	dir := t.TempDir()
	env := []string{"HOME=" + dir}
	os.MkdirAll(filepath.Join(dir, ".config", "aerc"), 0755)
	rulesFile := filepath.Join(dir, ".config", "aerc", "mailrules.json")

	r := runWithEnv(t, env, "", "rules", "add", "--search", "from:test@x.com", "--folder", "Archive")
	if r.err != nil {
		t.Fatalf("add failed: %v\nstderr: %s", r.err, r.stderr)
	}
	if !strings.Contains(r.stderr, "Rule added") {
		t.Errorf("expected 'Rule added' in stderr, got: %s", r.stderr)
	}

	data, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatalf("reading rules: %v", err)
	}

	var rules []map[string]any
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("parsing rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0]["search"] != "from:test@x.com" {
		t.Errorf("search = %v", rules[0]["search"])
	}
}

func TestAddDuplicateRule(t *testing.T) {
	dir := t.TempDir()
	env := []string{"HOME=" + dir}
	os.MkdirAll(filepath.Join(dir, ".config", "aerc"), 0755)

	r := runWithEnv(t, env, "", "rules", "add", "--search", "from:a@b.com", "--folder", "X")
	if r.err != nil {
		t.Fatalf("first add: %v", r.err)
	}

	r = runWithEnv(t, env, "", "rules", "add", "--search", "from:a@b.com", "--folder", "X")
	if r.err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestAddWithEnvOverride(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom-rules.json")
	env := []string{
		"HOME=" + dir,
		"AERC_RULES_FILE=" + customPath,
	}

	r := runWithEnv(t, env, "", "rules", "add", "--search", "from:env@test.com", "--folder", "EnvFolder")
	if r.err != nil {
		t.Fatalf("add failed: %v\nstderr: %s", r.err, r.stderr)
	}

	data, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("reading rules at custom path: %v", err)
	}

	var rules []map[string]any
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("parsing rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0]["search"] != "from:env@test.com" {
		t.Errorf("search = %v", rules[0]["search"])
	}
}

func TestAddWithFlagOverride(t *testing.T) {
	dir := t.TempDir()
	flagPath := filepath.Join(dir, "flag-rules.json")
	env := []string{"HOME=" + dir}

	r := runWithEnv(t, env, "", "rules", "add", "--search", "from:flag@test.com", "--folder", "FlagFolder", "--rules-file", flagPath)
	if r.err != nil {
		t.Fatalf("add failed: %v\nstderr: %s", r.err, r.stderr)
	}

	data, err := os.ReadFile(flagPath)
	if err != nil {
		t.Fatalf("reading rules at flag path: %v", err)
	}

	var rules []map[string]any
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("parsing rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0]["search"] != "from:flag@test.com" {
		t.Errorf("search = %v", rules[0]["search"])
	}
}
