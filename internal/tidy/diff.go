package tidy

// ByteRange is a half-open byte span [Start, End) in newText where
// the rune sequence diverges from oldText. Ranges are sorted by Start
// and never overlap.
type ByteRange struct{ Start, End int }

// DiffRanges returns the byte ranges in newText whose runes were not
// matched against oldText by a longest common subsequence walk.
// Adjacent change positions coalesce into one range. Pure deletions
// (runes in oldText that disappear from newText) produce no range —
// there is no place in newText to underline.
//
// Runes that are technically kept by the LCS but sit inside a replace
// block (insertion immediately before, deletion immediately after) are
// included in the range so that transpositions highlight the full
// disturbed window rather than one side of the swap.
func DiffRanges(oldText, newText string) []ByteRange {
	if newText == "" {
		return nil
	}
	if oldText == newText {
		return nil
	}

	oldRunes := []rune(oldText)
	newRunes := []rune(newText)
	n, m := len(oldRunes), len(newRunes)

	// Build LCS table.
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if oldRunes[i-1] == newRunes[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else if lcs[i-1][j] >= lcs[i][j-1] {
				lcs[i][j] = lcs[i-1][j]
			} else {
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}

	// Traceback: record which new-rune indices are matched and to which old index.
	type matchPair struct {
		matched bool
		oldIdx  int
	}
	matches := make([]matchPair, m)
	for k := range matches {
		matches[k].oldIdx = -1
	}
	i, j := n, m
	for i > 0 && j > 0 {
		if oldRunes[i-1] == newRunes[j-1] {
			matches[j-1] = matchPair{true, i - 1}
			i--
			j--
		} else if lcs[i-1][j] >= lcs[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Mark new positions that are unmatched (inserted or substituted).
	marked := make([]bool, m)
	for k, mp := range matches {
		if !mp.matched {
			marked[k] = true
		}
	}

	// Precompute the old index of the next kept new rune at each position.
	// Used to detect deletions immediately following a keep.
	nextKeepOld := make([]int, m)
	lastOld := -1
	for k := m - 1; k >= 0; k-- {
		if matches[k].matched {
			lastOld = matches[k].oldIdx
		}
		nextKeepOld[k] = lastOld
	}

	// Expand: a kept new rune belongs to a replace block when an insertion
	// falls immediately before it (matches[k-1].matched == false) and
	// deletions follow before the next kept rune (gap in old indices).
	// Including these runes ensures transpositions highlight the full window.
	for k, mp := range matches {
		if !mp.matched || k == 0 || matches[k-1].matched {
			continue
		}
		var nextOld int
		if k+1 < m {
			nextOld = nextKeepOld[k+1]
		} else {
			nextOld = -1
		}
		if nextOld > mp.oldIdx+1 {
			marked[k] = true
		}
	}

	var out []ByteRange
	bytePos := 0
	runStart, inRun := -1, false
	for k, r := range newRunes {
		size := utf8RuneLen(r)
		if marked[k] {
			if !inRun {
				runStart = bytePos
				inRun = true
			}
		} else if inRun {
			out = append(out, ByteRange{runStart, bytePos})
			inRun = false
		}
		bytePos += size
	}
	if inRun {
		out = append(out, ByteRange{runStart, bytePos})
	}
	return out
}

func utf8RuneLen(r rune) int {
	switch {
	case r < 0x80:
		return 1
	case r < 0x800:
		return 2
	case r < 0x10000:
		return 3
	default:
		return 4
	}
}
