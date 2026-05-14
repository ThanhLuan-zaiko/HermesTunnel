package cli

import (
	"fmt"

	"hermes-tunnel/internal/client"

	"github.com/spf13/cobra"
)

func newConnectCommand() *cobra.Command {
	cfg := client.Config{
		ServerAddr: "127.0.0.1:8081",
		LocalURL:   "http://localhost:3000",
	}

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect a local HTTP service to a Hermes Tunnel server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Logf = func(format string, args ...any) {
				fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
			}

			tunnelClient, err := client.New(cfg)
			if err != nil {
				return err
			}

			return tunnelClient.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(&cfg.Name, "name", "", "public tunnel name, for example app")
	cmd.Flags().StringVar(&cfg.ServerAddr, "server", cfg.ServerAddr, "Hermes control server address")
	cmd.Flags().StringVar(&cfg.LocalURL, "local", cfg.LocalURL, "local HTTP service URL")
	cmd.Flags().StringVar(&cfg.Token, "token", "", "shared token required by the server")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}
