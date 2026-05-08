package catkin

import (
	"bufio"
	"embed"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

//go:embed spellcheck/en_US.txt spellcheck/project.txt
var spellcheckFS embed.FS

// Speller is a lightweight in-memory spellchecker. A nil receiver makes
// Check return true so callers without a Speller degrade to no-op.
type Speller struct {
	known map[string]uint32
	// delIdx is the SymSpell deletion-distance index; Suggest builds it
	// lazily under once. Check does not need it.
	delIdx map[string][]string
	once   sync.Once
}

// NewSpeller loads en_US + project from the embedded filesystem and
// unions extra (the user wordlist) on top at max-frequency rank so user
// terms outrank similar dictionary words in suggestions.
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

// newSpellerFromReader is the test-friendly constructor; a nil project
// reader skips the project allowlist.
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
		known[w] = 1
	}
	return &Speller{known: known}, nil
}

// loadInto reads one-word-per-line wordlists, skipping comments and blanks.
// maxFreq inserts every entry at rank 1 (project allowlist behavior).
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

// Check reports whether word is in the dictionary (case-insensitive).
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

// d=2 covers >95% of single-word English typos at a modest index size.
const maxEditDistance = 2

// buildIndex generates, for each known word, the set of strings reachable
// by ≤ maxEditDistance deletions. Lookup deletes the query the same way
// and verifies real distance against the bucket.
func (s *Speller) buildIndex() {
	s.delIdx = make(map[string][]string, len(s.known)*4)
	for w := range s.known {
		for d := range deletes(w, maxEditDistance) {
			s.delIdx[d] = append(s.delIdx[d], w)
		}
	}
}

// deletes returns the set of distinct strings produced by deleting up
// to dist characters from w (the result includes w itself).
func deletes(w string, dist int) map[string]struct{} {
	out := map[string]struct{}{w: {}}
	if dist <= 0 || len(w) == 0 {
		return out
	}
	for i := 0; i < len(w); i++ {
		shorter := w[:i] + w[i+1:]
		if _, seen := out[shorter]; seen {
			continue
		}
		out[shorter] = struct{}{}
		for d := range deletes(shorter, dist-1) {
			out[d] = struct{}{}
		}
	}
	return out
}

// editDistance is plain Levenshtein capped at limit; the true distance
// surfaces as limit+1 when it would exceed limit.
func editDistance(a, b string, limit int) int {
	la, lb := len(a), len(b)
	if abs(la-lb) > limit {
		return limit + 1
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
			if curr[j] < rowMin {
				rowMin = curr[j]
			}
		}
		if rowMin > limit {
			return limit + 1
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Suggest returns up to n corrections for word, ordered by edit
// distance then frequency rank ascending.
func (s *Speller) Suggest(word string, n int) []string {
	if s == nil || word == "" || n <= 0 {
		return nil
	}
	s.once.Do(s.buildIndex)
	w := strings.ToLower(word)

	type cand struct {
		word string
		dist int
		rank uint32
	}
	seen := map[string]struct{}{}
	var cands []cand
	for d := range deletes(w, maxEditDistance) {
		for _, candidate := range s.delIdx[d] {
			if _, dup := seen[candidate]; dup {
				continue
			}
			seen[candidate] = struct{}{}
			ed := editDistance(w, candidate, maxEditDistance)
			if ed > maxEditDistance {
				continue
			}
			cands = append(cands, cand{word: candidate, dist: ed, rank: s.known[candidate]})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].dist != cands[j].dist {
			return cands[i].dist < cands[j].dist
		}
		return cands[i].rank < cands[j].rank
	})
	if len(cands) > n {
		cands = cands[:n]
	}
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.word
	}
	return out
}

// LoadUserWordlist reads one-word-per-line entries from path, skipping
// comments and blanks. A missing file is not an error.
func LoadUserWordlist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

// NewSpellcheckAnnotator wires speller into the Annotator interface.
// styles.Squiggle is baked into each annotation so the renderer skips a
// second lookup. Not safe for concurrent use.
func NewSpellcheckAnnotator(speller *Speller, styles Styles) Annotator {
	return &spellcheckAnnotator{
		speller: speller,
		styles:  styles,
		ignored: map[string]struct{}{},
	}
}

type spellcheckAnnotator struct {
	speller *Speller
	styles  Styles
	ignored map[string]struct{}
}

func (s *spellcheckAnnotator) Annotate(src string) []Annotation {
	if s.speller == nil {
		return nil
	}
	mask := buildSkipMask(src)
	var out []Annotation
	i := 0
	for i < len(src) {
		r, size := utf8.DecodeRuneInString(src[i:])
		if !isWordRune(r) {
			i += size
			continue
		}
		start := i
		for i < len(src) {
			r, size = utf8.DecodeRuneInString(src[i:])
			if !isWordRune(r) {
				break
			}
			i += size
		}
		end := i
		if mask.covers(start, end) {
			continue
		}
		word := src[start:end]
		if isAllUpperShort(word) {
			continue
		}
		if _, ig := s.ignored[strings.ToLower(word)]; ig {
			continue
		}
		if s.speller.Check(word) {
			continue
		}
		out = append(out, Annotation{
			Range:   Range{Start: start, End: end},
			Kind:    KindMisspelling,
			Style:   s.styles.Squiggle,
			Payload: MisspellingPayload{Word: word, Suggestions: s.speller.Suggest(word, 5)},
		})
	}
	return out
}

// IgnoreInSession adds word to the in-memory ignore set; subsequent
// Annotate calls treat it as known until the annotator is rebuilt.
func (s *spellcheckAnnotator) IgnoreInSession(word string) {
	s.ignored[strings.ToLower(word)] = struct{}{}
}

// isAllUpperShort reports whether word is all-upper and ≤4 runes. The
// heuristic lets common acronyms (HTTP, JMAP) pass without an allowlist.
func isAllUpperShort(word string) bool {
	n := 0
	for _, r := range word {
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
		n++
		if n > 4 {
			return false
		}
	}
	return n > 0
}

// skipMask is the set of byte ranges spellcheck must not flag: fenced
// blocks, inline code spans, and link URLs.
type skipMask struct {
	ranges []Range
}

func (m skipMask) covers(start, end int) bool {
	for _, r := range m.ranges {
		if start >= r.Start && end <= r.End {
			return true
		}
	}
	return false
}

// buildSkipMask returns the union of byte ranges spellcheck must skip:
// fenced blocks (marker + interior), inline code spans, and link URLs.
func buildSkipMask(src string) skipMask {
	var ranges []Range
	lines := strings.Split(src, "\n")
	ctxs := Classify(lines)

	off := 0
	for i, l := range lines {
		lineEnd := off + len(l)
		if ctxs[i].Kind == BlockCodeFence || ctxs[i].InsideFence {
			ranges = append(ranges, Range{Start: off, End: lineEnd})
		}
		off = lineEnd + 1
	}

	off = 0
	for _, l := range lines {
		lineOff := off
		// after advances past each matched span as walkSpans visits l.
		after := 0
		walkSpans(l, func(kind spanKind, text string, sub []string) {
			switch kind {
			case spanCode:
				if idx := strings.Index(l[after:], text); idx >= 0 {
					start := lineOff + after + idx
					ranges = append(ranges, Range{Start: start, End: start + len(text)})
					after += idx + len(text)
				}
			case spanLink:
				// sub = [full, linkText, url]; mask only the URL.
				if len(sub) >= 3 {
					urlPart := "(" + sub[2] + ")"
					if idx := strings.Index(l[after:], urlPart); idx >= 0 {
						start := lineOff + after + idx
						ranges = append(ranges, Range{Start: start, End: start + len(urlPart)})
					}
					if idx := strings.Index(l[after:], text); idx >= 0 {
						after += idx + len(text)
					}
				}
			}
		})
		off = lineOff + len(l) + 1
	}

	return skipMask{ranges: ranges}
}
