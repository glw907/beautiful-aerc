// SPDX-License-Identifier: MIT

package ui

import "github.com/glw907/poplar/internal/ui/uicore"

// DimANSI returns s with SGR faint (ESC[2m) injected throughout.
// The canonical implementation lives in internal/ui/uicore; this wrapper
// keeps internal/ui consumers source-compatible during the subpackage migration.
func DimANSI(s string) string {
	return uicore.DimANSI(s)
}
