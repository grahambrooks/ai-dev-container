package config

import (
	"testing"

	"github.com/grahambrooks/devc/pkg/types"
)

func TestConfigHash_Deterministic(t *testing.T) {
	devCfg := &types.DevContainerConfig{Image: "ubuntu:22.04"}
	custom := &types.DevcCustomization{Agent: "claude", SecurityProfile: "moderate"}

	h1 := ConfigHash(devCfg, custom)
	h2 := ConfigHash(devCfg, custom)

	if h1 != h2 {
		t.Errorf("same config should produce same hash: %q != %q", h1, h2)
	}
}

func TestConfigHash_DifferentOnChange(t *testing.T) {
	devCfg := &types.DevContainerConfig{Image: "ubuntu:22.04"}
	custom := &types.DevcCustomization{Agent: "claude", SecurityProfile: "moderate"}

	h1 := ConfigHash(devCfg, custom)

	custom.Agent = "codex"
	h2 := ConfigHash(devCfg, custom)

	if h1 == h2 {
		t.Error("different agent should produce different hash")
	}
}

func TestConfigHash_ImageChange(t *testing.T) {
	custom := &types.DevcCustomization{SecurityProfile: "moderate"}

	h1 := ConfigHash(&types.DevContainerConfig{Image: "ubuntu:22.04"}, custom)
	h2 := ConfigHash(&types.DevContainerConfig{Image: "ubuntu:24.04"}, custom)

	if h1 == h2 {
		t.Error("different image should produce different hash")
	}
}

func TestConfigHash_FeatureChange(t *testing.T) {
	custom := &types.DevcCustomization{SecurityProfile: "moderate"}

	cfg1 := &types.DevContainerConfig{
		Image:    "ubuntu:22.04",
		Features: map[string]interface{}{"ghcr.io/devcontainers/features/node:1": map[string]interface{}{}},
	}
	cfg2 := &types.DevContainerConfig{
		Image: "ubuntu:22.04",
	}

	h1 := ConfigHash(cfg1, custom)
	h2 := ConfigHash(cfg2, custom)

	if h1 == h2 {
		t.Error("adding features should change hash")
	}
}
