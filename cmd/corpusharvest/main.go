// Package main is the corpusharvest tool. It connects to the Fastmail
// account via JMAP and downloads a classified sample of messages into
// corpus/raw/ for use in rendering experiments.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	jmapmail "git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/glw907/poplar/internal/config"
)

const (
	maxPerClass  = 25
	minPerClass  = 15
	maxPerSender = 5
	fetchChunk   = 250
	queryLimit   = 500
)

// candidate is one message seen during the sweep.
type candidate struct {
	id         string
	blobID     string
	from       string
	senderKey  string
	subject    string
	receivedAt time.Time
	folder     string
	class      string
	inWindow   bool // within the 12-month window
}

// corpusEntry is one record in corpus/manifest.json.
type corpusEntry struct {
	ID      string `json:"id"`
	Class   string `json:"class"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Folder  string `json:"folder"`
	Path    string `json:"path"`
}

// classStats records the per-class result after selection.
type classStats struct {
	Count  int    `json:"count"`
	Window string `json:"window"`
}

// harvestManifest is the top-level corpus/manifest.json structure.
type harvestManifest struct {
	HarvestDate string                `json:"harvest_date"`
	Stats       map[string]classStats `json:"stats"`
	Entries     []corpusEntry         `json:"entries"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("harvest", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath string
		corpusDir  string
		dryRun     bool
	)
	flag.StringVar(&configPath, "config", "", "path to poplar config (default: ~/.config/poplar/config.toml)")
	flag.StringVar(&corpusDir, "corpus", "corpus", "output directory for corpus files")
	flag.BoolVar(&dryRun, "dry-run", false, "classify and report without downloading blobs")
	flag.Parse()

	accts, _, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if len(accts) == 0 {
		return fmt.Errorf("config: no accounts")
	}
	acct := accts[0]

	token, err := acct.ResolvePassword()
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}
	if token == "" {
		return fmt.Errorf("API token is empty; source ~/.local/secrets before running")
	}

	cli := &jmap.Client{SessionEndpoint: acct.Source}
	cli.WithAccessToken(token)
	if err := cli.Authenticate(); err != nil {
		return fmt.Errorf("JMAP authenticate: %w", err)
	}

	accountID := cli.Session.PrimaryAccounts[jmapmail.URI]
	slog.Info("connected", "account", accountID)

	mboxes, err := listMailboxes(cli, accountID)
	if err != nil {
		return fmt.Errorf("list mailboxes: %w", err)
	}
	slog.Info("mailboxes", "count", len(mboxes))

	cutoff := time.Now().AddDate(-1, 0, 0).UTC().Truncate(24 * time.Hour)
	slog.Info("pass 1: 12-month window", "after", cutoff.Format("2006-01-02"))

	seen := make(map[string]bool)
	var pass1 []candidate
	for _, mbox := range mboxes {
		batch, err := queryMailboxCandidates(cli, accountID, mbox, &cutoff, &cutoff, seen)
		if err != nil {
			slog.Warn("mailbox query error", "mailbox", mbox.Name, "err", err)
			continue
		}
		slog.Info("mailbox scanned", "name", mbox.Name, "candidates", len(batch))
		pass1 = append(pass1, batch...)
	}

	byClass := groupByClass(pass1)

	// Select from pass 1 pool and identify actual shortfalls after sender cap.
	pool := make(map[string][]candidate)
	for k, v := range byClass {
		sorted := make([]candidate, len(v))
		copy(sorted, v)
		slices.SortFunc(sorted, func(a, b candidate) int {
			return b.receivedAt.Compare(a.receivedAt)
		})
		pool[k] = sorted
	}

	sel1 := make(map[string][]candidate)
	for _, cls := range allClasses {
		sel1[cls] = selectCandidates(pool[cls], maxPerClass, maxPerSender)
	}

	var shortfallClasses []string
	for _, cls := range allClasses {
		if len(sel1[cls]) < minPerClass {
			shortfallClasses = append(shortfallClasses, cls)
		}
	}

	if len(shortfallClasses) > 0 {
		slog.Info("pass 2: widening window for shortfalls", "classes", shortfallClasses)
		for _, mbox := range mboxes {
			// Query all-time but still track inWindow by the original cutoff.
			batch, err := queryMailboxCandidates(cli, accountID, mbox, nil, &cutoff, seen)
			if err != nil {
				slog.Warn("all-time query error", "mailbox", mbox.Name, "err", err)
				continue
			}
			for _, c := range batch {
				if slices.Contains(shortfallClasses, c.class) {
					pool[c.class] = append(pool[c.class], c)
				}
			}
		}
		// Re-sort and re-select for shortfall classes.
		for _, cls := range shortfallClasses {
			slices.SortFunc(pool[cls], func(a, b candidate) int {
				return b.receivedAt.Compare(a.receivedAt)
			})
			sel1[cls] = selectCandidates(pool[cls], maxPerClass, maxPerSender)
		}
	}

	windows := make(map[string]string)
	var selected []candidate
	for _, cls := range allClasses {
		sel := sel1[cls]
		selected = append(selected, sel...)

		count := len(sel)
		inWindow := 0
		for _, c := range sel {
			if c.inWindow {
				inWindow++
			}
		}
		switch {
		case count >= minPerClass && inWindow == count:
			windows[cls] = "12mo"
		case count >= minPerClass:
			windows[cls] = "all-time"
		default:
			windows[cls] = fmt.Sprintf("partial-%d", count)
		}

		shortfall := ""
		if count < minPerClass {
			shortfall = fmt.Sprintf(" SHORTFALL: %d < %d", count, minPerClass)
		}
		fmt.Printf("%-16s  %3d  window=%-12s%s\n", cls, count, windows[cls], shortfall)
	}

	if dryRun {
		slog.Info("dry-run: skipping download")
		return nil
	}

	for _, cls := range allClasses {
		dir := filepath.Join(corpusDir, "raw", cls)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	var entries []corpusEntry
	for _, c := range selected {
		data, err := downloadBlob(cli, accountID, c.blobID)
		if err != nil {
			slog.Warn("blob download error", "id", c.id, "err", err)
			continue
		}
		fsID := sanitizeID(c.id)
		relPath := filepath.Join("raw", c.class, fsID+".eml")
		absPath := filepath.Join(corpusDir, relPath)
		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", absPath, err)
		}
		entries = append(entries, corpusEntry{
			ID:      c.id,
			Class:   c.class,
			From:    c.from,
			Subject: c.subject,
			Date:    c.receivedAt.Format(time.RFC3339),
			Folder:  c.folder,
			Path:    relPath,
		})
	}

	stats := make(map[string]classStats, len(allClasses))
	for _, cls := range allClasses {
		count := 0
		for _, e := range entries {
			if e.Class == cls {
				count++
			}
		}
		stats[cls] = classStats{Count: count, Window: windows[cls]}
	}

	m := harvestManifest{
		HarvestDate: time.Now().UTC().Format(time.RFC3339),
		Stats:       stats,
		Entries:     entries,
	}
	mdata, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	mpath := filepath.Join(corpusDir, "manifest.json")
	if err := os.WriteFile(mpath, mdata, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	slog.Info("harvest complete", "messages", len(entries), "manifest", mpath)
	return nil
}

func listMailboxes(cli *jmap.Client, accountID jmap.ID) ([]*mailbox.Mailbox, error) {
	req := &jmap.Request{}
	req.Invoke(&mailbox.Get{Account: accountID})
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Mailbox/get: %w", err)
	}
	var out []*mailbox.Mailbox
	for _, inv := range resp.Responses {
		gr, ok := inv.Args.(*mailbox.GetResponse)
		if !ok {
			continue
		}
		for _, m := range gr.List {
			if m.Role == mailbox.RoleSent || m.Role == mailbox.RoleDrafts {
				continue
			}
			out = append(out, m)
		}
	}
	return out, nil
}

// classificationProperties is the Email/get property set for header-based
// classification. The "headers" property returns all raw RFC 5322 headers.
var classificationProperties = []string{
	"id", "blobId", "from", "subject", "receivedAt", "headers",
}

// queryMailboxCandidates queries a mailbox for candidate messages. after
// controls the JMAP date filter (nil = all-time). windowCutoff determines
// whether a message is marked as inWindow regardless of the query filter.
func queryMailboxCandidates(cli *jmap.Client, accountID jmap.ID, mbox *mailbox.Mailbox, after *time.Time, windowCutoff *time.Time, seen map[string]bool) ([]candidate, error) {
	ids, err := queryMailboxIDs(cli, accountID, mbox.ID, after)
	if err != nil {
		return nil, err
	}

	var newIDs []jmap.ID
	for _, id := range ids {
		s := string(id)
		if !seen[s] {
			seen[s] = true
			newIDs = append(newIDs, id)
		}
	}
	if len(newIDs) == 0 {
		return nil, nil
	}

	emails, err := fetchEmailHeaders(cli, accountID, newIDs)
	if err != nil {
		return nil, err
	}

	var cands []candidate
	for _, e := range emails {
		h := extractMsgHeaders(e)
		cls := classify(h)
		ra := safeTime(e.ReceivedAt)
		inWin := windowCutoff == nil || ra.After(*windowCutoff) || ra.Equal(*windowCutoff)
		// For github-ci, use the List-Id as the diversity key so that
		// notifications from different repos count as different senders.
		sk := senderKey(e.From)
		if cls == "github-ci" && h.listID != "" {
			sk = strings.ToLower(strings.Trim(h.listID, " <>"))
		}
		cands = append(cands, candidate{
			id:         string(e.ID),
			blobID:     string(e.BlobID),
			from:       formatFrom(e.From),
			senderKey:  sk,
			subject:    e.Subject,
			receivedAt: ra,
			folder:     mbox.Name,
			class:      cls,
			inWindow:   inWin,
		})
	}
	return cands, nil
}

func queryMailboxIDs(cli *jmap.Client, accountID jmap.ID, mboxID jmap.ID, after *time.Time) ([]jmap.ID, error) {
	var all []jmap.ID
	var position int64
	for {
		filter := &email.FilterCondition{InMailbox: mboxID}
		if after != nil {
			t := *after
			filter.After = &t
		}
		req := &jmap.Request{}
		req.Invoke(&email.Query{
			Account:  accountID,
			Filter:   filter,
			Sort:     []*email.SortComparator{{Property: "receivedAt", IsAscending: false}},
			Position: position,
			Limit:    queryLimit,
		})
		resp, err := cli.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Email/query: %w", err)
		}
		var batch []jmap.ID
		for _, inv := range resp.Responses {
			qr, ok := inv.Args.(*email.QueryResponse)
			if !ok {
				continue
			}
			batch = qr.IDs
		}
		all = append(all, batch...)
		if len(batch) < queryLimit {
			break
		}
		position += int64(len(batch))
	}
	return all, nil
}

func fetchEmailHeaders(cli *jmap.Client, accountID jmap.ID, ids []jmap.ID) ([]*email.Email, error) {
	var all []*email.Email
	for chunk := range slices.Chunk(ids, fetchChunk) {
		req := &jmap.Request{}
		req.Invoke(&email.Get{
			Account:    accountID,
			IDs:        chunk,
			Properties: classificationProperties,
		})
		resp, err := cli.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Email/get: %w", err)
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

func extractMsgHeaders(e *email.Email) msgHeaders {
	h := msgHeaders{
		from:    formatFrom(e.From),
		subject: e.Subject,
	}
	for _, raw := range e.Headers {
		switch strings.ToLower(raw.Name) {
		case "list-id":
			h.listID = raw.Value
		case "list-unsubscribe":
			h.listUnsubscribe = raw.Value
		case "content-type":
			h.contentType = raw.Value
		}
	}
	return h
}

// selectCandidates picks up to max candidates with round-robin sender
// diversity, capping each sender at senderMax.
func selectCandidates(cands []candidate, max, senderMax int) []candidate {
	bySender := make(map[string][]candidate)
	var senderOrder []string
	for _, c := range cands {
		if _, exists := bySender[c.senderKey]; !exists {
			senderOrder = append(senderOrder, c.senderKey)
		}
		bySender[c.senderKey] = append(bySender[c.senderKey], c)
	}

	senderIdx := make(map[string]int, len(senderOrder))
	var out []candidate
	for len(out) < max {
		progress := false
		for _, sk := range senderOrder {
			if len(out) >= max {
				break
			}
			idx := senderIdx[sk]
			if idx >= senderMax || idx >= len(bySender[sk]) {
				continue
			}
			out = append(out, bySender[sk][idx])
			senderIdx[sk] = idx + 1
			progress = true
		}
		if !progress {
			break
		}
	}
	return out
}

func groupByClass(cands []candidate) map[string][]candidate {
	m := make(map[string][]candidate)
	for _, c := range cands {
		if c.class != "unclassified" {
			m[c.class] = append(m[c.class], c)
		}
	}
	return m
}

func downloadBlob(cli *jmap.Client, accountID jmap.ID, blobID string) ([]byte, error) {
	rc, err := cli.Download(accountID, jmap.ID(blobID))
	if err != nil {
		return nil, fmt.Errorf("download blob %s: %w", blobID, err)
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func formatFrom(addrs []*jmapmail.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	a := addrs[0]
	if a.Name != "" {
		return a.Name + " <" + a.Email + ">"
	}
	return a.Email
}

func senderKey(addrs []*jmapmail.Address) string {
	if len(addrs) == 0 {
		return ""
	}
	return strings.ToLower(addrs[0].Email)
}

func safeTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func sanitizeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
