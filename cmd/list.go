package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/grahambrooks/devc/internal/container"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List managed containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr, err := container.NewManager()
			if err != nil {
				return err
			}
			defer mgr.Close()

			containers, err := mgr.List()
			if err != nil {
				return err
			}

			if flagOutputFormat == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(containers)
			}

			if len(containers) == 0 {
				fmt.Println("No managed containers found")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tSTATE\tIMAGE\tAGENT\tSESSIONS\tWORKSPACE")
			for _, c := range containers {
				agent := c.Agent
				if agent == "" {
					agent = "-"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					c.Name, c.State, c.Image, agent, c.Sessions, c.WorkspaceFolder)
			}
			return w.Flush()
		},
	}

	return cmd
}
