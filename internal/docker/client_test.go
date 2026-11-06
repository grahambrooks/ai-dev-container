package docker

import (
	"slices"
	"testing"
)

func TestIsSensitiveEnvKey(t *testing.T) {
	sensitive := []string{
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"MY_TOKEN",
		"GH_TOKEN",
		"GITHUB_TOKEN",
		"DB_PASSWORD",
		"MY_SECRET",
		"AWS_CREDENTIAL",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"NO_PROXY",
		"ALL_PROXY",
		// Case variations
		"http_proxy",
		"https_proxy",
		"no_proxy",
	}

	for _, k := range sensitive {
		if !isSensitiveEnvKey(k) {
			t.Errorf("isSensitiveEnvKey(%q) = false, want true", k)
		}
	}

	safe := []string{
		"NODE_ENV",
		"PORT",
		"LOG_LEVEL",
		"DEBUG",
		"APP_NAME",
		"WORKSPACE",
		"HOME",
		"PATH",
		"LANG",
	}

	for _, k := range safe {
		if isSensitiveEnvKey(k) {
			t.Errorf("isSensitiveEnvKey(%q) = true, want false", k)
		}
	}
}

func TestSSHAgentForwarding(t *testing.T) {
	missing := func(string) bool { return false }
	present := func(string) bool { return true }

	tests := []struct {
		name         string
		host, target string
		inVM         bool
		exists       func(string) bool
		wantFwd      bool
		wantEnv      string
		wantGroups   []string
	}{
		{"agent not set", "", "", false, present, false, "", nil},
		{"linux socket present", "/tmp/agent.sock", "/tmp/ssh-auth.sock", false, present, true, "SSH_AUTH_SOCK=/tmp/ssh-auth.sock", nil},
		{"linux socket missing", "/tmp/agent.sock", "/tmp/ssh-auth.sock", false, missing, false, "", nil},
		{"docker desktop vm socket", "/run/host-services/ssh-auth.sock", "/run/host-services/ssh-auth.sock", true, missing, true, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock", []string{"0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fwd := sshAgentForwarding(tt.host, tt.target, tt.inVM, tt.exists)
			if (fwd != nil) != tt.wantFwd {
				t.Fatalf("sshAgentForwarding() = %+v, want forwarding: %v", fwd, tt.wantFwd)
			}
			if fwd == nil {
				return
			}
			if fwd.Env != tt.wantEnv {
				t.Errorf("Env = %q, want %q", fwd.Env, tt.wantEnv)
			}
			if fwd.Mount.Source != tt.host || fwd.Mount.Target != tt.target || !fwd.Mount.ReadOnly {
				t.Errorf("Mount = %+v, want read-only bind %s -> %s", fwd.Mount, tt.host, tt.target)
			}
			if !slices.Equal(fwd.GroupAdd, tt.wantGroups) {
				t.Errorf("GroupAdd = %v, want %v", fwd.GroupAdd, tt.wantGroups)
			}
		})
	}
}
