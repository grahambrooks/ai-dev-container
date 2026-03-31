package agent

import (
	"os"
	"testing"
)

func TestResolveCredentials_NilResolveCreds(t *testing.T) {
	p := &Profile{Name: "test"}
	creds := ResolveCredentials(p)
	if creds == nil {
		t.Fatal("ResolveCredentials returned nil")
	}
	if len(creds.Env) != 0 {
		t.Errorf("expected empty Env, got %v", creds.Env)
	}
}

func TestResolveCredentials_WithResolveCreds(t *testing.T) {
	p := &Profile{
		Name: "test",
		ResolveCreds: func() *ResolvedCredentials {
			return &ResolvedCredentials{Env: []string{"TOKEN=abc"}}
		},
	}
	creds := ResolveCredentials(p)
	if len(creds.Env) != 1 || creds.Env[0] != "TOKEN=abc" {
		t.Errorf("expected [TOKEN=abc], got %v", creds.Env)
	}
}

func TestResolveGitHubCredentials_GHToken(t *testing.T) {
	clearGitHubEnv(t)
	t.Setenv("GH_TOKEN", "ghp_test123")

	creds := ResolveGitHubCredentials()
	if len(creds.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(creds.Env))
	}
	if creds.Env[0] != "GH_TOKEN=ghp_test123" {
		t.Errorf("got %q, want GH_TOKEN=ghp_test123", creds.Env[0])
	}
}

func TestResolveGitHubCredentials_GitHubToken(t *testing.T) {
	clearGitHubEnv(t)
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")

	creds := ResolveGitHubCredentials()
	if len(creds.Env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(creds.Env))
	}
	// Should be normalized to GH_TOKEN
	if creds.Env[0] != "GH_TOKEN=ghp_fallback" {
		t.Errorf("got %q, want GH_TOKEN=ghp_fallback", creds.Env[0])
	}
}

func TestResolveGitHubCredentials_GHTokenPrecedence(t *testing.T) {
	clearGitHubEnv(t)
	t.Setenv("GH_TOKEN", "primary")
	t.Setenv("GITHUB_TOKEN", "fallback")

	creds := ResolveGitHubCredentials()
	if len(creds.Env) != 1 || creds.Env[0] != "GH_TOKEN=primary" {
		t.Errorf("GH_TOKEN should take precedence, got %v", creds.Env)
	}
}

func TestResolveClaudeCredentials_APIKey(t *testing.T) {
	clearClaudeEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	creds := ResolveClaudeCredentials()
	if len(creds.Env) != 1 || creds.Env[0] != "ANTHROPIC_API_KEY=sk-ant-test" {
		t.Errorf("got %v, want [ANTHROPIC_API_KEY=sk-ant-test]", creds.Env)
	}
}

func TestResolveClaudeCredentials_OAuthToken(t *testing.T) {
	clearClaudeEnv(t)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-tok")

	creds := ResolveClaudeCredentials()
	if len(creds.Env) != 1 || creds.Env[0] != "CLAUDE_CODE_OAUTH_TOKEN=oauth-tok" {
		t.Errorf("got %v, want [CLAUDE_CODE_OAUTH_TOKEN=oauth-tok]", creds.Env)
	}
}

func TestResolveClaudeCredentials_OAuthWithRefresh(t *testing.T) {
	clearClaudeEnv(t)
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-tok")
	t.Setenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "refresh-tok")

	creds := ResolveClaudeCredentials()
	if len(creds.Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d: %v", len(creds.Env), creds.Env)
	}
	envMap := envToMap(creds.Env)
	if envMap["CLAUDE_CODE_OAUTH_TOKEN"] != "oauth-tok" {
		t.Errorf("missing CLAUDE_CODE_OAUTH_TOKEN")
	}
	if envMap["CLAUDE_CODE_OAUTH_REFRESH_TOKEN"] != "refresh-tok" {
		t.Errorf("missing CLAUDE_CODE_OAUTH_REFRESH_TOKEN")
	}
}

func TestResolveClaudeCredentials_APIKeyPrecedence(t *testing.T) {
	clearClaudeEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-tok")

	creds := ResolveClaudeCredentials()
	if len(creds.Env) != 1 || creds.Env[0] != "ANTHROPIC_API_KEY=sk-ant-test" {
		t.Errorf("ANTHROPIC_API_KEY should take precedence, got %v", creds.Env)
	}
}

func TestResolveClaudeCredentials_NoCredentials(t *testing.T) {
	clearClaudeEnv(t)

	creds := ResolveClaudeCredentials()
	// May have keychain creds on macOS, but should not panic
	if creds == nil {
		t.Fatal("should not return nil")
	}
}

func TestResolveGitHubCredentials_NoCredentials(t *testing.T) {
	clearGitHubEnv(t)

	creds := ResolveGitHubCredentials()
	if creds == nil {
		t.Fatal("should not return nil")
	}
}

func clearGitHubEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v, ok := os.LookupEnv(key); ok {
			t.Setenv(key, v) // register for cleanup
		}
		os.Unsetenv(key)
	}
}

func clearClaudeEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_OAUTH_REFRESH_TOKEN"} {
		if v, ok := os.LookupEnv(key); ok {
			t.Setenv(key, v)
		}
		os.Unsetenv(key)
	}
}

func envToMap(env []string) map[string]string {
	m := make(map[string]string)
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				m[e[:i]] = e[i+1:]
				break
			}
		}
	}
	return m
}
