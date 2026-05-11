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

// HasNerdFont reports whether a Nerd Font is installed. On Linux it
// uses fc-list because sysfont misses fonts under
// ~/.local/share/fonts, falling back to sysfont when fc-list is
// unavailable. The result is cached.
var HasNerdFont = sync.OnceValue(func() bool {
	if families, ok := fcListFamilies(); ok {
		return hasNerdFontIn(families)
	}
	fonts := sysfont.NewFinder(nil).List()
	families := make([]string, 0, len(fonts))
	for _, f := range fonts {
		families = append(families, f.Family)
	}
	return hasNerdFontIn(families)
})

// fcListFamilies shells out to fc-list with a 2-second deadline so a
// hung fontconfig cannot stall startup.
func fcListFamilies() ([]string, bool) {
	path, err := exec.LookPath("fc-list")
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-f", "%{family}\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	return parseFcList(out.String()), true
}

// parseFcList extracts family names from fc-list output. The
// preferred form is `-f "%{family}\n"` (one comma-separated family
// list per line). The legacy `path:family[,family]:style=...` form is
// also tolerated.
func parseFcList(output string) []string {
	seen := make(map[string]bool)
	var families []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ": "); idx >= 0 {
			line = line[idx+2:]
		}
		if idx := strings.Index(line, ":style="); idx >= 0 {
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

// hasNerdFontIn matches lower-cased family names that contain
// "nerd font" or end with " nf".
func hasNerdFontIn(families []string) bool {
	for _, f := range families {
		s := strings.ToLower(strings.TrimSpace(f))
		if strings.Contains(s, "nerd font") || strings.HasSuffix(s, " nf") {
			return true
		}
	}
	return false
}
