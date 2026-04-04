package main

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/glw907/beautiful-aerc/internal/filter"
	"github.com/glw907/beautiful-aerc/internal/palette"
	"github.com/spf13/cobra"
)

func newHeadersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headers",
		Short: "Format and colorize email headers",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := loadPalette()
			if err != nil {
				return err
			}
			cols := termCols()
			return filter.Headers(os.Stdin, os.Stdout, colorsFromPalette(p), cols)
		},
	}
	return cmd
}

// loadPalette finds and loads the palette file relative to the binary location.
func loadPalette() (*palette.Palette, error) {
	binPath, _ := os.Executable()
	genDir := ""
	if binPath != "" {
		// Resolve symlinks, then navigate: .local/bin/ -> .config/aerc/generated
		resolved, err := filepath.EvalSymlinks(binPath)
		if err == nil {
			binPath = resolved
		}
		binDir := filepath.Dir(binPath)
		genDir = filepath.Join(binDir, "..", "..", ".config", "aerc", "generated")
	}
	path, err := palette.FindPath(genDir)
	if err != nil {
		return nil, err
	}
	return palette.Load(path)
}

// colorsFromPalette builds a ColorSet from palette entries.
func colorsFromPalette(p *palette.Palette) *filter.ColorSet {
	hdrKey := p.Get("ACCENT_PRIMARY")
	fgBase := p.Get("FG_BASE")
	fgDim := p.Get("FG_DIM")

	ansiKey, _ := palette.HexToANSI(hdrKey)
	ansiFG, _ := palette.HexToANSI(fgBase)
	ansiDim, _ := palette.HexToANSI(fgDim)

	cs := &filter.ColorSet{Reset: p.Reset()}
	if ansiKey != "" {
		cs.HdrKey = "\033[1;" + ansiKey + "m"
	}
	if ansiFG != "" {
		cs.HdrFG = "\033[" + ansiFG + "m"
	}
	if ansiDim != "" {
		cs.HdrDim = "\033[" + ansiDim + "m"
	}
	return cs
}

// termCols returns the terminal column count from AERC_COLUMNS or a default of 80.
func termCols() int {
	if s := os.Getenv("AERC_COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 80
}
