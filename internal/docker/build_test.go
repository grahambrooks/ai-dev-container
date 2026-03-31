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

func TestIsOCIFeature(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"ghcr.io/devcontainers/features/node:1", true},
		{"ghcr.io/grahambrooks/codemap/codemap:latest", true},
		{"node", false},
		{"python", false},
		{"myregistry.azurecr.io/features/tool:1", true},
		{"us-docker.pkg.dev/project/repo/feature:1", true},
	}
	for _, tt := range tests {
		got := isOCIFeature(tt.ref)
		if got != tt.want {
			t.Errorf("isOCIFeature(%q) = %v, want %v", tt.ref, got, tt.want)
		}
	}
}

func TestParseOCIRef(t *testing.T) {
	tests := []struct {
		ref      string
		registry string
		repo     string
		tag      string
	}{
		{"ghcr.io/owner/repo/feature:v1.0", "ghcr.io", "owner/repo/feature", "v1.0"},
		{"ghcr.io/owner/repo/feature:latest", "ghcr.io", "owner/repo/feature", "latest"},
		{"ghcr.io/owner/repo/feature", "ghcr.io", "owner/repo/feature", "latest"},
		{"ghcr.io/grahambrooks/codemap/codemap:2026.3.29", "ghcr.io", "grahambrooks/codemap/codemap", "2026.3.29"},
	}
	for _, tt := range tests {
		registry, repo, tag := parseOCIRef(tt.ref)
		if registry != tt.registry || repo != tt.repo || tag != tt.tag {
			t.Errorf("parseOCIRef(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.ref, registry, repo, tag, tt.registry, tt.repo, tt.tag)
		}
	}
}

func TestOCIFeatureInstallCommand(t *testing.T) {
	cmd := ociFeatureInstallCommand("ghcr.io/grahambrooks/codemap/codemap:2026.3.29", nil)
	if !strings.Contains(cmd, "ghcr.io") {
		t.Error("command should reference ghcr.io registry")
	}
	if !strings.Contains(cmd, "grahambrooks/codemap/codemap") {
		t.Error("command should reference the repository path")
	}
	if !strings.Contains(cmd, "2026.3.29") {
		t.Error("command should reference the tag")
	}
	if !strings.Contains(cmd, "install.sh") {
		t.Error("command should run install.sh")
	}
}

func TestOCIFeatureInstallCommandWithOpts(t *testing.T) {
	opts := map[string]string{"version": "1.2.3"}
	cmd := ociFeatureInstallCommand("ghcr.io/owner/repo/feature:latest", opts)
	if !strings.Contains(cmd, `export VERSION="1.2.3"`) {
		t.Errorf("command should export VERSION, got: %s", cmd)
	}
}

func TestFeatureInstallCommand_OCIFallback(t *testing.T) {
	cmd := featureInstallCommand("ghcr.io/grahambrooks/codemap/codemap:latest", nil)
	if strings.Contains(cmd, "Could not auto-install") {
		t.Error("OCI feature should not fall through to apt-get default")
	}
	if !strings.Contains(cmd, "install.sh") {
		t.Error("OCI feature should run install.sh")
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
