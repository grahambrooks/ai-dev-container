package security

import (
	"testing"

	"github.com/grahambrooks/devc/pkg/types"
)

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

func TestModerateProfile(t *testing.T) {
	p := GetProfile("moderate")
	if p.Network.Mode != "restricted" {
		t.Errorf("moderate network mode = %q, want restricted", p.Network.Mode)
	}
	if p.Resources.CPUs != "4" {
		t.Errorf("moderate CPUs = %q, want 4", p.Resources.CPUs)
	}
	if p.Resources.Memory != "8g" {
		t.Errorf("moderate memory = %q, want 8g", p.Resources.Memory)
	}
	if p.Resources.PidsLimit != 256 {
		t.Errorf("moderate pids limit = %d, want 256", p.Resources.PidsLimit)
	}
	if !p.DropAllCaps {
		t.Error("moderate should drop all caps")
	}
}

func TestStrictResources(t *testing.T) {
	p := GetProfile("strict")
	if p.Resources.CPUs != "2" {
		t.Errorf("strict CPUs = %q, want 2", p.Resources.CPUs)
	}
	if p.Resources.Memory != "4g" {
		t.Errorf("strict memory = %q, want 4g", p.Resources.Memory)
	}
	if p.Network.Mode != "none" {
		t.Errorf("strict network mode = %q, want none", p.Network.Mode)
	}
}

func TestAllProfilesHaveRunAsUser(t *testing.T) {
	for _, name := range []string{"strict", "moderate", "permissive"} {
		p := GetProfile(name)
		if p.RunAsUser != "1000:1000" {
			t.Errorf("profile %q RunAsUser = %q, want 1000:1000", name, p.RunAsUser)
		}
	}
}

func TestApplyCustomizations_Nil(t *testing.T) {
	base := GetProfile("moderate")
	result := ApplyCustomizations(base, nil)
	if result != base {
		t.Error("nil customization should return base profile unchanged")
	}
}

func TestApplyCustomizations_NetworkOverride(t *testing.T) {
	base := GetProfile("moderate")
	custom := &types.DevcCustomization{
		Network: &types.NetworkConfig{
			Mode:      "none",
			Allowlist: []string{"example.com"},
		},
	}
	result := ApplyCustomizations(base, custom)
	if result.Network.Mode != "none" {
		t.Errorf("network mode = %q, want none", result.Network.Mode)
	}
	if len(result.Network.Allowlist) != 1 || result.Network.Allowlist[0] != "example.com" {
		t.Errorf("allowlist = %v, want [example.com]", result.Network.Allowlist)
	}
	// Base should not be modified
	if base.Network.Mode != "restricted" {
		t.Error("base profile was modified")
	}
}

func TestApplyCustomizations_ResourceOverride(t *testing.T) {
	base := GetProfile("strict")
	custom := &types.DevcCustomization{
		Resources: &types.ResourceConfig{
			CPUs:   "8",
			Memory: "16g",
		},
	}
	result := ApplyCustomizations(base, custom)
	if result.Resources.CPUs != "8" {
		t.Errorf("CPUs = %q, want 8", result.Resources.CPUs)
	}
	if result.Resources.Memory != "16g" {
		t.Errorf("memory = %q, want 16g", result.Resources.Memory)
	}
	// Base should not be modified
	if base.Resources.CPUs != "2" {
		t.Error("base profile was modified")
	}
}

func TestApplyCustomizations_NoOverride(t *testing.T) {
	base := GetProfile("moderate")
	custom := &types.DevcCustomization{}
	result := ApplyCustomizations(base, custom)
	if result.Network.Mode != base.Network.Mode {
		t.Error("empty customization should not change network")
	}
	if result.Resources.CPUs != base.Resources.CPUs {
		t.Error("empty customization should not change resources")
	}
}
