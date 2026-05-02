// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/glw907/poplar/internal/config"
	"github.com/glw907/poplar/internal/term"
	"github.com/spf13/cobra"
	xterm "golang.org/x/term"
)

// newDiagnoseCmd returns the diagnose subcommand.
func newDiagnoseCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "diagnose",
		Short:        "Print terminal + font detection state and resolved icon mode",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagnose()
		},
	}
}

func runDiagnose() error {
	fmt.Println("Terminal:")
	fmt.Printf("  TERM           = %s\n", os.Getenv("TERM"))
	fmt.Printf("  COLORTERM      = %s\n", os.Getenv("COLORTERM"))
	fmt.Printf("  is_terminal    = %v\n", xterm.IsTerminal(int(os.Stdout.Fd())))
	fmt.Println()

	fmt.Println("Fonts:")
	hasNF := term.HasNerdFont()
	fmt.Printf("  has_nerd_font  = %v\n", hasNF)
	fmt.Printf("  source         = sysfont\n")
	fmt.Println()

	fmt.Println("Probe:")
	start := time.Now()
	w := term.MeasureSPUACells()
	dur := time.Since(start)
	fmt.Printf("  cell_width     = %d  (0 = probe failed)\n", w)
	fmt.Printf("  duration       = %s\n", dur.Round(100*time.Microsecond))
	fmt.Println()

	configPath, _, _ := config.Resolve("")
	cfgIcons := "auto"
	if configPath != "" {
		if uiCfg, err := config.LoadUI(configPath); err == nil {
			cfgIcons = uiCfg.Icons
		}
	}
	mode, cellWidth := term.Resolve(cfgIcons, hasNF, w)

	iconSet := "SimpleIcons"
	if mode == term.IconModeFancy {
		iconSet = "FancyIcons"
	}

	fmt.Println("Resolved:")
	fmt.Printf("  config.icons   = %s\n", cfgIcons)
	fmt.Printf("  effective_mode = %s\n", mode)
	fmt.Printf("  spua_cell_w    = %d\n", cellWidth)
	fmt.Printf("  icon_set       = %s\n", iconSet)

	return nil
}
