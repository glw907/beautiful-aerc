package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/header"
)

func newExtractCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "extract <from|subject|to>",
		Short: "Extract a field from message headers on stdin",
		Example: `  # Extract sender address
  cat message.eml | fastmail-cli rules extract from

  # Extract all recipients (To + Cc)
  cat message.eml | fastmail-cli rules extract to`,
		Args: cobra.ExactArgs(1),
		RunE: runExtract,
	}
}

func runExtract(cmd *cobra.Command, args []string) error {
	field := args[0]

	switch field {
	case "from":
		value := header.ExtractFrom(os.Stdin)
		if value == "" {
			return fmt.Errorf("no from header found")
		}
		fmt.Print(value)
	case "subject":
		value := header.ExtractSubject(os.Stdin)
		if value == "" {
			return fmt.Errorf("no subject header found")
		}
		fmt.Print(value)
	case "to":
		addrs := header.ExtractTo(os.Stdin)
		if len(addrs) == 0 {
			return fmt.Errorf("no to/cc headers found")
		}
		for _, a := range addrs {
			fmt.Println(a)
		}
	default:
		return fmt.Errorf("unknown field: %s (use 'from', 'subject', or 'to')", field)
	}

	return nil
}
