package cli

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/client"
)

type Options struct {
	SocketPath string
	JSON       bool
}

func New() *cobra.Command {
	opts := &Options{}

	root := &cobra.Command{
		Use:           "lightscale",
		Short:         "Self-hosted wireguard VPN gateway",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&opts.SocketPath, "socket", "", "path to lightscaled UNIX socket (default: from config)")
	root.PersistentFlags().BoolVar(&opts.JSON, "json", false, "emit JSON instead of human-readable tables")

	root.AddCommand(newUserCmd(opts))
	root.AddCommand(newUserGroupCmd(opts))
	root.AddCommand(newServiceCmd(opts))
	root.AddCommand(newServiceGroupCmd(opts))
	root.AddCommand(newPolicyCmd(opts))
	root.AddCommand(newStatusCmd(opts))
	root.AddCommand(newDNSCmd(opts))
	root.AddCommand(newPeersCmd(opts))
	root.AddCommand(newConnectionsCmd(opts))

	return root
}
func newClient(opts *Options) *client.Client {
	return client.New(opts.SocketPath)
}
func signalCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cancel()
	}()
	return ctx, cancel
}
