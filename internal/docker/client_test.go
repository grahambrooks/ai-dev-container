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
