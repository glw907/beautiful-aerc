package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Rule represents a mail filter rule in Fastmail's import/export format.
type Rule struct {
	Name             string   `json:"name"`
	Search           string   `json:"search"`
	FileIn           string   `json:"fileIn"`
	SkipInbox        bool     `json:"skipInbox"`
	Stop             bool     `json:"stop"`
	MarkRead         bool     `json:"markRead"`
	MarkFlagged      bool     `json:"markFlagged"`
	MarkSpam         bool     `json:"markSpam"`
	Discard          bool     `json:"discard"`
	RedirectTo       []string `json:"redirectTo"`
	ShowNotification bool     `json:"showNotification"`
	Conditions       any      `json:"conditions"`
	Combinator       string   `json:"combinator"`
	SnoozeUntil      any      `json:"snoozeUntil"`
	PreviousFileIn   any      `json:"previousFileInName"`
	Created          string   `json:"created"`
	Updated          string   `json:"updated"`
}

// Load reads rules from a JSON file. Returns an empty slice if the file
// does not exist.
func Load(path string) ([]Rule, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading rules file: %w", err)
	}
	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parsing rules file: %w", err)
	}
	return rules, nil
}

// Save writes rules to a JSON file atomically. Creates parent
// directories if needed.
func Save(path string, rules []Rule) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding rules: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".tmp-rules-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	ok = true
	return nil
}

// Add appends a new rule to the rules file. Returns an error if a rule
// with the same search and folder already exists.
func Add(path, search, folder string) error {
	existing, err := Load(path)
	if err != nil {
		return err
	}

	for _, r := range existing {
		if r.Search == search && r.FileIn == folder {
			return fmt.Errorf("rule already exists: %s -> %s", search, folder)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	rule := Rule{
		Name:       search + " -> " + folder,
		Search:     search,
		FileIn:     folder,
		SkipInbox:  true,
		Stop:       true,
		Combinator: "all",
		Created:    now,
		Updated:    now,
	}

	existing = append(existing, rule)
	return Save(path, existing)
}
