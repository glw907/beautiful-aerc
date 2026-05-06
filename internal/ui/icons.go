// SPDX-License-Identifier: MIT

package ui

import "github.com/glw907/poplar/internal/ui/uicore"

// IconSet is the per-mode iconography vocabulary for poplar's UI
// surfaces. Defined in uicore for shared access across subpackages.
// SimpleIcons uses Unicode Narrow-class codepoints (lipgloss.Width == 1).
// FancyIcons uses Nerd Font SPUA-A glyphs (U+F0000–U+FFFFD).
//
// Add a field here whenever a new render surface needs an icon. Both
// tables must be updated together. Tests in icons_test.go enforce the
// class invariants.
type IconSet = uicore.IconSet

// SimpleIcons is the Unicode-Narrow iconography used when no Nerd Font
// is detected (or icons = "simple"). Every rune must be East Asian
// Width Na or N. Verified by TestSimpleIcons_AllNarrow.
var SimpleIcons = uicore.SimpleIcons

// FancyIcons is the Nerd Font SPUA-A iconography used when a Nerd Font
// is detected (or icons = "fancy"). Verified by TestFancyIcons_AllSPUA.
var FancyIcons = uicore.FancyIcons
