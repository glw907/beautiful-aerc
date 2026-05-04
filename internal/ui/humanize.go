// SPDX-License-Identifier: MIT

package ui

import "fmt"

// humanizeBytes formats a byte count for the attachment chip row
// and picker. Decimal one-place precision above 1 KB.
func humanizeBytes(n int64) string {
	const k = 1024.0
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/k)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
	}
}
