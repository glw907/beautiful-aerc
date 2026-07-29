package jmap

import (
	"encoding/json"
	"testing"
)

// TestEmailDecode proves a message record reads back whole, so a
// mistyped tag cannot leave a header, an address, or a body part
// silently missing.
func TestEmailDecode(t *testing.T) {
	raw := []byte(`{"id":"M1","blobId":"G1","threadId":"T1",` +
		`"mailboxIds":{"MA":true},"keywords":{"$seen":true},"size":4127,` +
		`"receivedAt":"2026-07-28T09:00:00Z","sentAt":"2026-07-28T08:59:00Z",` +
		`"headers":[{"name":"X-Spam-Score","value":"0.1"}],` +
		`"messageId":["m1@example.com"],"inReplyTo":["m0@example.com"],` +
		`"references":["m0@example.com"],` +
		`"from":[{"name":"Ann","email":"ann@example.com"}],` +
		`"to":[{"email":"bob@example.com"}],"subject":"Lunch",` +
		`"textBody":[{"partId":"1","type":"text/plain","size":12}],` +
		`"bodyValues":{"1":{"value":"see you at 1","isTruncated":false}},` +
		`"hasAttachment":false,"preview":"see you at 1"}`)

	var email Email
	if err := json.Unmarshal(raw, &email); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if email.ID != "M1" || email.BlobID != "G1" || email.ThreadID != "T1" {
		t.Errorf("ids = %q/%q/%q, want M1/G1/T1", email.ID, email.BlobID, email.ThreadID)
	}
	if !email.MailboxIDs["MA"] {
		t.Errorf("MailboxIDs = %v, want MA", email.MailboxIDs)
	}
	if !email.Keywords["$seen"] {
		t.Errorf("Keywords = %v, want $seen", email.Keywords)
	}
	if email.Size != 4127 {
		t.Errorf("Size = %d, want 4127", email.Size)
	}
	if email.ReceivedAt == nil || email.ReceivedAt.Time().Hour() != 9 {
		t.Errorf("ReceivedAt = %v, want 09:00Z", email.ReceivedAt)
	}
	if email.SentAt == nil || email.SentAt.Time().Minute() != 59 {
		t.Errorf("SentAt = %v, want 08:59Z", email.SentAt)
	}
	if len(email.Headers) != 1 || email.Headers[0].Name != "X-Spam-Score" {
		t.Errorf("Headers = %v, want the spam score", email.Headers)
	}
	if len(email.MessageID) != 1 || len(email.InReplyTo) != 1 || len(email.References) != 1 {
		t.Errorf("threading headers = %v/%v/%v, want one each",
			email.MessageID, email.InReplyTo, email.References)
	}
	if len(email.From) != 1 || email.From[0].String() != "Ann <ann@example.com>" {
		t.Errorf("From = %v, want Ann <ann@example.com>", email.From)
	}
	if len(email.To) != 1 || email.To[0].String() != "bob@example.com" {
		t.Errorf("To = %v, want a bare address", email.To)
	}
	if email.Subject != "Lunch" || email.Preview != "see you at 1" {
		t.Errorf("Subject/Preview = %q/%q", email.Subject, email.Preview)
	}
	if len(email.TextBody) != 1 || email.TextBody[0].Type != "text/plain" {
		t.Errorf("TextBody = %v, want one text part", email.TextBody)
	}
	if body := email.BodyValues["1"]; body == nil || body.Value != "see you at 1" {
		t.Errorf("BodyValues[1] = %v, want the fetched content", body)
	}
}

// TestBodyValueTruncationAlwaysMarshals pins the one Boolean in the
// package that is sent even when false. A body value that says
// nothing about truncation reads as complete, and treating a cut-off
// body as whole is silent data loss.
func TestBodyValueTruncationAlwaysMarshals(t *testing.T) {
	data, err := json.Marshal(&BodyValue{Value: "hi"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"value":"hi","isTruncated":false}`; string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}

	truncated, err := json.Marshal(&BodyValue{Value: "hi", IsTruncated: true})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"value":"hi","isTruncated":true}`; string(truncated) != want {
		t.Errorf("Marshal = %s, want %s", truncated, want)
	}
}

// TestBodyPartNesting proves a MIME tree survives decoding to the
// depth a real multipart message reaches.
func TestBodyPartNesting(t *testing.T) {
	raw := []byte(`{"type":"multipart/mixed","subParts":[` +
		`{"type":"multipart/alternative","subParts":[` +
		`{"partId":"1","type":"text/plain","size":10},` +
		`{"partId":"2","type":"text/html","size":24}]},` +
		`{"partId":"3","blobId":"G2","type":"application/pdf","name":"receipt.pdf",` +
		`"disposition":"attachment","size":9000,"cid":"c1","language":["en"],` +
		`"charset":"utf-8","location":"http://example.com/r"}]}`)

	var part BodyPart
	if err := json.Unmarshal(raw, &part); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(part.SubParts) != 2 {
		t.Fatalf("SubParts = %d, want 2", len(part.SubParts))
	}
	alternative := part.SubParts[0]
	if len(alternative.SubParts) != 2 || alternative.SubParts[1].Type != "text/html" {
		t.Errorf("nested alternative = %v, want a plain and an HTML part", alternative.SubParts)
	}

	attachment := part.SubParts[1]
	fields := []struct {
		name string
		got  string
		want string
	}{
		{"PartID", attachment.PartID, "3"},
		{"BlobID", string(attachment.BlobID), "G2"},
		{"Type", attachment.Type, "application/pdf"},
		{"Name", attachment.Name, "receipt.pdf"},
		{"Disposition", attachment.Disposition, "attachment"},
		{"CID", attachment.CID, "c1"},
		{"Charset", attachment.Charset, "utf-8"},
		{"Location", attachment.Location, "http://example.com/r"},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("BodyPart.%s = %q, want %q", f.name, f.got, f.want)
		}
	}
	if attachment.Size != 9000 {
		t.Errorf("Size = %d, want 9000", attachment.Size)
	}
	if len(attachment.Language) != 1 || attachment.Language[0] != "en" {
		t.Errorf("Language = %v, want [en]", attachment.Language)
	}
}

// TestIdentityDecode proves the identity record reads back, including
// the signatures a compose screen seeds from.
func TestIdentityDecode(t *testing.T) {
	raw := []byte(`{"id":"I1","name":"Ann","email":"ann@example.com",` +
		`"replyTo":[{"email":"replies@example.com"}],"bcc":[{"email":"archive@example.com"}],` +
		`"textSignature":"-- \nAnn","htmlSignature":"<p>Ann</p>","mayDelete":true}`)

	var identity Identity
	if err := json.Unmarshal(raw, &identity); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if identity.ID != "I1" || identity.Name != "Ann" || identity.Email != "ann@example.com" {
		t.Errorf("identity = %+v", identity)
	}
	if len(identity.ReplyTo) != 1 || identity.ReplyTo[0].Email != "replies@example.com" {
		t.Errorf("ReplyTo = %v", identity.ReplyTo)
	}
	if len(identity.Bcc) != 1 || identity.Bcc[0].Email != "archive@example.com" {
		t.Errorf("Bcc = %v", identity.Bcc)
	}
	if identity.TextSignature == "" || identity.HTMLSignature == "" {
		t.Errorf("signatures = %q/%q, want both", identity.TextSignature, identity.HTMLSignature)
	}
	if !identity.MayDelete {
		t.Error("MayDelete = false, want true")
	}
}

// TestEmailSubmissionDecode proves the submission record reads back,
// including the per-recipient delivery status the outbox reports from.
func TestEmailSubmissionDecode(t *testing.T) {
	raw := []byte(`{"id":"ES1","identityId":"I1","emailId":"M1","threadId":"T1",` +
		`"envelope":{"mailFrom":{"email":"ann@example.com","parameters":{"HOLDFOR":"86400"}},` +
		`"rcptTo":[{"email":"bob@example.com","parameters":null}]},` +
		`"sendAt":"2026-07-28T09:00:00Z","undoStatus":"final",` +
		`"deliveryStatus":{"bob@example.com":{"smtpReply":"250 2.0.0 Ok","delivered":"yes",` +
		`"displayed":"unknown"}},"dsnBlobIds":["G3"],"mdnBlobIds":["G4"]}`)

	var submission EmailSubmission
	if err := json.Unmarshal(raw, &submission); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if submission.UndoStatus != "final" {
		t.Errorf("UndoStatus = %q, want final", submission.UndoStatus)
	}
	if submission.SendAt == nil || submission.SendAt.Time().Hour() != 9 {
		t.Errorf("SendAt = %v, want 09:00Z", submission.SendAt)
	}
	if submission.Envelope == nil || submission.Envelope.MailFrom == nil {
		t.Fatal("Envelope.MailFrom is nil")
	}
	if got := submission.Envelope.MailFrom.Parameters["HOLDFOR"]; got != "86400" {
		t.Errorf("HOLDFOR = %v, want 86400", got)
	}
	if len(submission.Envelope.RcptTo) != 1 {
		t.Fatalf("RcptTo = %v, want one recipient", submission.Envelope.RcptTo)
	}
	status := submission.DeliveryStatus["bob@example.com"]
	if status == nil || status.Delivered != "yes" || status.SMTPReply != "250 2.0.0 Ok" {
		t.Errorf("DeliveryStatus = %+v, want a delivered recipient", status)
	}
	if len(submission.DSNBlobIDs) != 1 || len(submission.MDNBlobIDs) != 1 {
		t.Errorf("notification blobs = %v/%v, want one each",
			submission.DSNBlobIDs, submission.MDNBlobIDs)
	}
}
