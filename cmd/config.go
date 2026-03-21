package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/grahambrooks/devc/internal/agent"
	"github.com/grahambrooks/devc/internal/config"
	"github.com/grahambrooks/devc/pkg/types"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and modify devcontainer configuration",
	}

	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigSetCmd(),
		newConfigAddFeatureCmd(),
		newConfigRemoveFeatureCmd(),
	)

	// Keep backward compat: `devc config [path]` still shows config
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newConfigShowCmd().RunE(cmd, args)
	}
	cmd.Args = cobra.MaximumNArgs(1)

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [path]",
		Short: "Display merged configuration",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getWorkspaceFolder(args)

			devCfg, err := config.LoadDevcontainerConfig(ws)
			if err != nil {
				return err
			}

			globalCfg, err := config.LoadGlobalConfig()
			if err != nil {
				return err
			}

			custom, err := config.ExtractDevcCustomization(devCfg)
			if err != nil {
				return err
			}

			merged := config.MergeCustomization(globalCfg, custom)

			result := map[string]interface{}{
				"devcontainer":   devCfg,
				"devc":           merged,
				"containerName":  config.ContainerName(ws),
				"workspaceMount": config.WorkspaceInContainer(devCfg, ws),
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return fmt.Errorf("encoding config: %w", err)
			}

			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	var (
		imageFlag    string
		agentFlag    string
		securityFlag string
		cpusFlag     string
		memoryFlag   string
		networkFlag  string
	)

	cmd := &cobra.Command{
		Use:   "set [path]",
		Short: "Modify devcontainer.json settings",
		Long: `Modify devcontainer.json settings in place.

Examples:
  devc config set --image python
  devc config set --image myregistry/custom:v2
  devc config set --agent claude --security-profile strict
  devc config set --cpus 8 --memory 16g
  devc config set --network none`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := getWorkspaceFolder(args)
			cfgPath := config.FindDevcontainerPath(ws)

			devCfg, err := config.LoadDevcontainerConfig(ws)
			if err != nil {
				return fmt.Errorf("no devcontainer.json found; run 'devc init' first")
			}

			changed := false

			// Image
			if imageFlag != "" {
				if img := config.FindImage(imageFlag); img != nil {
					devCfg.Image = img.Reference
					fmt.Printf("Image: %s (%s)\n", img.Name, img.Reference)
				} else {
					devCfg.Image = imageFlag
					fmt.Printf("Image: %s\n", imageFlag)
				}
				changed = true
			}

			// Devc customization fields
			custom, err := config.ExtractDevcCustomization(devCfg)
			if err != nil {
				return err
			}

			if agentFlag != "" {
				p := agent.GetProfile(agentFlag)
				if p == nil {
					return fmt.Errorf("unknown agent %q; use 'devc init --list-agents' to see options", agentFlag)
				}

				custom.Agent = agentFlag
				fmt.Printf("Agent: %s (%s)\n", agentFlag, p.DisplayName)

				// Add agent's required features
				if devCfg.Features == nil {
					devCfg.Features = make(map[string]interface{})
				}
				for ref, opts := range p.DevcontainerFeatures() {
					devCfg.Features[ref] = opts
					fmt.Printf("Feature: %s\n", ref)
				}

				// Set install command as postCreateCommand
				if p.InstallCmd != "" {
					devCfg.PostCreateCommand = p.InstallCmd
					fmt.Printf("Install: %s\n", p.InstallCmd)
				}

				// Add agent's environment variables
				if len(p.EnvVars) > 0 {
					if devCfg.ContainerEnv == nil {
						devCfg.ContainerEnv = make(map[string]string)
					}
					for k, v := range p.EnvVars {
						devCfg.ContainerEnv[k] = v
					}
				}

				// Merge network allowlist
				if custom.Network == nil {
					custom.Network = &types.NetworkConfig{Mode: "restricted"}
				}
				existing := make(map[string]bool)
				for _, d := range custom.Network.Allowlist {
					existing[d] = true
				}
				for _, d := range p.NetworkAllow {
					if !existing[d] {
						custom.Network.Allowlist = append(custom.Network.Allowlist, d)
					}
				}

				changed = true
			}
			if securityFlag != "" {
				custom.SecurityProfile = securityFlag
				fmt.Printf("Security profile: %s\n", securityFlag)
				changed = true
			}
			if cpusFlag != "" {
				if custom.Resources == nil {
					custom.Resources = &types.ResourceConfig{}
				}
				custom.Resources.CPUs = cpusFlag
				fmt.Printf("CPUs: %s\n", cpusFlag)
				changed = true
			}
			if memoryFlag != "" {
				if custom.Resources == nil {
					custom.Resources = &types.ResourceConfig{}
				}
				custom.Resources.Memory = memoryFlag
				fmt.Printf("Memory: %s\n", memoryFlag)
				changed = true
			}
			if networkFlag != "" {
				if custom.Network == nil {
					custom.Network = &types.NetworkConfig{}
				}
				custom.Network.Mode = networkFlag
				fmt.Printf("Network: %s\n", networkFlag)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no changes specified; use flags like --image, --agent, --cpus, --memory, --network, --security-profile")
			}

			// Write customization back
			if devCfg.Customizations == nil {
				devCfg.Customizations = make(map[string]interface{})
			}
			customData, err := json.Marshal(custom)
			if err != nil {
				return fmt.Errorf("marshaling customization: %w", err)
			}
			var customMap map[string]interface{}
			if err := json.Unmarshal(customData, &customMap); err != nil {
				return fmt.Errorf("converting customization: %w", err)
			}
			devCfg.Customizations["devc"] = customMap

			if err := config.SaveDevcontainerConfig(cfgPath, devCfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Updated %s\n", cfgPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&imageFlag, "image", "", "base image name or full reference (use 'devc init --list-images')")
	cmd.Flags().StringVar(&agentFlag, "agent", "", "AI agent (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&securityFlag, "security-profile", "", "security preset (strict, moderate, permissive)")
	cmd.Flags().StringVar(&cpusFlag, "cpus", "", "CPU limit (e.g., 4)")
	cmd.Flags().StringVar(&memoryFlag, "memory", "", "memory limit (e.g., 8g)")
	cmd.Flags().StringVar(&networkFlag, "network", "", "network mode (none, restricted, host)")

	return cmd
}

func newConfigAddFeatureCmd() *cobra.Command {
	var versionFlag string

	cmd := &cobra.Command{
		Use:   "add-feature <feature> [path]",
		Short: "Add a Dev Container Feature",
		Long: `Add a Dev Container Feature to devcontainer.json.

The feature argument can be a short name from the official registry or a full
OCI reference. Options can be passed as key=value after the feature name.

Examples:
  devc config add-feature git
  devc config add-feature node --version 20
  devc config add-feature ghcr.io/devcontainers/features/python:1
  devc config add-feature ghcr.io/devcontainers/features/docker-in-docker:2`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature := args[0]
			ws := getWorkspaceFolder(args[1:])
			cfgPath := config.FindDevcontainerPath(ws)

			devCfg, err := config.LoadDevcontainerConfig(ws)
			if err != nil {
				return fmt.Errorf("no devcontainer.json found; run 'devc init' first")
			}

			// Resolve feature reference
			featureRef := resolveFeatureRef(feature)

			// Build options from --version flag
			featureOpts := make(map[string]interface{})
			if versionFlag != "" {
				featureOpts["version"] = versionFlag
			}

			// Add to features map
			if devCfg.Features == nil {
				devCfg.Features = make(map[string]interface{})
			}

			var opts interface{} = featureOpts
			if len(featureOpts) == 0 {
				opts = map[string]interface{}{}
			}
			devCfg.Features[featureRef] = opts

			if err := config.SaveDevcontainerConfig(cfgPath, devCfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Added feature %s\n", featureRef)
			fmt.Printf("Updated %s\n", cfgPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&versionFlag, "version", "", "feature version")

	return cmd
}

func newConfigRemoveFeatureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-feature <feature> [path]",
		Short: "Remove a Dev Container Feature",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature := args[0]
			ws := getWorkspaceFolder(args[1:])
			cfgPath := config.FindDevcontainerPath(ws)

			devCfg, err := config.LoadDevcontainerConfig(ws)
			if err != nil {
				return fmt.Errorf("no devcontainer.json found; run 'devc init' first")
			}

			if devCfg.Features == nil {
				return fmt.Errorf("no features configured")
			}

			featureRef := resolveFeatureRef(feature)

			// Try exact match first, then prefix match
			deleted := false
			for key := range devCfg.Features {
				if key == featureRef || key == feature || strings.Contains(key, "/"+feature+":") || strings.HasSuffix(key, "/"+feature) {
					delete(devCfg.Features, key)
					fmt.Printf("Removed feature %s\n", key)
					deleted = true
					break
				}
			}

			if !deleted {
				return fmt.Errorf("feature %q not found in configuration", feature)
			}

			if err := config.SaveDevcontainerConfig(cfgPath, devCfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Printf("Updated %s\n", cfgPath)
			return nil
		},
	}
}

// resolveFeatureRef expands short feature names to full OCI references.
func resolveFeatureRef(feature string) string {
	// Already a full reference
	if strings.Contains(feature, "/") {
		return feature
	}

	// Well-known official features
	return "ghcr.io/devcontainers/features/" + feature + ":latest"
}
