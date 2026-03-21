package cmd

import (
	"github.com/grahambrooks/devc/internal/container"
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "down [path]",
		Short: "Stop and remove a container",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager()
			if err != nil {
				return err
			}
			return mgr.Down(getWorkspaceFolder(args), forceFlag)
		},
	}

	cmd.Flags().BoolVar(&forceFlag, "force", false, "remove even with active sessions")

	return cmd
}
