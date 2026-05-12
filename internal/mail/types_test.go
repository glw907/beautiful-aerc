package mail

import "testing"

func TestDispositionString(t *testing.T) {
	cases := []struct {
		d    Disposition
		want string
	}{
		{DispAttachment, "attachment"},
		{DispInline, "inline"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Errorf("Disposition(%d).String() = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestParseDisposition(t *testing.T) {
	cases := []struct {
		in      string
		want    Disposition
		wantErr bool
	}{
		{"attachment", DispAttachment, false},
		{"inline", DispInline, false},
		{"ATTACHMENT", DispAttachment, false},
		{"", 0, true},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := ParseDisposition(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("ParseDisposition(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if !c.wantErr && got != c.want {
			t.Errorf("ParseDisposition(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestClassifyDisposition(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		contentID string
		want      Disposition
	}{
		{"explicit attachment", "attachment", "", DispAttachment},
		{"explicit inline", "inline", "", DispInline},
		{"raw wins over cid", "attachment", "<cid@x>", DispAttachment},
		{"unknown raw + cid → inline", "", "<cid@x>", DispInline},
		{"unknown raw + blank cid → attachment", "", "   ", DispAttachment},
		{"unknown raw + no cid → attachment", "bogus", "", DispAttachment},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyDisposition(c.raw, c.contentID); got != c.want {
				t.Errorf("ClassifyDisposition(%q, %q) = %v, want %v", c.raw, c.contentID, got, c.want)
			}
		})
	}
}
