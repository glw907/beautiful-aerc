// SPDX-License-Identifier: MIT

package mailimap

import (
	"testing"

	"github.com/glw907/poplar/internal/mail"
)

func TestAttachments(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()

	// Fixture: multipart/mixed root
	//   section "1": text/plain  — body, must be skipped
	//   section "2": application/pdf, disposition=attachment, filename=r.pdf, size=4096
	//   section "3": image/png, ContentID="<logo@x>", no explicit disposition
	cmd.bodyStructure[mail.UID("7")] = BodyStructure{
		Section:  "",
		MIMEType: "multipart/mixed",
		Children: []BodyStructure{
			{
				Section:  "1",
				MIMEType: "text/plain",
			},
			{
				Section:     "2",
				MIMEType:    "application/pdf",
				Disposition: "attachment",
				Filename:    "r.pdf",
				SizeBytes:   4096,
			},
			{
				Section:   "3",
				MIMEType:  "image/png",
				ContentID: "<logo@x>",
			},
		},
	}

	b := connectFake(t, cmd)

	atts, err := b.Attachments("7")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("len(atts) = %d, want 2; got %+v", len(atts), atts)
	}

	// PDF: section "2", DispAttachment
	pdf := atts[0]
	if pdf.PartID != "2" {
		t.Errorf("atts[0].PartID = %q, want '2'", pdf.PartID)
	}
	if pdf.MIMEType != "application/pdf" {
		t.Errorf("atts[0].MIMEType = %q, want 'application/pdf'", pdf.MIMEType)
	}
	if pdf.Filename != "r.pdf" {
		t.Errorf("atts[0].Filename = %q, want 'r.pdf'", pdf.Filename)
	}
	if pdf.Size != 4096 {
		t.Errorf("atts[0].Size = %d, want 4096", pdf.Size)
	}
	if pdf.Disposition != mail.DispAttachment {
		t.Errorf("atts[0].Disposition = %v, want DispAttachment", pdf.Disposition)
	}

	// PNG: section "3", DispInline (ContentID fallback)
	img := atts[1]
	if img.PartID != "3" {
		t.Errorf("atts[1].PartID = %q, want '3'", img.PartID)
	}
	if img.MIMEType != "image/png" {
		t.Errorf("atts[1].MIMEType = %q, want 'image/png'", img.MIMEType)
	}
	if img.ContentID != "logo@x" {
		t.Errorf("atts[1].ContentID = %q, want 'logo@x'", img.ContentID)
	}
	if img.Disposition != mail.DispInline {
		t.Errorf("atts[1].Disposition = %v, want DispInline", img.Disposition)
	}
}

func TestAttachmentsTopLevelPlain(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.bodyStructure[mail.UID("1")] = BodyStructure{
		Section:  "1",
		MIMEType: "text/plain",
	}

	b := connectFake(t, cmd)

	atts, err := b.Attachments("1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if len(atts) != 0 {
		t.Errorf("want empty slice, got %+v", atts)
	}
}

// TestAttachmentsNestedPlainKept ensures a text/plain nested inside
// multipart/related (not at top level) is NOT skipped.
func TestAttachmentsNestedPlainKept(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.bodyStructure[mail.UID("2")] = BodyStructure{
		Section:  "",
		MIMEType: "multipart/mixed",
		Children: []BodyStructure{
			{
				Section:  "",
				MIMEType: "multipart/related",
				Children: []BodyStructure{
					{
						Section:  "1.1",
						MIMEType: "text/html",
					},
					{
						Section:   "1.2",
						MIMEType:  "image/png",
						ContentID: "<img@x>",
					},
				},
			},
		},
	}

	b := connectFake(t, cmd)

	atts, err := b.Attachments("2")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	// text/html at 1.1 is nested, so NOT skipped as a body candidate.
	// image/png at 1.2 has ContentID → DispInline.
	// Both should be present.
	if len(atts) != 2 {
		t.Fatalf("want 2 parts, got %d: %+v", len(atts), atts)
	}
}

func TestAttachmentsMissingUID(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()

	b := connectFake(t, cmd)

	_, err := b.Attachments("99")
	if err == nil {
		t.Fatal("expected error for missing UID, got nil")
	}
}

// TestFetchAttachment confirms FetchAttachment returns bytes stored under
// the "<uid>::<section>" key in the fake client.
func TestFetchAttachment(t *testing.T) {
	cmd := newFakeClient()
	cmd.caps = baseCaps()
	cmd.bodyParts["7::2"] = []byte("PDF bytes")

	b := connectFake(t, cmd)

	got, err := b.FetchAttachment("7", "2")
	if err != nil {
		t.Fatalf("FetchAttachment: %v", err)
	}
	want := []byte("PDF bytes")
	if string(got) != string(want) {
		t.Errorf("FetchAttachment = %q, want %q", got, want)
	}
}
