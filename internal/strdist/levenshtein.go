// Package strdist provides string-distance primitives.
package strdist

import "math"

// Levenshtein returns the edit distance between a and b.
// When limit > 0, the function returns limit+1 as soon as the true
// distance is known to exceed limit; pass limit ≤ 0 to disable the cap.
func Levenshtein(a, b string, limit int) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	if limit <= 0 {
		limit = math.MaxInt
	}
	if max(la-lb, lb-la) > limit {
		return limit + 1
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range lb + 1 {
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
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
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
