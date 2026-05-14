package cli

import (
	"fmt"

	"hermes-tunnel/internal/gateway"

	"github.com/spf13/cobra"
)

func newServerCommand() *cobra.Command {
	cfg := gateway.Config{
		PublicAddr:  ":8080",
		ControlAddr: ":8081",
	}

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve"},
		Short:   "Run the public Hermes Tunnel gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Logf = func(format string, args ...any) {
				fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
			}

			server, err := gateway.New(cfg)
			if err != nil {
				return err
			}

			return server.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&cfg.PublicAddr, "public", cfg.PublicAddr, "public HTTP listen address")
	cmd.Flags().StringVar(&cfg.ControlAddr, "control", cfg.ControlAddr, "client control listen address")
	cmd.Flags().StringVar(&cfg.Token, "token", "", "shared token required from clients")
	cmd.Flags().Int64Var(&cfg.MaxBodyBytes, "max-body-bytes", 10<<20, "maximum request or response body size")

	return cmd
}
