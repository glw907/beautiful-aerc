// SPDX-License-Identifier: MIT

package term

// IconMode is the resolved iconography mode the UI should render.
type IconMode int

const (
	IconModeSimple IconMode = iota
	IconModeFancy
)

// String returns the display name of the icon mode.
func (m IconMode) String() string {
	switch m {
	case IconModeFancy:
		return "fancy"
	default:
		return "simple"
	}
}

// Resolve maps a config-icons value plus runtime detection results to
// the (mode, spuaCellWidth) pair the UI uses.
//
//	cfg          — UIConfig.Icons literal: "auto" | "simple" | "fancy".
//	               Unknown values are treated as "auto".
//	hasNerdFont  — result of HasNerdFont().
//	probe        — result of MeasureSPUACells(): 1, 2, or 0 (failed).
//
// Defaults on probe failure in fancy mode: spuaCellWidth=2 (Mono Nerd
// Font, the legacy assumption). In simple mode: spuaCellWidth=1
// (lipgloss.Width is canonical and the helper degenerates).
func Resolve(cfg string, hasNerdFont bool, probe int) (IconMode, int) {
	mode := IconModeSimple
	switch cfg {
	case "fancy":
		mode = IconModeFancy
	case "simple":
		mode = IconModeSimple
	default: // "auto" or unknown
		if hasNerdFont {
			mode = IconModeFancy
		}
	}
	if mode == IconModeSimple {
		return IconModeSimple, 1
	}
	switch probe {
	case 1, 2:
		return IconModeFancy, probe
	default:
		return IconModeFancy, 2
	}
}
