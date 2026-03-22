package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ResolvedCredentials holds environment variables to inject for agent auth.
type ResolvedCredentials struct {
	Env []string // KEY=value pairs
}

// ResolveCredentials extracts auth credentials from the host system for
// the given agent profile. On macOS, this reads from Keychain. On Linux,
// it checks for credential files or env vars.
func ResolveCredentials(profile *Profile) *ResolvedCredentials {
	switch profile.Name {
	case "claude":
		return resolveClaudeCredentials()
	default:
		return &ResolvedCredentials{}
	}
}

// resolveClaudeCredentials extracts Claude Code OAuth tokens from the host.
//
// Claude Code stores credentials in:
//   - macOS: Keychain under "Claude Code-credentials" (encrypted with "Claude Safe Storage")
//   - Linux: ~/.claude/credentials (plain JSON, if setup-token was used)
//   - Env vars: CLAUDE_CODE_OAUTH_TOKEN, ANTHROPIC_API_KEY (checked first)
func resolveClaudeCredentials() *ResolvedCredentials {
	creds := &ResolvedCredentials{}

	// If the user already has an API key or OAuth token set, pass it through
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		creds.Env = append(creds.Env, "ANTHROPIC_API_KEY="+key)
		return creds
	}
	if token := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); token != "" {
		creds.Env = append(creds.Env, "CLAUDE_CODE_OAUTH_TOKEN="+token)
		if refresh := os.Getenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN"); refresh != "" {
			creds.Env = append(creds.Env, "CLAUDE_CODE_OAUTH_REFRESH_TOKEN="+refresh)
		}
		return creds
	}

	// Try platform-specific credential stores
	switch runtime.GOOS {
	case "darwin":
		return resolveClaudeCredentialsMacOS()
	case "linux":
		return resolveClaudeCredentialsLinux()
	default:
		return creds
	}
}

// claudeKeychainCredentials is the JSON structure stored in macOS Keychain.
type claudeKeychainCredentials struct {
	ClaudeAIOAuth *claudeOAuthEntry `json:"claudeAiOauth"`
}

type claudeOAuthEntry struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func resolveClaudeCredentialsMacOS() *ResolvedCredentials {
	creds := &ResolvedCredentials{}

	// Extract from macOS Keychain
	out, err := exec.Command("security", "find-generic-password",
		"-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return creds
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return creds
	}

	var keychainData claudeKeychainCredentials
	if err := json.Unmarshal([]byte(raw), &keychainData); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: could not parse Claude keychain credentials: %v\n", err)
		return creds
	}

	if keychainData.ClaudeAIOAuth == nil {
		return creds
	}

	if keychainData.ClaudeAIOAuth.AccessToken != "" {
		creds.Env = append(creds.Env, "CLAUDE_CODE_OAUTH_TOKEN="+keychainData.ClaudeAIOAuth.AccessToken)
	}
	if keychainData.ClaudeAIOAuth.RefreshToken != "" {
		creds.Env = append(creds.Env, "CLAUDE_CODE_OAUTH_REFRESH_TOKEN="+keychainData.ClaudeAIOAuth.RefreshToken)
	}

	return creds
}

func resolveClaudeCredentialsLinux() *ResolvedCredentials {
	creds := &ResolvedCredentials{}

	// On Linux, check for a credentials file (created by `claude setup-token`)
	home, err := os.UserHomeDir()
	if err != nil {
		return creds
	}

	data, err := os.ReadFile(home + "/.claude/credentials")
	if err != nil {
		return creds
	}

	var fileCreds claudeKeychainCredentials
	if err := json.Unmarshal(data, &fileCreds); err != nil {
		return creds
	}

	if fileCreds.ClaudeAIOAuth != nil {
		if fileCreds.ClaudeAIOAuth.AccessToken != "" {
			creds.Env = append(creds.Env, "CLAUDE_CODE_OAUTH_TOKEN="+fileCreds.ClaudeAIOAuth.AccessToken)
		}
		if fileCreds.ClaudeAIOAuth.RefreshToken != "" {
			creds.Env = append(creds.Env, "CLAUDE_CODE_OAUTH_REFRESH_TOKEN="+fileCreds.ClaudeAIOAuth.RefreshToken)
		}
	}

	return creds
}
