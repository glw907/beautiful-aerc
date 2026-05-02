// SPDX-License-Identifier: MIT

package mail

import "testing"

func TestClassifyEmpty(t *testing.T) {
	if got := Classify(nil); got != nil {
		t.Errorf("Classify(nil) = %v, want nil", got)
	}
	if got := Classify([]Folder{}); got != nil {
		t.Errorf("Classify([]) = %v, want nil", got)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		in      Folder
		wantCan string
		wantDN  string
		wantGrp Group
	}{
		{
			name:    "role attribute wins over name",
			in:      Folder{Name: "Weird Inbox Name", Role: "inbox"},
			wantCan: "Inbox",
			wantDN:  "Inbox",
			wantGrp: GroupPrimary,
		},
		{
			name:    "inbox by alias no role",
			in:      Folder{Name: "INBOX"},
			wantCan: "Inbox",
			wantDN:  "Inbox",
			wantGrp: GroupPrimary,
		},
		{
			name:    "gmail sent mail via alias",
			in:      Folder{Name: "[Gmail]/Sent Mail"},
			wantCan: "Sent",
			wantDN:  "Sent",
			wantGrp: GroupPrimary,
		},
		{
			name:    "outlook sent items",
			in:      Folder{Name: "Sent Items"},
			wantCan: "Sent",
			wantDN:  "Sent",
			wantGrp: GroupPrimary,
		},
		{
			name:    "icloud deleted messages",
			in:      Folder{Name: "Deleted Messages"},
			wantCan: "Trash",
			wantDN:  "Trash",
			wantGrp: GroupDisposal,
		},
		{
			name:    "junk alias maps to Spam",
			in:      Folder{Name: "Junk"},
			wantCan: "Spam",
			wantDN:  "Spam",
			wantGrp: GroupDisposal,
		},
		{
			name:    "gmail all mail maps to Archive",
			in:      Folder{Name: "[Gmail]/All Mail"},
			wantCan: "Archive",
			wantDN:  "Archive",
			wantGrp: GroupPrimary,
		},
		{
			name:    "outlook junk email",
			in:      Folder{Name: "Junk Email"},
			wantCan: "Spam",
			wantDN:  "Spam",
			wantGrp: GroupDisposal,
		},
		{
			name:    "role junk maps to Spam",
			in:      Folder{Name: "whatever", Role: "junk"},
			wantCan: "Spam",
			wantDN:  "Spam",
			wantGrp: GroupDisposal,
		},
		{
			name:    "gmail starred is custom",
			in:      Folder{Name: "[Gmail]/Starred"},
			wantCan: "",
			wantDN:  "[Gmail]/Starred",
			wantGrp: GroupCustom,
		},
		{
			name:    "nested custom folder",
			in:      Folder{Name: "Lists/golang"},
			wantCan: "",
			wantDN:  "Lists/golang",
			wantGrp: GroupCustom,
		},
		{
			name:    "drafts role",
			in:      Folder{Name: "Drafts", Role: "drafts"},
			wantCan: "Drafts",
			wantDN:  "Drafts",
			wantGrp: GroupPrimary,
		},
		{
			name:    "archive role",
			in:      Folder{Name: "Archive", Role: "archive"},
			wantCan: "Archive",
			wantDN:  "Archive",
			wantGrp: GroupPrimary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify([]Folder{tt.in})
			if len(got) != 1 {
				t.Fatalf("Classify returned %d results, want 1", len(got))
			}
			cf := got[0]
			if cf.Canonical != tt.wantCan {
				t.Errorf("Canonical = %q, want %q", cf.Canonical, tt.wantCan)
			}
			if cf.DisplayName != tt.wantDN {
				t.Errorf("DisplayName = %q, want %q", cf.DisplayName, tt.wantDN)
			}
			if cf.Group != tt.wantGrp {
				t.Errorf("Group = %v, want %v", cf.Group, tt.wantGrp)
			}
			if cf.Folder.Name != tt.in.Name {
				t.Errorf("Folder.Name = %q, want %q", cf.Folder.Name, tt.in.Name)
			}
		})
	}
}

func TestClassifyKnownProviderFolders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Yahoo
		{"yahoo bulk", "Bulk Mail", "Spam"},
		// iCloud
		{"icloud sent", "Sent Messages", "Sent"},
		{"icloud trash", "Deleted Messages", "Trash"},
		// Zoho — uses standard names
		{"zoho sent", "Sent", "Sent"},
		// Gmail (covered fully in Pass 8.1; sample here)
		{"gmail trash", "[Gmail]/Trash", "Trash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyOne(Folder{Name: tc.in})
			if got.Canonical != tc.want {
				t.Errorf("Canonical = %q, want %q", got.Canonical, tc.want)
			}
		})
	}
}

func TestClassifyPreservesOrder(t *testing.T) {
	in := []Folder{
		{Name: "Lists/golang"},
		{Name: "Inbox"},
		{Name: "Trash"},
	}
	got := Classify(in)
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}
	if got[0].DisplayName != "Lists/golang" {
		t.Errorf("got[0] = %q, want Lists/golang", got[0].DisplayName)
	}
	if got[1].Canonical != "Inbox" {
		t.Errorf("got[1] canonical = %q, want Inbox", got[1].Canonical)
	}
	if got[2].Canonical != "Trash" {
		t.Errorf("got[2] canonical = %q, want Trash", got[2].Canonical)
	}
}
