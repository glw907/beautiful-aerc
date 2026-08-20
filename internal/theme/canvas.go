package theme

import (
	"image"

	"charm.land/lipgloss/v2"
)

// Canvas is the render seam's cell-buffer compositor (ui.Render): a
// Paint call composes one already fully-rendered, already-sized
// block at rect's origin, in call order, so a later call draws over
// an earlier one (the mechanism LayoutMode's own Main doc comment
// describes: a backdrop painted first, a named pane overlaid on
// top). It is the one place outside this package's other files that
// reaches for a raw lipgloss.Canvas/Layer/Compositor call, so
// internal/ui's own compositing logic, which is pure LayoutMode
// geometry, never crosses UX-3's styling boundary.
type Canvas struct {
	canvas *lipgloss.Canvas
}

// NewCanvas returns an empty width×height Canvas.
func NewCanvas(width, height int) *Canvas {
	return &Canvas{canvas: lipgloss.NewCanvas(width, height)}
}

// Paint composes content, whose own rendered width and height must
// already equal rect's, at rect's origin. Paint trusts the caller on
// this: content wider or taller than rect is never clipped to rect,
// only to the canvas's own edge, so it silently overlaps whatever
// neighboring pane a later Paint call has not yet overlaid it with.
func (c *Canvas) Paint(rect image.Rectangle, content string) {
	c.canvas.Compose(lipgloss.NewCompositor(lipgloss.NewLayer(content).X(rect.Min.X).Y(rect.Min.Y)))
}

// Render returns the composed frame as one styled string.
func (c *Canvas) Render() string {
	return c.canvas.Render()
}
