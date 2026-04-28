package term

import (
	"strings"
	"sync"

	"github.com/adrg/sysfont"
)

var (
	hasNerdFontOnce   sync.Once
	hasNerdFontResult bool
)

// HasNerdFont reports whether a Nerd Font is installed on this system.
// First call enumerates installed fonts via sysfont; subsequent calls
// return the cached result. Returns false on enumeration failure.
func HasNerdFont() bool {
	hasNerdFontOnce.Do(func() {
		fonts := sysfont.NewFinder(nil).List()
		families := make([]string, 0, len(fonts))
		for _, f := range fonts {
			families = append(families, f.Family)
		}
		hasNerdFontResult = hasNerdFontIn(families)
	})
	return hasNerdFontResult
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
