package cmd

import (
	"fmt"

	"github.com/grahambrooks/devc/internal/container"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var dryRunFlag bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove all stopped managed containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager()
			if err != nil {
				return err
			}
			defer mgr.Close()

			removed, err := mgr.Clean(dryRunFlag)
			if err != nil {
				return err
			}

			if len(removed) == 0 {
				fmt.Println("No stopped containers to clean")
				return nil
			}

			verb := "Removed"
			if dryRunFlag {
				verb = "Would remove"
			}
			for _, name := range removed {
				fmt.Printf("%s %s\n", verb, name)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "show what would be removed")

	return cmd
}
