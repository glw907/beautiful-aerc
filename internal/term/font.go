// SPDX-License-Identifier: MIT

package term

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/adrg/sysfont"
)

var (
	hasNerdFontOnce   sync.Once
	hasNerdFontResult bool
)

// HasNerdFont reports whether a Nerd Font is installed on this system.
// On Linux, uses fc-list as the primary source because sysfont misses
// fonts installed under ~/.local/share/fonts. Falls back to sysfont
// enumeration when fc-list is unavailable or exits non-zero.
// Subsequent calls return the cached result.
func HasNerdFont() bool {
	hasNerdFontOnce.Do(func() {
		if families, ok := fcListFamilies(); ok {
			hasNerdFontResult = hasNerdFontIn(families)
			return
		}
		fonts := sysfont.NewFinder(nil).List()
		families := make([]string, 0, len(fonts))
		for _, f := range fonts {
			families = append(families, f.Family)
		}
		hasNerdFontResult = hasNerdFontIn(families)
	})
	return hasNerdFontResult
}

// fcListFamilies shells out to fc-list to enumerate font families
// known to fontconfig. Returns (families, true) on success or
// (nil, false) when fc-list is not in PATH or exits non-zero.
// A 2-second context keeps a hung fontconfig from stalling startup.
func fcListFamilies() ([]string, bool) {
	path, err := exec.LookPath("fc-list")
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, ":family")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	return parseFcList(out.String()), true
}

// parseFcList extracts font family names from fc-list stdout.
// Each line is one or more comma-separated families optionally
// followed by ":style=..." — for example:
//
//	JetBrainsMono Nerd Font,JetBrainsMono NF:style=Thin Italic
//
// The function splits on commas and colon, strips whitespace, and
// returns the de-duplicated family names.
func parseFcList(output string) []string {
	seen := make(map[string]bool)
	var families []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Drop anything after the first colon (":style=...").
		if idx := strings.Index(line, ":"); idx >= 0 {
			line = line[:idx]
		}
		for _, part := range strings.Split(line, ",") {
			f := strings.TrimSpace(part)
			if f == "" || seen[f] {
				continue
			}
			seen[f] = true
			families = append(families, f)
		}
	}
	return families
}

// hasNerdFontIn is the pure-string check; isolated for testability.
// A family qualifies if its lower-cased + trimmed name contains
// "nerd font" or ends with " nf".
func hasNerdFontIn(families []string) bool {
	for _, f := range families {
		s := strings.ToLower(strings.TrimSpace(f))
		if strings.Contains(s, "nerd font") || strings.HasSuffix(s, " nf") {
			return true
		}
	}
	return false
}
