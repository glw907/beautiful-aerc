// SPDX-License-Identifier: MIT

package config

// suggestProvider returns the closest known-provider name to s
// within Levenshtein distance 2, or "" if no close match exists.
// Includes the imap/jmap fallbacks in the search space.
func suggestProvider(s string) string {
	if s == "" {
		return ""
	}
	candidates := []string{"imap", "jmap"}
	for k := range Providers {
		candidates = append(candidates, k)
	}
	bestName := ""
	bestDist := 3
	for _, c := range candidates {
		d := levenshtein(s, c)
		if d < bestDist {
			bestDist = d
			bestName = c
		}
	}
	if bestDist > 2 {
		return ""
	}
	return bestName
}

func levenshtein(a, b string) int {
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
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
