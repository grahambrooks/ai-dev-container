package security

import "testing"

func TestGetProfile(t *testing.T) {
	tests := []struct {
		name     string
		wantName string
	}{
		{"strict", "strict"},
		{"moderate", "moderate"},
		{"permissive", "permissive"},
		{"unknown", "moderate"}, // falls back to moderate
	}

	for _, tt := range tests {
		p := GetProfile(tt.name)
		if p.Name != tt.wantName {
			t.Errorf("GetProfile(%q).Name = %q, want %q", tt.name, p.Name, tt.wantName)
		}
	}
}

func TestStrictDropsAllCaps(t *testing.T) {
	p := GetProfile("strict")
	if !p.DropAllCaps {
		t.Error("strict profile should drop all capabilities")
	}
	if len(p.AddCaps) > 0 {
		t.Error("strict profile should not add any capabilities")
	}
}

func TestPermissiveKeepsCaps(t *testing.T) {
	p := GetProfile("permissive")
	if p.DropAllCaps {
		t.Error("permissive profile should not drop capabilities")
	}
}
