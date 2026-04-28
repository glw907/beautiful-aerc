// SPDX-License-Identifier: MIT

package config

import (
	"strings"
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

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

func TestExistingFolderKeys(t *testing.T) {
	contents := `[ui]
threading = true

[ui.folders.Inbox]
rank = 1

[ui.folders."Lists/golang"]
rank = 5
`
	keys, err := ExistingFolderKeys([]byte(contents))
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

func sliceContainsSubseq(src, target []string) bool {
	i := 0
	for _, s := range src {
		if i < len(target) && s == target[i] {
			i++
		}
	}
	return i == len(target)
}
