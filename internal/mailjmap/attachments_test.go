package mailjmap

import (
	"testing"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/glw907/poplar/internal/mail"
)

func TestAttachments_TwoParts(t *testing.T) {
	// Fixture: a multipart message with one PDF attachment and one
	// inline image identified by CID. The top-level bodyStructure
	// has SubParts so the walker descends into them.
	bodyStructure := &email.BodyPart{
		PartID: "",
		Type:   "multipart/mixed",
		SubParts: []*email.BodyPart{
			{
				PartID:      "1",
				BlobID:      jmap.ID("blob-pdf"),
				Type:        "application/pdf",
				Size:        4096,
				Disposition: "attachment",
				Name:        "report.pdf",
			},
			{
				PartID: "2",
				BlobID: jmap.ID("blob-img"),
				Type:   "image/png",
				Size:   1024,
				CID:    "logo@x",
				// No Disposition set. CID present means DispInline.
				Name: "logo.png",
			},
		},
	}

	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name: "Email/get",
				Args: &email.GetResponse{
					List: []*email.Email{
						{
							ID:            "uid-1",
							BodyStructure: bodyStructure,
						},
					},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	atts, err := b.Attachments("uid-1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("len(atts) = %d, want 2", len(atts))
	}

	// Part 1: PDF attachment.
	pdf := atts[0]
	if pdf.PartID != "1" {
		t.Errorf("pdf.PartID = %q, want %q", pdf.PartID, "1")
	}
	if pdf.Filename != "report.pdf" {
		t.Errorf("pdf.Filename = %q, want %q", pdf.Filename, "report.pdf")
	}
	if pdf.MIMEType != "application/pdf" {
		t.Errorf("pdf.MIMEType = %q, want %q", pdf.MIMEType, "application/pdf")
	}
	if pdf.Size != 4096 {
		t.Errorf("pdf.Size = %d, want 4096", pdf.Size)
	}
	if pdf.ContentID != "" {
		t.Errorf("pdf.ContentID = %q, want empty", pdf.ContentID)
	}
	if pdf.Disposition != mail.DispAttachment {
		t.Errorf("pdf.Disposition = %v, want DispAttachment", pdf.Disposition)
	}

	// Part 2: inline image via CID.
	img := atts[1]
	if img.PartID != "2" {
		t.Errorf("img.PartID = %q, want %q", img.PartID, "2")
	}
	if img.Filename != "logo.png" {
		t.Errorf("img.Filename = %q, want %q", img.Filename, "logo.png")
	}
	if img.MIMEType != "image/png" {
		t.Errorf("img.MIMEType = %q, want %q", img.MIMEType, "image/png")
	}
	if img.Size != 1024 {
		t.Errorf("img.Size = %d, want 1024", img.Size)
	}
	if img.ContentID != "logo@x" {
		t.Errorf("img.ContentID = %q, want %q", img.ContentID, "logo@x")
	}
	if img.Disposition != mail.DispInline {
		t.Errorf("img.Disposition = %v, want DispInline", img.Disposition)
	}

	// partBlobIDs must be populated for both parts.
	b.mu.Lock()
	pmap := b.partBlobIDs["uid-1"]
	b.mu.Unlock()
	if pmap == nil {
		t.Fatal("partBlobIDs[uid-1] is nil")
	}
	if got := pmap["1"]; got != "blob-pdf" {
		t.Errorf("partBlobIDs[uid-1][1] = %q, want %q", got, "blob-pdf")
	}
	if got := pmap["2"]; got != "blob-img" {
		t.Errorf("partBlobIDs[uid-1][2] = %q, want %q", got, "blob-img")
	}
}

func TestAttachments_SkipsTopLevelTextParts(t *testing.T) {
	// A plain single-part text/plain message, with the body at the top
	// level, should return empty attachments.
	bodyStructure := &email.BodyPart{
		PartID: "1",
		BlobID: jmap.ID("blob-body"),
		Type:   "text/plain",
		Size:   500,
	}

	fake := &fakeClient{
		respond: func(_ *jmap.Request) (*jmap.Response, error) {
			return fakeResponse(&jmap.Invocation{
				Name: "Email/get",
				Args: &email.GetResponse{
					List: []*email.Email{
						{
							ID:            "uid-2",
							BodyStructure: bodyStructure,
						},
					},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-1", nil)

	atts, err := b.Attachments("uid-2")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if len(atts) != 0 {
		t.Errorf("len(atts) = %d, want 0 for top-level text/plain", len(atts))
	}

	// partBlobIDs entry should exist but be empty.
	b.mu.Lock()
	pmap := b.partBlobIDs["uid-2"]
	b.mu.Unlock()
	if pmap == nil {
		t.Fatal("partBlobIDs[uid-2] is nil")
	}
	if len(pmap) != 0 {
		t.Errorf("partBlobIDs[uid-2] len = %d, want 0", len(pmap))
	}
}

func TestAttachments_RequestShape(t *testing.T) {
	var capturedReq *jmap.Request
	fake := &fakeClient{
		respond: func(req *jmap.Request) (*jmap.Response, error) {
			capturedReq = req
			return fakeResponse(&jmap.Invocation{
				Name: "Email/get",
				Args: &email.GetResponse{
					List: []*email.Email{{ID: "uid-1"}},
				},
			}), nil
		},
	}
	b := newTestBackend(fake, "acct-42", nil)

	if _, err := b.Attachments("uid-1"); err != nil {
		t.Fatalf("Attachments: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("no request sent")
	}
	if len(capturedReq.Calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(capturedReq.Calls))
	}
	g, ok := capturedReq.Calls[0].Args.(*email.Get)
	if !ok {
		t.Fatalf("args type = %T, want *email.Get", capturedReq.Calls[0].Args)
	}
	if g.Account != "acct-42" {
		t.Errorf("account = %q, want %q", g.Account, "acct-42")
	}
	if len(g.IDs) != 1 || g.IDs[0] != "uid-1" {
		t.Errorf("IDs = %v, want [uid-1]", g.IDs)
	}
	propSet := make(map[string]bool, len(g.Properties))
	for _, p := range g.Properties {
		propSet[p] = true
	}
	for _, want := range attachmentProperties {
		if !propSet[want] {
			t.Errorf("missing required property %q", want)
		}
	}
}

// --- FetchAttachment ---

func TestFetchAttachment(t *testing.T) {
	blobBytes := []byte("fake-pdf-content")

	bodyStructure := &email.BodyPart{
		PartID: "",
		Type:   "multipart/mixed",
		SubParts: []*email.BodyPart{
			{
				PartID:      "1",
				BlobID:      jmap.ID("blob-pdf"),
				Type:        "application/pdf",
				Size:        uint64(len(blobBytes)),
				Disposition: "attachment",
				Name:        "report.pdf",
			},
		},
	}

	t.Run("warm_cache", func(t *testing.T) {
		// Attachments populates partBlobIDs. FetchAttachment should
		// not issue a second Email/get.
		fake := &fakeClient{
			respond: func(_ *jmap.Request) (*jmap.Response, error) {
				return fakeResponse(&jmap.Invocation{
					Name: "Email/get",
					Args: &email.GetResponse{
						List: []*email.Email{
							{ID: "uid-1", BodyStructure: bodyStructure},
						},
					},
				}), nil
			},
		}
		b := newTestBackend(fake, "acct-1", nil)
		b.downloadBlob = func(_ string) ([]byte, error) { return blobBytes, nil }

		// Warm the cache.
		if _, err := b.Attachments("uid-1"); err != nil {
			t.Fatalf("Attachments: %v", err)
		}
		requestsBefore := len(fake.sent)

		got, err := b.FetchAttachment("uid-1", "1")
		if err != nil {
			t.Fatalf("FetchAttachment: %v", err)
		}
		if string(got) != string(blobBytes) {
			t.Errorf("got %q, want %q", got, blobBytes)
		}
		// No extra Email/get should have been issued.
		if len(fake.sent) != requestsBefore {
			t.Errorf("extra requests after warm FetchAttachment: got %d total, want %d", len(fake.sent), requestsBefore)
		}
	})

	t.Run("cold_cache", func(t *testing.T) {
		// No prior Attachments call: FetchAttachment must issue an
		// Email/get to prime partBlobIDs before downloading.
		fake := &fakeClient{
			respond: func(_ *jmap.Request) (*jmap.Response, error) {
				return fakeResponse(&jmap.Invocation{
					Name: "Email/get",
					Args: &email.GetResponse{
						List: []*email.Email{
							{ID: "uid-1", BodyStructure: bodyStructure},
						},
					},
				}), nil
			},
		}
		b := newTestBackend(fake, "acct-1", nil)
		b.downloadBlob = func(_ string) ([]byte, error) { return blobBytes, nil }

		got, err := b.FetchAttachment("uid-1", "1")
		if err != nil {
			t.Fatalf("FetchAttachment cold: %v", err)
		}
		if string(got) != string(blobBytes) {
			t.Errorf("got %q, want %q", got, blobBytes)
		}
		// One Email/get must have been issued by the cold-miss path.
		if len(fake.sent) < 1 {
			t.Error("expected at least one Email/get request for cold FetchAttachment")
		}
	})
}
