// Package main is the renderspike tool. It renders each message from
// corpus/manifest.json through three arms (lynx, w3m, legacy) and writes
// the results to corpus/renders/ for quality comparison.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// manifestEntry is one record in corpus/manifest.json.
type manifestEntry struct {
	ID    string `json:"id"`
	Class string `json:"class"`
	Path  string `json:"path"`
}

// manifest is the top-level manifest structure (fields we need).
type manifest struct {
	Entries []manifestEntry `json:"entries"`
}

type armStat struct {
	MS    int64  `json:"ms"`
	Bytes int    `json:"bytes"`
	Error string `json:"error,omitempty"`
}

type msgStat struct {
	Class  string  `json:"class"`
	Source string  `json:"source"`
	Lynx   armStat `json:"lynx"`
	W3M    armStat `json:"w3m"`
	Legacy armStat `json:"legacy"`
}

type statsFile struct {
	Generated string             `json:"generated"`
	Messages  map[string]msgStat `json:"messages"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("renderspike", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		corpusDir  string
		rendersDir string
	)
	flag.StringVar(&corpusDir, "corpus", "corpus", "path to corpus directory")
	flag.StringVar(&rendersDir, "renders", "", "output directory (default: corpus/renders)")
	flag.Parse()

	if rendersDir == "" {
		rendersDir = filepath.Join(corpusDir, "renders")
	}

	if err := ensureBinaries(); err != nil {
		return err
	}

	data, err := os.ReadFile(filepath.Join(corpusDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	slog.Info("loaded manifest", "entries", len(m.Entries))

	stats := statsFile{
		Generated: time.Now().UTC().Format(time.RFC3339),
		Messages:  make(map[string]msgStat, len(m.Entries)),
	}

	var errs int
	for _, entry := range m.Entries {
		raw, err := os.ReadFile(filepath.Join(corpusDir, entry.Path))
		if err != nil {
			slog.Warn("read eml", "id", entry.ID, "err", err)
			errs++
			continue
		}

		part, err := extractBody(raw)
		if err != nil {
			slog.Warn("extract body", "id", entry.ID, "err", err)
			errs++
			continue
		}

		lynxRes := runLynx(part.content)
		w3mRes := runW3M(part.content, part.contentType)
		legacyRes := runLegacy(part.content, part.contentType)

		for _, pair := range []struct {
			arm string
			res armResult
		}{
			{"lynx", lynxRes},
			{"w3m", w3mRes},
			{"legacy", legacyRes},
		} {
			if pair.res.errMsg != "" {
				slog.Warn("arm failed", "id", entry.ID, "arm", pair.arm, "err", pair.res.errMsg)
				continue
			}
			outPath := renderOutPath(rendersDir, pair.arm, entry.Class, entry.ID)
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("mkdir for %s: %w", outPath, err)
			}
			if err := os.WriteFile(outPath, pair.res.output, 0o644); err != nil {
				slog.Warn("write render", "id", entry.ID, "arm", pair.arm, "err", err)
			}
		}

		stats.Messages[entry.ID] = msgStat{
			Class:  entry.Class,
			Source: part.contentType,
			Lynx:   toArmStat(lynxRes),
			W3M:    toArmStat(w3mRes),
			Legacy: toArmStat(legacyRes),
		}
	}

	if err := os.MkdirAll(rendersDir, 0o755); err != nil {
		return fmt.Errorf("mkdir renders: %w", err)
	}
	statsData, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(rendersDir, "stats.json"), statsData); err != nil {
		return fmt.Errorf("write stats: %w", err)
	}

	slog.Info("done", "entries", len(m.Entries), "errors", errs, "renders", rendersDir)
	return nil
}

func toArmStat(r armResult) armStat {
	return armStat{
		MS:    r.ms,
		Bytes: len(r.output),
		Error: r.errMsg,
	}
}

// renderOutPath returns the output path for one rendered message.
func renderOutPath(dir, arm, class, id string) string {
	return filepath.Join(dir, arm, class, id+".md")
}

// writeFileAtomic writes data to path via a temp file in the same directory
// then renames into place.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
