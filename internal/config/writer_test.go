package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glw907/poplar/internal/mail"
)

func TestRender_MinimalImapAccount(t *testing.T) {
	got := Render([]AccountConfig{{
		Name:        "Personal",
		Backend:     "imap",
		Email:       "user@example.com",
		Host:        "imap.example.com",
		Port:        993,
		PasswordCmd: "op read op://Personal/Mail/credential",
	}}, DefaultUIConfig(), defaultCache())

	want := `[[account]]
name = "Personal"
provider = "imap"
email = "user@example.com"
host = "imap.example.com"
port = 993
password-cmd = "op read op://Personal/Mail/credential"
`
	if string(got) != want {
		t.Errorf("Render() mismatch.\n--got--\n%s\n--want--\n%s", got, want)
	}
}

func TestRender_RoundTripsViaParseAccounts(t *testing.T) {
	in := []AccountConfig{{
		Name:        "Fastmail",
		Backend:     "jmap",
		Email:       "you@fastmail.com",
		Source:      "https://api.fastmail.com/jmap/session",
		PasswordCmd: "echo token",
	}}

	rendered := Render(in, DefaultUIConfig(), defaultCache())
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, rendered, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := ParseAccounts(path)
	if err != nil {
		t.Fatalf("ParseAccounts: %v\n--rendered--\n%s", err, rendered)
	}
	if len(out) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(out))
	}
	got := out[0]
	if got.Name != in[0].Name || got.Email != in[0].Email ||
		got.Source != in[0].Source || got.PasswordCmd != in[0].PasswordCmd {
		t.Errorf("round-trip mismatch:\nin  = %+v\nout = %+v", in[0], got)
	}
}

func TestRender_SMTPBeforeIdentitiesRoundTrips(t *testing.T) {
	in := []AccountConfig{{
		Name:    "Personal",
		Backend: "imap",
		Email:   "u@example.com",
		Host:    "imap.example.com",
		Port:    993,
		SMTP: SMTPConfig{
			Host: "smtp.example.com",
			Port: 465,
		},
		Identities: []Identity{{
			Name:  "Personal",
			Email: "u@example.com",
		}},
		PasswordCmd: "echo x",
	}}
	rendered := Render(in, DefaultUIConfig(), defaultCache())
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, rendered, 0644); err != nil {
		t.Fatal(err)
	}

	out, err := ParseAccounts(path)
	if err != nil {
		t.Fatalf("ParseAccounts: %v\n--rendered--\n%s", err, rendered)
	}
	if got := out[0].SMTP.Host; got != "smtp.example.com" {
		t.Errorf("SMTP.Host = %q, want smtp.example.com (round-trip lost SMTP block)", got)
	}
	if len(out[0].Identities) != 1 {
		t.Fatalf("Identities len = %d, want 1", len(out[0].Identities))
	}
	if got := out[0].Identities[0].Email; got != "u@example.com" {
		t.Errorf("Identities[0].Email = %q, want u@example.com", got)
	}
}

func TestRender_UIBlockOmittedAtDefaults(t *testing.T) {
	got := Render([]AccountConfig{{Name: "x", Backend: "imap", Email: "x@x"}}, DefaultUIConfig(), defaultCache())
	if strings.Contains(string(got), "[ui]") {
		t.Errorf("Render() emitted [ui] block at defaults:\n%s", got)
	}
	if strings.Contains(string(got), "[cache]") {
		t.Errorf("Render() emitted [cache] block at defaults:\n%s", got)
	}
}

func TestRender_UIBlockEmittedWhenNonDefault(t *testing.T) {
	ui := DefaultUIConfig()
	ui.UndoSeconds = 12
	ui.UndoSendWindow = 30 * time.Second

	got := string(Render([]AccountConfig{{Name: "x", Backend: "imap", Email: "x@x"}}, ui, defaultCache()))
	for _, want := range []string{"[ui]", "undo_seconds = 12", `undo-send-window = "30s"`} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() missing %q in:\n%s", want, got)
		}
	}
}

func TestRender_CacheBlockEmittedWhenSet(t *testing.T) {
	c := CacheConfig{MaxSize: 512 * 1024 * 1024, MaxAttachmentSize: 4 * 1024 * 1024 * 1024}
	got := string(Render([]AccountConfig{{Name: "x", Backend: "imap", Email: "x@x"}}, DefaultUIConfig(), c))
	for _, want := range []string{"[cache]", `max-size = "512MB"`, `max-attachment-size = "4GB"`} {
		if !strings.Contains(got, want) {
			t.Errorf("Render() missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderOAuthAccount(t *testing.T) {
	acct := AccountConfig{
		Name:    "Work",
		Preset:  "gmail",
		Backend: "imap",
		Email:   "u@gmail.com",
		Auth:    "xoauth2",
		OAuth: &OAuthConfig{
			ClientID:     "id",
			ClientSecret: "sec",
			Scopes:       []string{"s"},
		},
		OAuthStore: "keyring",
	}
	out := string(Render([]AccountConfig{acct}, DefaultUIConfig(), defaultCache()))
	for _, want := range []string{
		`provider = "gmail"`,
		`oauth-store = "keyring"`,
		`[account.oauth]`,
		`client-id = "id"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderFolderSubsections_Empty(t *testing.T) {
	got := RenderFolderSubsections(nil, nil)
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestRenderFolderSubsections_CanonicalsAndCustom(t *testing.T) {
	classified := mail.Classify([]mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "Drafts", Role: "drafts"},
		{Name: "Sent", Role: "sent"},
		{Name: "Archive", Role: "archive"},
		{Name: "Junk"},
		{Name: "Trash", Role: "trash"},
		{Name: "Lists/golang"},
		{Name: "Lists/rust"},
	})
	got := RenderFolderSubsections(classified, nil)

	expectOrder := []string{
		`[ui.folders.Inbox]`,
		`[ui.folders.Drafts]`,
		`[ui.folders.Sent]`,
		`[ui.folders.Archive]`,
		``,
		`[ui.folders.Spam]`,
		`[ui.folders.Trash]`,
		``,
		`[ui.folders."Lists/golang"]`,
		`[ui.folders."Lists/rust"]`,
	}
	lines := strings.Split(got, "\n")
	var headers []string
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "[ui.folders") {
			headers = append(headers, line)
		}
	}
	if !sliceContainsSubseq(headers, expectOrder) {
		t.Fatalf("expected header order %v in output, got %v\nfull output:\n%s", expectOrder, headers, got)
	}

	wantFields := []string{"# label =", "# rank =", "# threading =", "# sort =", "# hide ="}
	for _, f := range wantFields {
		if !strings.Contains(got, f) {
			t.Errorf("expected %q in output", f)
		}
	}
}

func TestRenderFolderSubsections_SkipsExisting(t *testing.T) {
	classified := mail.Classify([]mail.Folder{
		{Name: "Inbox", Role: "inbox"},
		{Name: "Drafts", Role: "drafts"},
	})
	existing := map[string]bool{"Inbox": true}
	got := RenderFolderSubsections(classified, existing)

	if strings.Contains(got, "[ui.folders.Inbox]") {
		t.Errorf("Inbox subsection should have been skipped, got %q", got)
	}
	if !strings.Contains(got, "[ui.folders.Drafts]") {
		t.Errorf("Drafts subsection should be present")
	}
}

func TestRenderFolderSubsections_QuotesCustomNames(t *testing.T) {
	classified := mail.Classify([]mail.Folder{
		{Name: "Lists/golang"},
	})
	got := RenderFolderSubsections(classified, nil)
	if !strings.Contains(got, `[ui.folders."Lists/golang"]`) {
		t.Errorf("custom folder name should be quoted, got %q", got)
	}
}

func TestMergeFolderSubsections_EmptyNewContent(t *testing.T) {
	orig := "[ui]\nthreading = true\n"
	got := MergeFolderSubsections([]byte(orig), "")
	if got != orig {
		t.Errorf("expected unchanged file, got %q", got)
	}
}

func TestMergeFolderSubsections_Appends(t *testing.T) {
	orig := "[ui]\nthreading = true\n"
	got := MergeFolderSubsections([]byte(orig), "[ui.folders.Inbox]\n# rank = 0\n")
	want := "[ui]\nthreading = true\n\n[ui.folders.Inbox]\n# rank = 0\n"
	if got != want {
		t.Errorf("merge mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFolderKeys(t *testing.T) {
	contents := `[ui]
threading = true

[ui.folders.Inbox]
rank = 1

[ui.folders."Lists/golang"]
rank = 5
`
	keys, err := FolderKeys([]byte(contents))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Inbox": true, "Lists/golang": true}
	if len(keys) != len(want) {
		t.Fatalf("got %d keys, want %d: %v", len(keys), len(want), keys)
	}
	for k := range want {
		if !keys[k] {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestRenderMultilineSignature(t *testing.T) {
	accts := []AccountConfig{{
		Name:     "geoff",
		Preset:   "fastmail",
		Backend:  "jmap",
		Email:    "geoff@907.life",
		Password: "tok",
		Identities: []Identity{{
			Name:  "Geoff",
			Email: "geoff@907.life",
			Signatures: []Signature{{
				Name: "default",
				Text: "Geoff Wright\ngeoff@907.life",
			}},
		}},
	}}
	got := string(Render(accts, UIConfig{}, CacheConfig{}))
	want := "text = \"\"\"\nGeoff Wright\ngeoff@907.life\"\"\"\n"
	if !strings.Contains(got, want) {
		t.Errorf("Render output missing multi-line signature block.\n--- want substring ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestRenderParseSignatureRoundTrip(t *testing.T) {
	body := "Geoff Wright\ngeoff@907.life"
	accts := []AccountConfig{{
		Name:     "geoff",
		Preset:   "fastmail",
		Backend:  "jmap",
		Email:    "geoff@907.life",
		Password: "tok",
		Identities: []Identity{{
			Name:       "Geoff",
			Email:      "geoff@907.life",
			Signatures: []Signature{{Name: "default", Text: body}},
		}},
	}}
	tomlBytes := Render(accts, UIConfig{}, CacheConfig{})
	parsed, err := ParseAccountsFromBytes(tomlBytes)
	if err != nil {
		t.Fatalf("ParseAccountsFromBytes: %v", err)
	}
	if len(parsed) != 1 || len(parsed[0].Identities) != 1 || len(parsed[0].Identities[0].Signatures) != 1 {
		t.Fatalf("unexpected shape: %+v", parsed)
	}
	want := "-- \n" + body
	if got := parsed[0].Identities[0].Signatures[0].Text; got != want {
		t.Errorf("round-trip Text = %q, want %q", got, want)
	}
}

func sliceContainsSubseq(src, target []string) bool {
	i := 0
	for _, s := range src {
		if i < len(target) && s == target[i] {
			i++
		}
	}
	return i == len(target)
}
