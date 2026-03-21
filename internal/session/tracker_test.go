package session

import "testing"

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "no active sessions"},
		{1, "1 active session"},
		{3, "3 active sessions"},
	}
	for _, tt := range tests {
		got := FormatCount(tt.n)
		if got != tt.want {
			t.Errorf("FormatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
