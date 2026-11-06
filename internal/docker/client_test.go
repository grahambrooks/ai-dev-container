package docker

import (
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
		wantMount    bool
		wantEnv      string
	}{
		{"agent not set", "", "", false, present, false, ""},
		{"linux socket present", "/tmp/agent.sock", "/tmp/ssh-auth.sock", false, present, true, "SSH_AUTH_SOCK=/tmp/ssh-auth.sock"},
		{"linux socket missing", "/tmp/agent.sock", "/tmp/ssh-auth.sock", false, missing, false, ""},
		{"docker desktop vm socket", "/run/host-services/ssh-auth.sock", "/run/host-services/ssh-auth.sock", true, missing, true, "SSH_AUTH_SOCK=/run/host-services/ssh-auth.sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, env := sshAgentForwarding(tt.host, tt.target, tt.inVM, tt.exists)
			if (m != nil) != tt.wantMount {
				t.Fatalf("mount = %v, want mount: %v", m, tt.wantMount)
			}
			if env != tt.wantEnv {
				t.Errorf("env = %q, want %q", env, tt.wantEnv)
			}
			if m != nil && (m.Source != tt.host || m.Target != tt.target || !m.ReadOnly) {
				t.Errorf("mount = %+v, want read-only bind %s -> %s", *m, tt.host, tt.target)
			}
		})
	}
}
