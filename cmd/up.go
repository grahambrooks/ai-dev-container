package cmd

import (
	"github.com/grahambrooks/devc/internal/container"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var (
		agentFlag    string
		securityFlag string
		detachFlag   bool
		rebuildFlag  bool
	)

	cmd := &cobra.Command{
		Use:   "up [path]",
		Short: "Create and start a development container",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager()
			if err != nil {
				return err
			}
			return mgr.Up(container.UpOptions{
				WorkspaceFolder: getWorkspaceFolder(args),
				Agent:           agentFlag,
				SecurityProfile: securityFlag,
				Detach:          detachFlag,
				Rebuild:         rebuildFlag,
			})
		},
	}

	cmd.Flags().StringVar(&agentFlag, "agent", "", "AI agent profile (claude, codex, gemini, opencode)")
	cmd.Flags().StringVar(&securityFlag, "security-profile", "", "security preset (strict, moderate, permissive)")
	cmd.Flags().BoolVar(&detachFlag, "detach", false, "don't attach after starting")
	cmd.Flags().BoolVar(&rebuildFlag, "rebuild", false, "force rebuild even if container exists")

	return cmd
}
