// SPDX-License-Identifier: MIT

package ui

import "github.com/glw907/poplar/internal/ui/uicore"

// PlaceOverlay composites fg (the overlay) at position (x, y) over bg.
// The canonical implementation lives in internal/ui/uicore; this alias
// keeps internal/ui consumers source-compatible during the subpackage migration.
func PlaceOverlay(x, y int, fg, bg string) string {
	return uicore.PlaceOverlay(x, y, fg, bg)
}
