package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"github.com/spf13/cobra"
)

const (
	sessionEndpoint   = "https://api.fastmail.com/jmap/session"
	queryPageSize     = 500
	fetchBatchSize    = 200
	maxBodyValueBytes = 65536

	stateKeyPosition = "harvest_position"
)

type harvestFlags struct {
	reset bool
}

func newHarvestCmd() *cobra.Command {
	var f harvestFlags
	cmd := &cobra.Command{
		Use:          "harvest",
		Short:        "Fetch real mail archive into the local SQLite store",
		SilenceUsage: true,
		RunE:         func(_ *cobra.Command, _ []string) error { return runHarvest(&f) },
	}
	cmd.Flags().BoolVar(&f.reset, "reset", false, "discard saved position and re-harvest from the start")
	return cmd
}

func runHarvest(f *harvestFlags) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	token := os.Getenv("FASTMAIL_API_TOKEN")
	if token == "" {
		return fmt.Errorf("FASTMAIL_API_TOKEN not set; source ~/.local/secrets first")
	}

	cli := &jmap.Client{SessionEndpoint: sessionEndpoint}
	cli.WithAccessToken(token)
	if err := cli.Authenticate(); err != nil {
		return fmt.Errorf("JMAP authenticate: %w", err)
	}

	accountID, err := findMailAccount(cli.Session.Accounts)
	if err != nil {
		return err
	}
	slog.Info("authenticated", "account", accountID)

	mboxNames, err := fetchMailboxNames(cli, accountID)
	if err != nil {
		return fmt.Errorf("list mailboxes: %w", err)
	}
	slog.Info("mailboxes", "count", len(mboxNames))

	w, r, err := openDB()
	if err != nil {
		return err
	}
	defer w.Close()
	defer r.Close()

	if err := initSchema(w); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	startPos := int64(0)
	if !f.reset {
		if v, ok, err := getState(r, stateKeyPosition); err != nil {
			return err
		} else if ok {
			startPos, _ = strconv.ParseInt(v, 10, 64)
			slog.Info("resuming harvest", "position", startPos)
		}
	}

	var total int64
	position := startPos

	for {
		ids, err := queryPage(cli, accountID, position)
		if err != nil {
			return fmt.Errorf("Email/query at %d: %w", position, err)
		}
		if len(ids) == 0 {
			break
		}

		emails, err := fetchEmailBatch(cli, accountID, ids)
		if err != nil {
			return fmt.Errorf("Email/get: %w", err)
		}

		msgs := make([]message, 0, len(emails))
		for _, e := range emails {
			msgs = append(msgs, emailToMessage(e, mboxNames))
		}

		if err := insertMessages(w, msgs); err != nil {
			return fmt.Errorf("insert batch at %d: %w", position, err)
		}

		position += int64(len(ids))
		total += int64(len(emails))

		if err := setState(w, stateKeyPosition, strconv.FormatInt(position, 10)); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "\rharvested %d messages (position %d)", total, position)

		if len(ids) < queryPageSize {
			break
		}
	}

	fmt.Fprintln(os.Stderr)
	slog.Info("harvest complete", "fetched", total, "total_position", position)
	return nil
}

// findMailAccount returns the account ID whose capabilities include the JMAP
// mail URI. Fastmail exposes two accounts; only one carries mail capability.
func findMailAccount(accounts map[jmap.ID]jmap.Account) (jmap.ID, error) {
	for id, acct := range accounts {
		if _, ok := acct.Capabilities[jmapmail.URI]; ok {
			return id, nil
		}
	}
	return "", fmt.Errorf("no account with mail capability in JMAP session")
}

func fetchMailboxNames(cli *jmap.Client, accountID jmap.ID) (map[jmap.ID]string, error) {
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{Account: accountID})
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Mailbox/get: %w", err)
	}
	names := make(map[jmap.ID]string)
	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*mailbox.GetResponse)
		if !ok {
			continue
		}
		for _, m := range gr.List {
			names[m.ID] = m.Name
		}
	}
	return names, nil
}

func queryPage(cli *jmap.Client, accountID jmap.ID, position int64) ([]jmap.ID, error) {
	req := &jmap.Request{}
	req.Invoke(&email.Query{
		Account:  accountID,
		Sort:     []*email.SortComparator{{Property: "receivedAt", IsAscending: false}},
		Position: position,
		Limit:    queryPageSize,
	})
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Email/query: %w", err)
	}
	for _, inv := range resp.Responses {
		qr, ok := inv.Args.(*email.QueryResponse)
		if !ok {
			continue
		}
		return qr.IDs, nil
	}
	return nil, nil
}

var fetchProperties = []string{
	"id", "blobId", "threadId", "mailboxIds", "keywords",
	"from", "to", "cc",
	"subject", "receivedAt", "size", "hasAttachment",
	"textBody", "htmlBody", "bodyValues",
}

func fetchEmailBatch(cli *jmap.Client, accountID jmap.ID, ids []jmap.ID) ([]*email.Email, error) {
	var all []*email.Email
	for chunk := range slices.Chunk(ids, fetchBatchSize) {
		req := &jmap.Request{}
		req.Invoke(&email.Get{
			Account:             accountID,
			IDs:                 chunk,
			Properties:          fetchProperties,
			FetchTextBodyValues: true,
			FetchHTMLBodyValues: true,
			MaxBodyValueBytes:   maxBodyValueBytes,
		})
		resp, err := cli.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Email/get chunk: %w", err)
		}
		for _, inv := range resp.Responses {
			gr, ok := inv.Args.(*email.GetResponse)
			if !ok {
				continue
			}
			all = append(all, gr.List...)
		}
	}
	return all, nil
}

func emailToMessage(e *email.Email, mboxNames map[jmap.ID]string) message {
	mbox := "Unknown"
	for id := range e.MailboxIDs {
		if name, ok := mboxNames[id]; ok {
			mbox = name
			break
		}
	}

	var rcvd int64
	if e.ReceivedAt != nil {
		rcvd = e.ReceivedAt.Unix()
	}

	mboxIDs := make([]string, 0, len(e.MailboxIDs))
	for id := range e.MailboxIDs {
		mboxIDs = append(mboxIDs, string(id))
	}

	to := make([]string, 0, len(e.To))
	for _, a := range e.To {
		to = append(to, formatAddr(a))
	}
	cc := make([]string, 0, len(e.CC))
	for _, a := range e.CC {
		cc = append(cc, formatAddr(a))
	}

	type msgJSON struct {
		ServerID      string   `json:"server_id"`
		BlobID        string   `json:"blob_id"`
		ThreadID      string   `json:"thread_id"`
		MailboxIDs    []string `json:"mailbox_ids"`
		From          string   `json:"from"`
		To            []string `json:"to"`
		CC            []string `json:"cc"`
		Subject       string   `json:"subject"`
		ReceivedAt    int64    `json:"received_at"`
		Size          uint64   `json:"size"`
		HasAttachment bool     `json:"has_attachment"`
		Flags         int      `json:"flags"`
	}

	from := ""
	if len(e.From) > 0 {
		from = formatAddr(e.From[0])
	}

	flags := keywordsToFlags(e.Keywords)
	data, _ := json.Marshal(msgJSON{
		ServerID:      string(e.ID),
		BlobID:        string(e.BlobID),
		ThreadID:      string(e.ThreadID),
		MailboxIDs:    mboxIDs,
		From:          from,
		To:            to,
		CC:            cc,
		Subject:       e.Subject,
		ReceivedAt:    rcvd,
		Size:          e.Size,
		HasAttachment: e.HasAttachment,
		Flags:         flags,
	})

	return message{
		serverID:      string(e.ID),
		threadKey:     string(e.ThreadID),
		mailbox:       mbox,
		receivedAt:    rcvd,
		subject:       e.Subject,
		fromAddr:      from,
		flags:         flags,
		hasAttachment: e.HasAttachment,
		size:          int64(e.Size),
		body:          extractBody(e),
		data:          string(data),
	}
}

func extractBody(e *email.Email) string {
	for _, part := range e.TextBody {
		if bv, ok := e.BodyValues[part.PartID]; ok && bv.Value != "" {
			return bv.Value
		}
	}
	for _, part := range e.HTMLBody {
		if bv, ok := e.BodyValues[part.PartID]; ok && bv.Value != "" {
			return bv.Value
		}
	}
	return ""
}

func keywordsToFlags(kw map[string]bool) int {
	var flags int
	if kw["$seen"] {
		flags |= 1 << 0
	}
	if kw["$flagged"] {
		flags |= 1 << 1
	}
	if kw["$answered"] {
		flags |= 1 << 2
	}
	if kw["$draft"] {
		flags |= 1 << 3
	}
	return flags
}

func formatAddr(a *jmapmail.Address) string {
	if a == nil {
		return ""
	}
	if a.Name != "" {
		return a.Name + " <" + a.Email + ">"
	}
	return a.Email
}

func firstMailboxID(ids map[jmap.ID]bool) string {
	for id := range ids {
		return strings.TrimSpace(string(id))
	}
	return ""
}
