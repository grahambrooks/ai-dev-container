package docker

import (
	"strings"
	"testing"
)

func TestExtractFeatureName(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"ghcr.io/devcontainers/features/node:1", "node"},
		{"ghcr.io/devcontainers/features/python:latest", "python"},
		{"ghcr.io/devcontainers/features/docker-in-docker:2", "docker-in-docker"},
		{"ghcr.io/some-org/features/custom:v1", "custom"},
		{"node", "node"},
		{"git", "git"},
	}

	for _, tt := range tests {
		got := extractFeatureName(tt.ref)
		if got != tt.want {
			t.Errorf("extractFeatureName(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestFeatureInstallCommand(t *testing.T) {
	tests := []struct {
		name     string
		opts     map[string]string
		contains string
	}{
		{"node", map[string]string{}, "nodesource"},
		{"node", map[string]string{"version": "20"}, "setup_20.x"},
		{"python", map[string]string{}, "python3"},
		{"git", map[string]string{}, "apt-get install -y git"},
		{"go", map[string]string{}, "go.dev"},
		{"rust", map[string]string{}, "rustup.rs"},
	}

	for _, tt := range tests {
		cmd := featureInstallCommand(tt.name, tt.opts)
		if !strings.Contains(cmd, tt.contains) {
			t.Errorf("featureInstallCommand(%q) = %q, want it to contain %q", tt.name, cmd, tt.contains)
		}
	}
}

func TestGenerateDockerfile(t *testing.T) {
	features := map[string]interface{}{
		"ghcr.io/devcontainers/features/node:1":     map[string]interface{}{"version": "lts"},
		"ghcr.io/devcontainers/features/git:latest": map[string]interface{}{},
	}

	df := generateDockerfile("ubuntu:22.04", features)

	if !strings.HasPrefix(df, "FROM ubuntu:22.04") {
		t.Error("Dockerfile should start with FROM base image")
	}
	if !strings.Contains(df, "USER root") {
		t.Error("Dockerfile should switch to root for installations")
	}
	if !strings.Contains(df, "nodesource") {
		t.Error("Dockerfile should install Node.js")
	}
	if !strings.Contains(df, "apt-get install -y git") {
		t.Error("Dockerfile should install git")
	}
}

func TestBuildTag_Deterministic(t *testing.T) {
	features := map[string]interface{}{
		"ghcr.io/devcontainers/features/node:1": map[string]interface{}{"version": "lts"},
	}

	tag1 := buildTag("ubuntu:22.04", features, "test-container")
	tag2 := buildTag("ubuntu:22.04", features, "test-container")

	if tag1 != tag2 {
		t.Errorf("same inputs should produce same tag: %q != %q", tag1, tag2)
	}

	tag3 := buildTag("ubuntu:24.04", features, "test-container")
	if tag1 == tag3 {
		t.Error("different base images should produce different tags")
	}
}
