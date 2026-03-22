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
	// Strict still needs minimal caps for container setup (chown, file ownership)
	allowed := map[string]bool{"CHOWN": true, "DAC_OVERRIDE": true, "FOWNER": true}
	for _, cap := range p.AddCaps {
		if !allowed[cap] {
			t.Errorf("strict profile has unexpected capability: %s", cap)
		}
	}
}

func TestPermissiveKeepsCaps(t *testing.T) {
	p := GetProfile("permissive")
	if p.DropAllCaps {
		t.Error("permissive profile should not drop capabilities")
	}
}
