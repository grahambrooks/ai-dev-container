package cmd

import (
	"github.com/graham/devc/internal/container"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "stop [path]",
		Short: "Stop a running container",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager(flagDockerPath)
			if err != nil {
				return err
			}
			return mgr.Stop(getWorkspaceFolder(args), forceFlag)
		},
	}

	cmd.Flags().BoolVar(&forceFlag, "force", false, "stop even with active sessions")

	return cmd
}
