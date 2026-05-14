package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "hermes %s\ncommit: %s\ndate: %s\n", info.Version, info.Commit, info.Date)
			return nil
		},
	}
}
