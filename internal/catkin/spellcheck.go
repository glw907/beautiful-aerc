package catkin

import (
	"bufio"
	"embed"
	"io"
	"strings"
)

//go:embed spellcheck/en_US.txt spellcheck/project.txt
var spellcheckFS embed.FS

// Speller is a lightweight in-memory spellchecker. Construct via
// NewSpeller. A nil receiver makes Check return true, so callers
// without a Speller degrade to no-op rather than spurious errors.
type Speller struct {
	known map[string]uint32 // lowercased word -> frequency rank (lower = more frequent)
	// SymSpell deletion-distance index, populated in Task 5.
	// Speller.Suggest uses it. Check does not.
	delIdx map[string][]string
}

// NewSpeller loads en_US + project from the embedded filesystem
// and unions extra (user wordlist) on top. Extra entries take
// precedence in case-folding and gain max-frequency rank so they
// outrank similar dictionary words in suggestions.
func NewSpeller(extra []string) (*Speller, error) {
	en, err := spellcheckFS.Open("spellcheck/en_US.txt")
	if err != nil {
		return nil, err
	}
	defer en.Close()
	proj, err := spellcheckFS.Open("spellcheck/project.txt")
	if err != nil {
		return nil, err
	}
	defer proj.Close()
	return newSpellerFromReader(en, proj, extra)
}

// newSpellerFromReader is the test-friendly constructor. project may be nil.
func newSpellerFromReader(en, project io.Reader, extra []string) (*Speller, error) {
	known := make(map[string]uint32, 50000)
	if err := loadInto(known, en, false); err != nil {
		return nil, err
	}
	if project != nil {
		if err := loadInto(known, project, true); err != nil {
			return nil, err
		}
	}
	for _, w := range extra {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" || strings.HasPrefix(w, "#") {
			continue
		}
		known[w] = 1 // max frequency for user terms
	}
	return &Speller{known: known}, nil
}

// loadInto reads one-word-per-line wordlists. Comments (#) and blank lines
// are skipped. If maxFreq is true, every entry is inserted at rank 1
// (project allowlist behavior).
func loadInto(dst map[string]uint32, r io.Reader, maxFreq bool) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	var rank uint32
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		w := strings.ToLower(line)
		rank++
		if _, ok := dst[w]; ok {
			continue
		}
		if maxFreq {
			dst[w] = 1
		} else {
			dst[w] = rank
		}
	}
	return sc.Err()
}

// Check reports whether word is in the dictionary. Comparison is
// case-insensitive. Empty strings return false.
func (s *Speller) Check(word string) bool {
	if s == nil {
		return true
	}
	if word == "" {
		return false
	}
	_, ok := s.known[strings.ToLower(word)]
	return ok
}
