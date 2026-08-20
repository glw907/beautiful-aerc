// Package sub proves the subpackage evasion: a Screen-shaped type
// declared under internal/ui/... but outside the ui root itself must
// still be caught, since a runtime, in-package test never sees a
// subpackage's own init at all.
package sub

import "a/internal/ui"

// SubScreen is registered normally, the happy-path subpackage case.
type SubScreen struct{}

func (SubScreen) Init() int             { return 0 }
func (SubScreen) Update(int) (int, int) { return 0, 0 }
func (SubScreen) View() string          { return "" }
func (SubScreen) Entry() ui.ScreenEntry { return ui.ScreenEntry{} }

func init() { ui.Register[SubScreen](ui.ScreenEntry{}) }
