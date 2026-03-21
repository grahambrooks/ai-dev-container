package cmd

import (
	"github.com/grahambrooks/devc/internal/container"
	"github.com/spf13/cobra"
)

func newAttachCmd() *cobra.Command {
	var shellFlag string

	cmd := &cobra.Command{
		Use:   "attach [path]",
		Short: "Attach an interactive session to a running container",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager(flagDockerPath)
			if err != nil {
				return err
			}
			return mgr.Attach(getWorkspaceFolder(args), shellFlag)
		},
	}

	cmd.Flags().StringVar(&shellFlag, "shell", "/bin/bash", "shell to use")

	return cmd
}
