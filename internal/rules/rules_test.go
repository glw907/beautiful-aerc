package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	rules, err := Load(path)
	if err != nil {
		t.Fatalf("Load non-existent file: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected empty slice, got %d rules", len(rules))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	rules := []Rule{
		{
			Name:   "from:test@example.com -> Archive",
			Search: "from:test@example.com",
			FileIn: "Archive",
		},
	}
	if err := Save(path, rules); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(loaded))
	}
	if loaded[0].Search != "from:test@example.com" {
		t.Errorf("Search = %q, want %q", loaded[0].Search, "from:test@example.com")
	}
	if loaded[0].FileIn != "Archive" {
		t.Errorf("FileIn = %q, want %q", loaded[0].FileIn, "Archive")
	}
}

func TestAddRule(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	err := Add(path, "from:a@b.com", "Notifications")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	rules, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "from:a@b.com -> Notifications" {
		t.Errorf("Name = %q", rules[0].Name)
	}
	if !rules[0].SkipInbox {
		t.Error("SkipInbox should be true")
	}
	if !rules[0].Stop {
		t.Error("Stop should be true")
	}
	if rules[0].Created == "" {
		t.Error("Created should be set")
	}
}

func TestAddDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	err := Add(path, "from:a@b.com", "Notifications")
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}
	err = Add(path, "from:a@b.com", "Notifications")
	if err == nil {
		t.Fatal("expected error for duplicate rule")
	}
}

func TestAddToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	if err := Add(path, "from:a@b.com", "Notifications"); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if err := Add(path, "from:c@d.com", "Archive"); err != nil {
		t.Fatalf("second Add: %v", err)
	}

	rules, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	path := filepath.Join(dir, "rules.json")
	rules := []Rule{{Name: "test", Search: "from:x@y.com", FileIn: "A"}}
	if err := Save(path, rules); err != nil {
		t.Fatalf("Save with nested dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}
