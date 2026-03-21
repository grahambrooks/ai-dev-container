package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/grahambrooks/devc/pkg/types"
)

func TestContainerName(t *testing.T) {
	name := ContainerName("/home/user/projects/my-app")
	if name == "" {
		t.Fatal("expected non-empty container name")
	}
	if name == ContainerName("/home/user/projects/other-app") {
		t.Fatal("different paths should produce different names")
	}
	// Same path should produce same name
	if name != ContainerName("/home/user/projects/my-app") {
		t.Fatal("same path should produce same name")
	}
}

func TestLoadDevcontainerConfig(t *testing.T) {
	dir := t.TempDir()
	devDir := filepath.Join(dir, ".devcontainer")
	if err := os.MkdirAll(devDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{
		"name":  "test",
		"image": "ubuntu:22.04",
		"customizations": map[string]interface{}{
			"devc": map[string]interface{}{
				"agent":           "claude",
				"securityProfile": "strict",
			},
		},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(devDir, "devcontainer.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadDevcontainerConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Name != "test" {
		t.Errorf("expected name 'test', got %q", loaded.Name)
	}
	if loaded.Image != "ubuntu:22.04" {
		t.Errorf("expected image 'ubuntu:22.04', got %q", loaded.Image)
	}
}

func TestExtractDevcCustomization(t *testing.T) {
	cfg := &types.DevContainerConfig{
		Customizations: map[string]interface{}{
			"devc": map[string]interface{}{
				"agent":           "claude",
				"securityProfile": "strict",
				"resources": map[string]interface{}{
					"cpus":   "2",
					"memory": "4g",
				},
			},
		},
	}

	custom, err := ExtractDevcCustomization(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if custom.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %q", custom.Agent)
	}
	if custom.SecurityProfile != "strict" {
		t.Errorf("expected profile 'strict', got %q", custom.SecurityProfile)
	}
	if custom.Resources == nil || custom.Resources.CPUs != "2" {
		t.Error("expected resources.cpus = '2'")
	}
}

func TestExtractDevcCustomization_NoCustomizations(t *testing.T) {
	cfg := &types.DevContainerConfig{}
	custom, err := ExtractDevcCustomization(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if custom.SecurityProfile != "moderate" {
		t.Errorf("expected default profile 'moderate', got %q", custom.SecurityProfile)
	}
}

func TestMergeCustomization(t *testing.T) {
	global := &types.GlobalConfig{
		Defaults: types.DevcCustomization{
			Agent:           "codex",
			SecurityProfile: "moderate",
			Resources: &types.ResourceConfig{
				CPUs:   "4",
				Memory: "8g",
			},
		},
	}

	project := &types.DevcCustomization{
		Agent: "claude",
		Resources: &types.ResourceConfig{
			CPUs:   "2",
			Memory: "4g",
		},
	}

	merged := MergeCustomization(global, project)
	if merged.Agent != "claude" {
		t.Errorf("expected project agent override, got %q", merged.Agent)
	}
	if merged.SecurityProfile != "moderate" {
		t.Errorf("expected global security profile, got %q", merged.SecurityProfile)
	}
	if merged.Resources.CPUs != "2" {
		t.Errorf("expected project cpus override, got %q", merged.Resources.CPUs)
	}
}

func TestFindImage(t *testing.T) {
	img := FindImage("python")
	if img == nil {
		t.Fatal("expected to find python image")
	}
	if img.Reference == "" {
		t.Error("expected non-empty reference")
	}

	if FindImage("nonexistent") != nil {
		t.Error("expected nil for unknown image")
	}
}

func TestFindImage_AllHaveReferences(t *testing.T) {
	for _, img := range ListImages() {
		if img.Name == "" {
			t.Error("image has empty name")
		}
		if img.Reference == "" {
			t.Errorf("image %q has empty reference", img.Name)
		}
		if img.Description == "" {
			t.Errorf("image %q has empty description", img.Name)
		}
	}
}

func TestSaveDevcontainerConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".devcontainer", "devcontainer.json")

	cfg := &types.DevContainerConfig{
		Name:  "test-save",
		Image: "ubuntu:22.04",
		Features: map[string]interface{}{
			"ghcr.io/devcontainers/features/git:latest": map[string]interface{}{},
		},
	}

	if err := SaveDevcontainerConfig(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was written and can be loaded back
	loaded, err := LoadDevcontainerConfig(dir)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}
	if loaded.Name != "test-save" {
		t.Errorf("expected name 'test-save', got %q", loaded.Name)
	}
	if loaded.Image != "ubuntu:22.04" {
		t.Errorf("expected image 'ubuntu:22.04', got %q", loaded.Image)
	}
	if len(loaded.Features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(loaded.Features))
	}
}

func TestSaveDevcontainerConfig_PreservesExistingFields(t *testing.T) {
	dir := t.TempDir()
	devDir := filepath.Join(dir, ".devcontainer")
	os.MkdirAll(devDir, 0755)
	path := filepath.Join(devDir, "devcontainer.json")

	// Write initial config with an extra field
	initial := `{"name": "test", "image": "old:image", "postCreateCommand": "npm install"}`
	os.WriteFile(path, []byte(initial), 0644)

	// Update just the image
	cfg := &types.DevContainerConfig{
		Name:  "test",
		Image: "new:image",
	}

	if err := SaveDevcontainerConfig(path, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reload raw to check postCreateCommand is preserved
	data, _ := os.ReadFile(path)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if raw["image"] != "new:image" {
		t.Errorf("expected updated image, got %v", raw["image"])
	}
	if raw["postCreateCommand"] != "npm install" {
		t.Errorf("expected preserved postCreateCommand, got %v", raw["postCreateCommand"])
	}
}

func TestWorkspaceInContainer(t *testing.T) {
	cfg := &types.DevContainerConfig{}
	path := WorkspaceInContainer(cfg, "/home/user/my-project")
	if filepath.Base(path) != "my-project" {
		t.Errorf("expected workspace base 'my-project', got %q", filepath.Base(path))
	}

	cfg.WorkspaceFolder = "/custom/path"
	path = WorkspaceInContainer(cfg, "/home/user/my-project")
	if path != "/custom/path" {
		t.Errorf("expected custom path, got %q", path)
	}
}
