package theme

// Spacing roles: cells and rows in the cairn gap grammar (design
// language section 7; decision 3 adds PadBand and PadCard). A
// screen reaches spacing only through these, never a markup-level
// literal.
const (
	GapLabel   = 1 // label to its value
	GapControl = 2 // control to neighbor
	Gutter     = 1 // marker column to text
	PadPane    = 1 // pane edge to content
	PadModalX  = 2 // modal horizontal inset
	PadModalY  = 1 // modal vertical inset
	GapSection = 1 // row between sections
	PadBand    = 2 // chrome band inset
	PadCard    = 2 // card inset
	GapPane    = 2 // pane to neighboring pane
	GapHint    = 3 // footer hint to its neighbor hint
	GapPin     = 6 // footer's last shown hint to the pinned help hint
)
