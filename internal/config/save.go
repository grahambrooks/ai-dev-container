package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grahambrooks/devc/pkg/types"
)

// FindDevcontainerPath returns the path to the existing devcontainer.json,
// or the default path if none exists.
func FindDevcontainerPath(workspaceFolder string) string {
	paths := []string{
		filepath.Join(workspaceFolder, ".devcontainer", "devcontainer.json"),
		filepath.Join(workspaceFolder, ".devcontainer.json"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return paths[0] // default location
}

// SaveDevcontainerConfig writes a DevContainerConfig to disk as JSON.
func SaveDevcontainerConfig(path string, cfg *types.DevContainerConfig) error {
	// Read existing raw JSON to preserve fields we don't model
	existing := make(map[string]interface{})
	if data, readErr := os.ReadFile(path); readErr == nil {
		if unmarshalErr := json.Unmarshal(data, &existing); unmarshalErr != nil {
			return fmt.Errorf("parsing existing config: %w", unmarshalErr)
		}
	}

	// Marshal the typed config and merge on top of existing
	typed, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	var updated map[string]interface{}
	if err := json.Unmarshal(typed, &updated); err != nil {
		return err
	}

	// Merge: typed fields overwrite existing, but existing fields not in typed are preserved
	for k, v := range updated {
		if v != nil {
			existing[k] = v
		}
	}

	// Remove nil values that clutter the output, but preserve "features" entries
	// since empty maps are valid there (meaning "use defaults")
	cleanMap(existing, "features")

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}

// cleanMap removes nil values and empty maps from a JSON-like map.
// preserveKeys are map keys whose children should not be pruned when empty.
func cleanMap(m map[string]interface{}, preserveKeys ...string) {
	preserve := make(map[string]bool)
	for _, k := range preserveKeys {
		preserve[k] = true
	}

	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		if sub, ok := v.(map[string]interface{}); ok {
			if preserve[k] {
				continue
			}
			cleanMap(sub)
			if len(sub) == 0 {
				delete(m, k)
			}
		}
	}
}
