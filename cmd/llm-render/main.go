// Package main is the llm-render tool. It renders each message from
// corpus/manifest.json through an LLM arm (claude -p on Haiku) and
// merges per-message latency and token counts into corpus/renders/stats.json.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

// manifestEntry is one record in corpus/manifest.json.
type manifestEntry struct {
	ID    string `json:"id"`
	Class string `json:"class"`
	Path  string `json:"path"`
}

// manifest is the top-level manifest structure.
type manifest struct {
	Entries []manifestEntry `json:"entries"`
}

// rawStats is the stats.json structure using RawMessage so the llm arm
// can merge into existing entries without touching other arms' data.
type rawStats struct {
	Generated string                     `json:"generated"`
	Messages  map[string]json.RawMessage `json:"messages"`
}

// entryResult carries one message's outcome back to the main goroutine.
// skip is set when a valid llm stat already exists in stats.json and the
// render file is present; in that case the main loop leaves the entry alone.
type entryResult struct {
	id    string
	class string
	stat  llmStat
	skip  bool
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("llm-render", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		corpusDir   string
		rendersDir  string
		principlesF string
		promptOut   string
		parallel    int
		limit       int
		model       string
	)
	flag.StringVar(&corpusDir, "corpus", "corpus", "path to corpus directory")
	flag.StringVar(&rendersDir, "renders", "", "output directory (default: corpus/renders)")
	flag.StringVar(&principlesF, "principles", "docs/poplar/research/2026-07-19-rendering-readability-principles.md", "rendering principles file")
	flag.StringVar(&promptOut, "prompt-out", "", "where to save prompt template (default: corpus/llm-prompt.md)")
	flag.IntVar(&parallel, "parallel", 4, "max concurrent LLM calls")
	flag.IntVar(&limit, "limit", 0, "process at most N entries (0 = all)")
	flag.StringVar(&model, "model", "claude-haiku-4-5-20251001", "model name")
	flag.Parse()

	if rendersDir == "" {
		rendersDir = filepath.Join(corpusDir, "renders")
	}
	if promptOut == "" {
		promptOut = filepath.Join(corpusDir, "llm-prompt.md")
	}
	llmDir := filepath.Join(rendersDir, "llm")

	principlesBytes, err := os.ReadFile(principlesF)
	if err != nil {
		return fmt.Errorf("read principles: %w", err)
	}
	principles := string(principlesBytes)

	if err := savePromptTemplate(promptOut, principles); err != nil {
		return fmt.Errorf("save prompt template: %w", err)
	}

	manifestData, err := os.ReadFile(filepath.Join(corpusDir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	slog.Info("loaded manifest", "entries", len(m.Entries))
	if limit > 0 && limit < len(m.Entries) {
		m.Entries = m.Entries[:limit]
	}

	s, err := loadStats(filepath.Join(rendersDir, "stats.json"))
	if err != nil {
		return fmt.Errorf("load stats: %w", err)
	}

	// validLLM is a pre-built snapshot (read-only in goroutines) of which
	// entries already have a non-error llm stat with recorded latency.
	// Using a snapshot avoids a data race with the results-processing loop
	// that writes to s.Messages concurrently.
	validLLM := make(map[string]bool, len(m.Entries))
	for _, e := range m.Entries {
		validLLM[e.ID] = hasValidLLMStat(s.Messages[e.ID])
	}

	results := make(chan entryResult, len(m.Entries))
	sem := semaphore.NewWeighted(int64(parallel))
	ctx := context.Background()
	var wg sync.WaitGroup

	for _, entry := range m.Entries {
		wg.Add(1)
		go func(e manifestEntry) {
			defer wg.Done()

			// If the render file already exists, skip or record token count
			// without making another LLM call.
			outPath := renderOutPath(llmDir, e.Class, e.ID)
			if existing, readErr := os.ReadFile(outPath); readErr == nil {
				if validLLM[e.ID] {
					// Latency already recorded; leave stats.json untouched.
					results <- entryResult{id: e.ID, class: e.Class, skip: true}
					return
				}
				results <- entryResult{
					id:    e.ID,
					class: e.Class,
					stat:  llmStat{Tokens: tokenEst(string(existing))},
				}
				return
			}

			if err := sem.Acquire(ctx, 1); err != nil {
				results <- entryResult{id: e.ID, class: e.Class, stat: llmStat{Error: "semaphore: " + err.Error()}}
				return
			}
			defer sem.Release(1)

			stat := processEntry(ctx, e, corpusDir, llmDir, principles, model)
			results <- entryResult{id: e.ID, class: e.Class, stat: stat}
		}(entry)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var errCount int
	for r := range results {
		if r.skip {
			continue
		}
		existing := s.Messages[r.id]
		if existing == nil {
			init := map[string]any{"class": r.class}
			existing, _ = json.Marshal(init)
		}
		merged, mergeErr := mergeEntry(existing, r.stat)
		if mergeErr != nil {
			slog.Warn("merge entry", "id", r.id, "err", mergeErr)
			continue
		}
		s.Messages[r.id] = merged
		if r.stat.Error != "" {
			errCount++
		}
	}

	s.Generated = time.Now().UTC().Format(time.RFC3339)
	if err := writeStats(filepath.Join(rendersDir, "stats.json"), s); err != nil {
		return fmt.Errorf("write stats: %w", err)
	}

	slog.Info("done", "entries", len(m.Entries), "errors", errCount, "llm-dir", llmDir)
	return nil
}

// processEntry renders one message through the LLM arm and writes the output
// file. It always returns an llmStat; errors are recorded inside the stat.
func processEntry(ctx context.Context, e manifestEntry, corpusDir, llmDir, principles, model string) llmStat {
	raw, err := os.ReadFile(filepath.Join(corpusDir, e.Path))
	if err != nil {
		return llmStat{Error: "read eml: " + err.Error()}
	}

	part, err := extractBody(raw)
	if err != nil {
		return llmStat{Error: "extract body: " + err.Error()}
	}

	prompt := buildPrompt(principles, string(part.content))
	output, ms, callErr := runLLM(ctx, prompt, model)
	if callErr != nil {
		slog.Warn("LLM call failed", "id", e.ID, "err", callErr)
		return llmStat{MS: ms, Error: callErr.Error()}
	}

	outPath := renderOutPath(llmDir, e.Class, e.ID)
	if mkErr := os.MkdirAll(filepath.Dir(outPath), 0o755); mkErr != nil {
		slog.Warn("mkdir render", "id", e.ID, "err", mkErr)
		return llmStat{MS: ms, Tokens: tokenEst(output), Error: "mkdir: " + mkErr.Error()}
	}
	if wErr := os.WriteFile(outPath, []byte(output), 0o644); wErr != nil {
		slog.Warn("write render", "id", e.ID, "err", wErr)
		return llmStat{MS: ms, Tokens: tokenEst(output), Error: "write: " + wErr.Error()}
	}

	return llmStat{MS: ms, Tokens: tokenEst(output)}
}

func loadStats(path string) (rawStats, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return rawStats{Messages: make(map[string]json.RawMessage)}, nil
	}
	if err != nil {
		return rawStats{}, fmt.Errorf("read stats: %w", err)
	}
	var s rawStats
	if err := json.Unmarshal(data, &s); err != nil {
		return rawStats{}, fmt.Errorf("parse stats: %w", err)
	}
	if s.Messages == nil {
		s.Messages = make(map[string]json.RawMessage)
	}
	return s, nil
}

func writeStats(path string, s rawStats) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}
	return writeFileAtomic(path, data)
}

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

// hasValidLLMStat reports whether the raw entry JSON already contains a
// non-error llm stat with recorded latency. Used to avoid clobbering a
// previously-recorded ms value on resume.
func hasValidLLMStat(raw json.RawMessage) bool {
	if raw == nil {
		return false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	llmRaw, ok := obj["llm"]
	if !ok {
		return false
	}
	var stat llmStat
	if err := json.Unmarshal(llmRaw, &stat); err != nil {
		return false
	}
	return stat.Error == "" && stat.MS > 0
}

// renderOutPath returns the output path for one LLM-rendered message.
func renderOutPath(dir, class, id string) string {
	return filepath.Join(dir, class, id+".md")
}

// savePromptTemplate writes the prompt template (with placeholders) to path.
func savePromptTemplate(path, principles string) error {
	tmpl := strings.ReplaceAll(promptTemplate, "{{PRINCIPLES}}", principles)
	tmpl = strings.ReplaceAll(tmpl, "{{EMAIL}}", "{{EMAIL_CONTENT_PLACEHOLDER}}")
	return os.WriteFile(path, []byte(tmpl), 0o644)
}
