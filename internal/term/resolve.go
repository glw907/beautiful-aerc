// Package term detects terminal capabilities at poplar startup:
// Nerd Font installation discovery (HasNerdFont) and a DSR/CPR
// probe of an SPUA-A glyph's rendered cell width (MeasureSPUACells).
package term

// IconMode is the resolved iconography mode the UI should render.
type IconMode int

const (
	IconModeSimple IconMode = iota
	IconModeFancy
)

func (m IconMode) String() string {
	switch m {
	case IconModeFancy:
		return "fancy"
	default:
		return "simple"
	}
}

// Resolve maps the configured icons preference plus runtime detection
// to (mode, spuaCellWidth). cfg is the UIConfig.Icons literal
// ("auto", "simple", "fancy", or unknown which auto-resolves). probe
// is MeasureSPUACells's result: 1, 2, or 0 for failure. On fancy mode
// with probe failure, the legacy Mono Nerd Font assumption (width=2)
// stands. Simple mode pins width=1, where lipgloss.Width is canonical.
func Resolve(cfg string, hasNerdFont bool, probe int) (IconMode, int) {
	mode := IconModeSimple
	switch cfg {
	case "fancy":
		mode = IconModeFancy
	case "simple":
		mode = IconModeSimple
	default:
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
