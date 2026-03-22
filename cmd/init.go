package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var (
		agentFlag  string
		imageFlag  string
		listImages bool
		listAgents bool
	)

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a devcontainer.json with AI safety defaults",
		Long: `Initialize a devcontainer.json with AI safety defaults.

Use --image to select a base image by name, or --list-images to see
all available images. If --image is not specified, defaults to "base" (Ubuntu).

Use --agent to pre-configure an AI coding agent. This adds the agent's
binary install command, network allowlist entries, and environment
variables. Use --list-agents to see options.

You can also pass a full image reference directly (e.g., --image myregistry/myimage:tag).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listImages {
				fmt.Print("Available images:\n\n")
				fmt.Print(config.FormatImageList())
				return nil
			}

			if listAgents {
				fmt.Print("Available agents:\n\n")
				fmt.Print(agent.FormatProfileList())
				fmt.Println()
				detected := agent.Detect()
				if len(detected) > 0 {
					fmt.Print("Detected on host:")
					for _, d := range detected {
						fmt.Printf(" %s", d.Name)
					}
					fmt.Println()
				}
				return nil
			}

			ws := getWorkspaceFolder(args)
			dir := filepath.Join(ws, ".devcontainer")
			target := filepath.Join(dir, "devcontainer.json")

			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("%s already exists; use 'devc config set' to modify it", target)
			}

			// Resolve image
			imageRef := "mcr.microsoft.com/devcontainers/base:ubuntu"
			if imageFlag != "" {
				if img := config.FindImage(imageFlag); img != nil {
					imageRef = img.Reference
				} else {
					imageRef = imageFlag
				}
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

			cfg := map[string]interface{}{
				"name":  filepath.Base(ws),
				"image": imageRef,
			}

			// Apply agent profiles
			if agentFlag != "" {
				var agentNames []string
				var allAllowlist []string
				var installCmds []string
				var envPass []string
				envSeen := make(map[string]bool)
				allowSeen := make(map[string]bool)

				for _, name := range strings.Split(agentFlag, ",") {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					p := agent.GetProfile(name)
					if p == nil {
						return fmt.Errorf("unknown agent %q; use --list-agents to see options", name)
					}
					agentNames = append(agentNames, name)
					for _, d := range p.NetworkAllow {
						if !allowSeen[d] {
							allowSeen[d] = true
							allAllowlist = append(allAllowlist, d)
						}
					}
					if p.InstallCmd != "" {
						installCmds = append(installCmds, p.InstallCmd)
					}
					for _, e := range p.EnvPassthrough {
						if !envSeen[e] {
							envSeen[e] = true
							envPass = append(envPass, e)
						}
					}
				}

				if len(agentNames) == 1 {
					devcConfig["agent"] = agentNames[0]
				} else {
					devcConfig["agents"] = agentNames
				}
				devcConfig["network"].(map[string]interface{})["allowlist"] = allAllowlist
				if len(installCmds) == 1 {
					cfg["postCreateCommand"] = installCmds[0]
				} else if len(installCmds) > 1 {
					cfg["postCreateCommand"] = strings.Join(installCmds, " && ")
				}
				if len(envPass) > 0 {
					devcConfig["envPassthrough"] = envPass
				}
			}

			cfg["customizations"] = map[string]interface{}{
				"devc": devcConfig,
			}

			data, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}

			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			if err := os.WriteFile(target, append(data, '\n'), 0644); err != nil {
				return err
			}

			fmt.Printf("Created %s\n", target)
			fmt.Printf("Image:  %s\n", imageRef)
			if agentFlag != "" {
				for _, name := range strings.Split(agentFlag, ",") {
					name = strings.TrimSpace(name)
					if p := agent.GetProfile(name); p != nil {
						fmt.Printf("Agent:  %s (%s)\n", name, p.DisplayName)
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&agentFlag, "agent", "", "pre-configure AI agents, comma-separated (use --list-agents to see options)")
	cmd.Flags().StringVar(&imageFlag, "image", "", "base image name or full reference (use --list-images to see options)")
	cmd.Flags().BoolVar(&listImages, "list-images", false, "list available base images")
	cmd.Flags().BoolVar(&listAgents, "list-agents", false, "list available AI agent profiles")

	return cmd
}
