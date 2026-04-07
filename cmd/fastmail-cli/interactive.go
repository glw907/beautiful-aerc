package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glw907/beautiful-aerc/internal/header"
	"github.com/glw907/beautiful-aerc/internal/jmap"
	"github.com/glw907/beautiful-aerc/internal/rules"
)

type interactiveFlags struct {
	rulesFile string
}

func newInteractiveCmd() *cobra.Command {
	var f interactiveFlags

	cmd := &cobra.Command{
		Use:   "interactive <from|subject|to>",
		Short: "Full interactive filter creation flow",
		Long:  "Reads a piped message from stdin, extracts headers, then prompts via /dev/tty for filter creation.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractive(args[0], resolveRulesFile(f.rulesFile))
		},
	}

	cmd.Flags().StringVar(&f.rulesFile, "rules-file", "", "Path to rules file (default: $AERC_RULES_FILE or ~/.config/aerc/mailrules.json)")

	return cmd
}

func runInteractive(field, rulesFilePath string) error {
	if field != "from" && field != "subject" && field != "to" {
		return fmt.Errorf("unknown field: %s (use 'from', 'subject', or 'to')", field)
	}

	// Read piped message from stdin
	msgData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading message: %w", err)
	}

	// Open tty BEFORE the switch - "to" case needs scanner for pick-list
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("opening terminal: %w", err)
	}
	defer tty.Close()
	scanner := bufio.NewScanner(tty)

	reader := bytes.NewReader(msgData)
	var value string
	switch field {
	case "from":
		value = header.ExtractFrom(reader)
		if value == "" {
			return fmt.Errorf("no from header found in message")
		}
	case "subject":
		value = header.ExtractSubject(reader)
		if value == "" {
			return fmt.Errorf("no subject header found in message")
		}
	case "to":
		addrs := header.ExtractTo(reader)
		if len(addrs) == 0 {
			return fmt.Errorf("no to/cc headers found in message")
		}

		if len(addrs) == 1 {
			value = addrs[0]
		} else {
			fmt.Fprintf(os.Stderr, "\nRecipients found:\n")
			for i, a := range addrs {
				fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, a)
			}
			fmt.Fprint(os.Stderr, "\nRecipient [1]: ")
			scanner.Scan()
			pick := strings.TrimSpace(scanner.Text())

			pickIdx := 0
			if pick != "" {
				n, err := strconv.Atoi(pick)
				if err != nil || n < 1 || n > len(addrs) {
					return fmt.Errorf("invalid choice: %s", pick)
				}
				pickIdx = n - 1
			}
			value = addrs[pickIdx]
		}
	}

	// For to field, skip the search value prompt - the pick-list already selected it
	if field != "to" {
		fmt.Fprintf(os.Stderr, "\nFilter by %s\n", field)
		fmt.Fprintf(os.Stderr, "Search [%s]: ", value)
		scanner.Scan()
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			value = input
		}
	}

	search := field + ":" + value

	// Fetch folders
	fmt.Fprintln(os.Stderr, "\nFetching folders...")
	s, err := newJMAPSession()
	if err != nil {
		return err
	}

	folders, err := jmap.ListFolders(s)
	if err != nil {
		return err
	}

	if len(folders) == 0 {
		return fmt.Errorf("no destination folders available")
	}

	// Display folder menu
	fmt.Fprintln(os.Stderr)
	for i, f := range folders {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, f.Name)
	}
	fmt.Fprint(os.Stderr, "\nFolder [1]: ")
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	idx := 0
	if choice != "" {
		n, err := strconv.Atoi(choice)
		if err != nil || n < 1 || n > len(folders) {
			return fmt.Errorf("invalid choice: %s", choice)
		}
		idx = n - 1
	}
	folder := folders[idx].Name

	if err := rules.Add(rulesFilePath, search, folder); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nRule added: %s -> %s\n", search, folder)

	fmt.Fprintln(os.Stderr, "Counting matching messages...")
	ids, err := jmap.QueryInbox(s, search)
	if err != nil {
		return fmt.Errorf("counting messages: %w", err)
	}

	if len(ids) == 0 {
		fmt.Fprintln(os.Stderr, "No matching messages in Inbox.")
		fmt.Fprint(os.Stderr, "\nPress Enter to close...")
		scanner.Scan()
		return nil
	}

	// Prompt to sweep
	fmt.Fprintf(os.Stderr, "Move %d messages to %s? [y/N]: ", len(ids), folder)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

	if answer != "y" && answer != "yes" {
		fmt.Fprintln(os.Stderr, "Skipped.")
		fmt.Fprint(os.Stderr, "\nPress Enter to close...")
		scanner.Scan()
		return nil
	}

	// Find mailbox IDs for the move
	inboxID, err := jmap.FindMailbox(s, "Inbox")
	if err != nil {
		return fmt.Errorf("finding Inbox: %w", err)
	}
	destID, err := jmap.FindMailbox(s, folder)
	if err != nil {
		return fmt.Errorf("finding folder %q: %w", folder, err)
	}

	if err := jmap.MoveEmails(s, ids, inboxID, destID); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Moved %d messages to %s\n", len(ids), folder)
	fmt.Fprint(os.Stderr, "\nPress Enter to close...")
	scanner.Scan()
	return nil
}
