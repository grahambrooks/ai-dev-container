package agent

import (
	"os"
	"runtime"
	"sort"
	"testing"
)

func TestGetProfile(t *testing.T) {
	tests := []struct {
		name string
		want string
		nil  bool
	}{
		{"claude", "claude", false},
		{"codex", "codex", false},
		{"copilot", "copilot", false},
		{"gemini", "gemini", false},
		{"aider", "aider", false},
		{"opencode", "opencode", false},
		{"nonexistent", "", true},
	}
	for _, tt := range tests {
		p := GetProfile(tt.name)
		if tt.nil {
			if p != nil {
				t.Errorf("GetProfile(%q) = %v, want nil", tt.name, p)
			}
		} else {
			if p == nil {
				t.Fatalf("GetProfile(%q) = nil, want %q", tt.name, tt.want)
			}
			if p.Name != tt.want {
				t.Errorf("GetProfile(%q).Name = %q, want %q", tt.name, p.Name, tt.want)
			}
		}
	}
}

func TestListProfiles(t *testing.T) {
	names := ListProfiles()
	if len(names) == 0 {
		t.Fatal("ListProfiles() returned empty list")
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("ListProfiles() not sorted: %v", names)
	}
	// Verify all known agents are present
	expected := map[string]bool{"claude": false, "codex": false, "copilot": false, "gemini": false, "aider": false, "opencode": false}
	for _, n := range names {
		expected[n] = true
	}
	for k, found := range expected {
		if !found {
			t.Errorf("ListProfiles() missing %q", k)
		}
	}
}

func TestFormatProfileList(t *testing.T) {
	out := FormatProfileList()
	if out == "" {
		t.Fatal("FormatProfileList() returned empty string")
	}
	for _, name := range ListProfiles() {
		p := GetProfile(name)
		if !contains(out, p.Name) || !contains(out, p.DisplayName) {
			t.Errorf("FormatProfileList() missing profile %q / %q", p.Name, p.DisplayName)
		}
	}
}

func TestProfileFields(t *testing.T) {
	for _, name := range ListProfiles() {
		p := GetProfile(name)
		if p.DisplayName == "" {
			t.Errorf("profile %q has empty DisplayName", name)
		}
		if p.Binary == "" {
			t.Errorf("profile %q has empty Binary", name)
		}
		if p.InstallCmd == "" {
			t.Errorf("profile %q has empty InstallCmd", name)
		}
	}
}

func TestClaudeProfileHasResolveCreds(t *testing.T) {
	p := GetProfile("claude")
	if p.ResolveCreds == nil {
		t.Error("claude profile should have ResolveCreds")
	}
}

func TestSSHAuthSockMount_Unset(t *testing.T) {
	orig := os.Getenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AUTH_SOCK")
	defer func() {
		if orig != "" {
			os.Setenv("SSH_AUTH_SOCK", orig)
		}
	}()

	host, container := SSHAuthSockMount()
	if host != "" || container != "" {
		t.Errorf("SSHAuthSockMount() with no SSH_AUTH_SOCK = (%q, %q), want (\"\", \"\")", host, container)
	}
}

func TestSSHAuthSockMount_Set(t *testing.T) {
	orig := os.Getenv("SSH_AUTH_SOCK")
	os.Setenv("SSH_AUTH_SOCK", "/tmp/test-agent.sock")
	defer func() {
		if orig != "" {
			os.Setenv("SSH_AUTH_SOCK", orig)
		} else {
			os.Unsetenv("SSH_AUTH_SOCK")
		}
	}()

	host, container := SSHAuthSockMount()
	if container == "" {
		t.Error("SSHAuthSockMount() container socket should not be empty when SSH_AUTH_SOCK is set")
	}
	if runtime.GOOS == "darwin" {
		if host != "" {
			t.Error("on macOS, host socket should be empty (Docker Desktop injects it)")
		}
		if container != "/run/host-services/ssh-auth.sock" {
			t.Errorf("on macOS, container socket = %q, want /run/host-services/ssh-auth.sock", container)
		}
	} else {
		if host != "/tmp/test-agent.sock" {
			t.Errorf("on Linux, host socket = %q, want /tmp/test-agent.sock", host)
		}
	}
}

func TestCommonAuthMounts(t *testing.T) {
	mounts := CommonAuthMounts()
	// Should always include .ssh
	found := false
	for _, m := range mounts {
		if m.HostPath == ".ssh" {
			found = true
			if !m.ReadOnly {
				t.Error(".ssh mount should be read-only")
			}
		}
	}
	if !found {
		t.Error("CommonAuthMounts() should include .ssh")
	}
}

func TestNetworkAllowNotEmpty(t *testing.T) {
	for _, name := range ListProfiles() {
		p := GetProfile(name)
		if len(p.NetworkAllow) == 0 {
			t.Errorf("profile %q has empty NetworkAllow", name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
