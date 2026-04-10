package main

import (
	"github.com/spf13/cobra"

	// Import forked workers for init() side effects (handler registration).
	_ "github.com/glw907/beautiful-aerc/internal/aercfork/worker"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "poplar",
		Short:        "A bubbletea-based terminal email client",
		SilenceUsage: true,
	}
	return cmd
}
