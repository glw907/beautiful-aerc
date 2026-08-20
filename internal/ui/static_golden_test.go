package ui_test

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"

	"github.com/glw907/poplar/internal/theme"
	"github.com/glw907/poplar/internal/ui/fixtures"
)

// TestRender_WideRung is a static golden over a case the gallery
// sweep does not cover (amendment B: no separate golden duplicates a
// gallery-covered case): the wide rung's PaneSplit compositing path,
// where the list/reader boundary is a blank gutter at true color and
// a drawn divider under ANSI-16's degrade rule. golden.RequireEqual
// owns the persisted file (testdata/TestRender_WideRung/*.golden);
// run with -update to accept a deliberate change.
func TestRender_WideRung(t *testing.T) {
	size := gallerySize{150, 26}

	t.Run("truecolor_dark_blank_gutter", func(t *testing.T) {
		th := theme.New(true, theme.ProfileTrueColor)
		golden.RequireEqual(t, []byte(escapeGalleryOutput(galleryRender(fixtures.Mail, size, th))))
	})

	t.Run("ansi16_drawn_divider", func(t *testing.T) {
		th := theme.New(true, theme.ProfileANSI16)
		golden.RequireEqual(t, []byte(escapeGalleryOutput(galleryRender(fixtures.Mail, size, th))))
	})
}
