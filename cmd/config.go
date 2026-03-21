package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/grahambrooks/devc/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config [path]",
		Short: "Read and display merged configuration",
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

	return cmd
}
