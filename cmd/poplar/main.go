package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := newRootCmd()
	cmd.AddCommand(newThemesCmd())
	cmd.AddCommand(newConfigCmd())
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
