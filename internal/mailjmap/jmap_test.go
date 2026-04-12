package mailjmap

import (
	"testing"

	"github.com/glw907/poplar/internal/mailworker/models"
	"github.com/glw907/poplar/internal/mail"
)

func TestTranslateFolder(t *testing.T) {
	tests := []struct {
		name string
		dir  *models.Directory
		want mail.Folder
	}{
		{
			name: "inbox with unread",
			dir: &models.Directory{
				Name:   "Inbox",
				Exists: 42,
				Unseen: 5,
				Role:   models.InboxRole,
			},
			want: mail.Folder{
				Name:   "Inbox",
				Exists: 42,
				Unseen: 5,
				Role:   "inbox",
			},
		},
		{
			name: "sent with no role",
			dir: &models.Directory{
				Name:   "Sent",
				Exists: 100,
				Unseen: 0,
			},
			want: mail.Folder{
				Name:   "Sent",
				Exists: 100,
				Unseen: 0,
				Role:   "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateFolder(tt.dir)
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Exists != tt.want.Exists {
				t.Errorf("Exists = %d, want %d", got.Exists, tt.want.Exists)
			}
			if got.Unseen != tt.want.Unseen {
				t.Errorf("Unseen = %d, want %d", got.Unseen, tt.want.Unseen)
			}
			if got.Role != tt.want.Role {
				t.Errorf("Role = %q, want %q", got.Role, tt.want.Role)
			}
		})
	}
}
