package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var agentFlag string

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a devcontainer.json with AI safety defaults",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getWorkspaceFolder(args)
			dir := filepath.Join(ws, ".devcontainer")
			target := filepath.Join(dir, "devcontainer.json")

			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("%s already exists", target)
			}

			devcConfig := map[string]interface{}{
				"securityProfile": "moderate",
				"network": map[string]interface{}{
					"mode":      "restricted",
					"allowlist": []string{},
				},
				"resources": map[string]interface{}{
					"cpus":      "4",
					"memory":    "8g",
					"pidsLimit": 256,
				},
				"session": map[string]interface{}{
					"stopOnLastDetach": true,
				},
			}

			if agentFlag != "" {
				devcConfig["agent"] = agentFlag
				if p := agent.GetProfile(agentFlag); p != nil {
					devcConfig["network"].(map[string]interface{})["allowlist"] = p.NetworkAllow
				}
			}

			config := map[string]interface{}{
				"name":  filepath.Base(ws),
				"image": "mcr.microsoft.com/devcontainers/base:ubuntu",
				"customizations": map[string]interface{}{
					"devc": devcConfig,
				},
			}

			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return err
			}

			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			if err := os.WriteFile(target, data, 0644); err != nil {
				return err
			}

			fmt.Printf("Created %s\n", target)
			return nil
		},
	}

	cmd.Flags().StringVar(&agentFlag, "agent", "", "pre-configure for AI agent (claude, codex, gemini, opencode)")

	return cmd
}
