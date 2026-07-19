package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/glw907/poplar/internal/filter"
	"github.com/glw907/poplar/internal/spikerender"
)

// armResult is the output of one rendering arm for one message.
type armResult struct {
	output []byte
	ms     int64
	errMsg string
}

// ensureBinaries checks that lynx and w3m are available, attempting an
// apt-get install if either is missing.
func ensureBinaries() error {
	var missing []string
	for _, name := range []string{"lynx", "w3m"} {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	args := append([]string{"apt-get", "install", "-y"}, missing...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt install %v: %w", missing, err)
	}
	return nil
}

const armTimeout = 15 * time.Second

// runLynx renders content through lynx -dump -nolist, reading from stdin.
func runLynx(content []byte) armResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), armTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lynx", "-dump", "-nolist", "-stdin")
	cmd.Stdin = bytes.NewReader(content)
	out, err := cmd.Output()
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return armResult{ms: ms, errMsg: err.Error()}
	}
	return armResult{output: out, ms: ms}
}

// runW3M renders content through w3m -dump, reading from stdin.
func runW3M(content []byte, contentType string) armResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), armTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "w3m", "-dump", "-T", contentType, "-")
	cmd.Stdin = bytes.NewReader(content)
	out, err := cmd.Output()
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return armResult{ms: ms, errMsg: err.Error()}
	}
	return armResult{output: out, ms: ms}
}

// runLegacy renders content through the internal filter pipeline.
func runLegacy(content []byte, contentType string) armResult {
	start := time.Now()
	var out string
	if contentType == "text/html" {
		out = filter.CleanHTML(string(content))
	} else {
		out = filter.CleanPlain(string(content))
	}
	ms := time.Since(start).Milliseconds()
	return armResult{output: []byte(out), ms: ms}
}

// runIterated renders content through the iterated pipeline: go-readability
// preprocessing plus the legacy filter pipeline plus email-specific rules.
func runIterated(content []byte, contentType string) armResult {
	start := time.Now()
	out := spikerender.Render(content, contentType)
	ms := time.Since(start).Milliseconds()
	return armResult{output: []byte(out), ms: ms}
}
