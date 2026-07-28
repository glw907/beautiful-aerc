// Package main is the perfspike tool. It harvests a real JMAP mail archive
// into SQLite, amplifies it to the QA-5 scale envelope, then measures
// performance numbers that become the build's acceptance gates.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "perfspike",
		Short:        "Phase 4 performance measurement spike",
		SilenceUsage: true,
	}
	cmd.AddCommand(newHarvestCmd())
	cmd.AddCommand(newAmplifyCmd())
	cmd.AddCommand(newBenchCmd())
	cmd.AddCommand(newStartupProbeCmd())
	return cmd
}
