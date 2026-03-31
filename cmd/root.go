package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagLogLevel     string
	flagOutputFormat string
)

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:          "devc",
		Short:        "AI-safe development containers",
		Long:         "Create and manage AI-safe development containers with devcontainer.json support.",
		Version:      version,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVar(&flagLogLevel, "log-level", "info", "log level (debug, info, warn, error)")
	root.PersistentFlags().StringVar(&flagOutputFormat, "output-format", "text", "output format (text, json)")

	root.AddCommand(
		newUpCmd(),
		newExecCmd(),
		newAttachCmd(),
		newStopCmd(),
		newDownCmd(),
		newListCmd(),
		newCleanCmd(),
		newConfigCmd(),
		newInitCmd(),
	)

	return root
}

func Execute(version string) {
	if err := NewRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}

func getWorkspaceFolder(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
