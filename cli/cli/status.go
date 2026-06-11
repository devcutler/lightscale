package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type statusJSON = wire.StatusSnapshot

func newStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var st statusJSON
			if err := newClient(opts).Get(ctx, "/api/status", &st); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), st)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"running         %v\npeers           %d\nactive_flows    %d\nuptime_sec      %d\nwireguard_udp   %s\nsocket_path     %s\n",
				st.Running, st.Peers, st.ActiveFlows, st.UptimeSec,
				st.WireGuardUDP, st.SocketPath)
			return nil
		},
	}
}
