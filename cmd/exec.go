package cmd

import (
	"github.com/grahambrooks/devc/internal/container"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var workspaceFlag string

	cmd := &cobra.Command{
		Use:   "exec [flags] -- <command...>",
		Short: "Execute a command in a running container",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager()
			if err != nil {
				return err
			}
			ws := workspaceFlag
			if ws == "" {
				ws = getWorkspaceFolder(nil)
			}
			return mgr.Exec(ws, args)
		},
	}

	cmd.Flags().StringVar(&workspaceFlag, "workspace-folder", "", "project root (default: cwd)")

	return cmd
}
