package uicore

import "testing"

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		w          int
		sidebar    int
		sender     int
		date       int
		flagColumn bool
		icons      bool
		spartan    bool
	}{
		// Sub-polish-bar: same shape as 80
		{w: 60, sidebar: 14, sender: 22, date: 0, flagColumn: false, icons: false, spartan: true},
		{w: 79, sidebar: 14, sender: 22, date: 0, flagColumn: false, icons: false, spartan: true},
		// Polish bar floor
		{w: 80, sidebar: 14, sender: 22, date: 0, flagColumn: false, icons: false, spartan: true},
		{w: 85, sidebar: 15, sender: 23, date: 0, flagColumn: false, icons: false, spartan: true},
		{w: 89, sidebar: 16, sender: 23, date: 0, flagColumn: false, icons: false, spartan: true},
		// Flag + compact-date threshold
		{w: 90, sidebar: 16, sender: 23, date: 3, flagColumn: true, icons: false, spartan: false},
		{w: 99, sidebar: 18, sender: 24, date: 3, flagColumn: true, icons: false, spartan: false},
		// Short-date threshold
		{w: 100, sidebar: 18, sender: 25, date: 5, flagColumn: true, icons: false, spartan: false},
		{w: 107, sidebar: 19, sender: 25, date: 5, flagColumn: true, icons: false, spartan: false},
		// Sidebar-icons threshold (sidebar reaches 20)
		{w: 108, sidebar: 20, sender: 26, date: 5, flagColumn: true, icons: true, spartan: false},
		{w: 110, sidebar: 20, sender: 26, date: 5, flagColumn: true, icons: true, spartan: false},
		{w: 120, sidebar: 22, sender: 27, date: 5, flagColumn: true, icons: true, spartan: false},
		{w: 140, sidebar: 26, sender: 30, date: 5, flagColumn: true, icons: true, spartan: false},
		// Sidebar + sender clamps
		{w: 159, sidebar: 30, sender: 32, date: 5, flagColumn: true, icons: true, spartan: false},
		{w: 160, sidebar: 30, sender: 32, date: 5, flagColumn: true, icons: true, spartan: false},
		{w: 220, sidebar: 30, sender: 32, date: 5, flagColumn: true, icons: true, spartan: false},
		{w: 320, sidebar: 30, sender: 32, date: 5, flagColumn: true, icons: true, spartan: false},
	}
	for _, tt := range tests {
		t.Run(layoutName(tt.w), func(t *testing.T) {
			got := ComputeLayout(tt.w)
			if got.Sidebar != tt.sidebar {
				t.Errorf("Sidebar(%d) = %d, want %d", tt.w, got.Sidebar, tt.sidebar)
			}
			if got.Sender != tt.sender {
				t.Errorf("Sender(%d) = %d, want %d", tt.w, got.Sender, tt.sender)
			}
			if got.Date != tt.date {
				t.Errorf("Date(%d) = %d, want %d", tt.w, got.Date, tt.date)
			}
			if got.FlagColumn != tt.flagColumn {
				t.Errorf("FlagColumn(%d) = %v, want %v", tt.w, got.FlagColumn, tt.flagColumn)
			}
			if got.Icons != tt.icons {
				t.Errorf("Icons(%d) = %v, want %v", tt.w, got.Icons, tt.icons)
			}
			if got.Spartan != tt.spartan {
				t.Errorf("Spartan(%d) = %v, want %v", tt.w, got.Spartan, tt.spartan)
			}
		})
	}
}

func layoutName(w int) string {
	return "w_" + itoa(w)
}

// Local helper to avoid importing strconv just for tests.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
