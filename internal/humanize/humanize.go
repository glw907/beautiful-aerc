// SPDX-License-Identifier: MIT

package humanize

import "fmt"

// Bytes formats n as a 1-decimal 1024-based size string. Used by the
// cache CLI and the attachment chip row.
func Bytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	const k = 1024.0
	v := float64(n) / k
	if v < 1024 {
		return fmt.Sprintf("%.1f KB", v)
	}
	v /= k
	if v < 1024 {
		return fmt.Sprintf("%.1f MB", v)
	}
	v /= k
	if v < 1024 {
		return fmt.Sprintf("%.1f GB", v)
	}
	v /= k
	return fmt.Sprintf("%.1f TB", v)
}
