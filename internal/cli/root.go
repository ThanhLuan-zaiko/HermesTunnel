package cli

import (
	"context"

	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Execute(ctx context.Context, info BuildInfo) error {
	cmd := NewRootCommand(info)
	return cmd.ExecuteContext(ctx)
}

func NewRootCommand(info BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hermes",
		Short:         "Hermes Tunnel exposes local HTTP services through a public gateway",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newConnectCommand(),
		newServerCommand(),
		newVersionCommand(info),
	)

	return cmd
}
