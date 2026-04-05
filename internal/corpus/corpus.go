package corpus

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FindDir locates or creates the corpus directory. Resolution order:
// 1. $AERC_CONFIG/../../corpus/ (env override)
// 2. configHint/../../corpus/ (caller-supplied aerc config path)
// 3. ~/.config/aerc/../../corpus/ (default)
// Creates the directory if it does not exist.
func FindDir(configHint string) (string, error) {
	var candidates []string

	if aercConfig := os.Getenv("AERC_CONFIG"); aercConfig != "" {
		candidates = append(candidates, filepath.Join(aercConfig, "..", "..", "corpus"))
	}

	if configHint != "" {
		candidates = append(candidates, filepath.Join(configHint, "..", "..", "corpus"))
	}

	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "aerc", "..", "..", "corpus"))
	}

	for _, c := range candidates {
		c = filepath.Clean(c)
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c, nil
		}
	}

	if len(candidates) > 0 {
		dir := filepath.Clean(candidates[0])
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("creating corpus directory %s: %w", dir, err)
		}
		return dir, nil
	}

	return "", fmt.Errorf("cannot determine corpus directory")
}

var htmlMarkers = []string{"<html", "<head", "<body", "<!doctype", "<table"}

// IsHTML reports whether data looks like HTML by checking the first 1024 bytes
// for common HTML markers (case-insensitive).
func IsHTML(data []byte) bool {
	n := len(data)
	if n > 1024 {
		n = 1024
	}
	lower := bytes.ToLower(data[:n])
	for _, marker := range htmlMarkers {
		if bytes.Contains(lower, []byte(marker)) {
			return true
		}
	}
	return false
}

// Save writes data to dir with a timestamped filename. The extension is .html
// if IsHTML reports true, otherwise .txt. Collisions within the same second
// are resolved by appending -2, -3, etc. Returns an error for empty input.
func Save(dir string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("no input")
	}

	ext := ".txt"
	if IsHTML(data) {
		ext = ".html"
	}

	stamp := time.Now().Format("20060102-150405")
	name := stamp + ext
	path := filepath.Join(dir, name)

	if _, err := os.Stat(path); err == nil {
		for i := 2; ; i++ {
			name = fmt.Sprintf("%s-%d%s", stamp, i, ext)
			path = filepath.Join(dir, name)
			if _, err := os.Stat(path); err != nil {
				break
			}
		}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("writing corpus file %s: %w", name, err)
	}
	return path, nil
}
