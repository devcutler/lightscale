package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type connectionJSON = wire.Connection

func newConnectionsCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:     "connections",
		Aliases: []string{"conns"},
		Short:   "Show active proxy flows",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var flows []connectionJSON
			if err := newClient(opts).Get(ctx, "/api/connections", &flows); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), flows)
			}
			sort.SliceStable(flows, func(i, j int) bool {
				if flows[i].SrcIP != flows[j].SrcIP {
					return ipLess(flows[i].SrcIP, flows[j].SrcIP)
				}
				return ipLess(flows[i].ObjectIP, flows[j].ObjectIP)
			})
			rows := make([][]string, 0, len(flows))
			for _, f := range flows {
				src := nonEmpty(f.SrcName, fmt.Sprintf("user#%d", f.SrcUserID))
				if f.SrcIP != "" {
					src += " (" + f.SrcIP + ")"
				}
				dst := nonEmpty(f.ObjectName, fmt.Sprintf("%s#%d", f.ObjectType, f.ObjectID))
				if f.ObjectIP != "" {
					dst += " (" + f.ObjectIP + ")"
				}
				rows = append(rows, []string{
					fmt.Sprintf("%d", f.ID),
					src,
					f.ObjectType,
					dst,
					fmt.Sprintf("%d/%s", f.Port, f.Protocol),
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no active flows)")
				return nil
			}
			table(cmd.OutOrStdout(),
				[]string{"ID", "FROM", "TYPE", "TO", "PORT/PROTO"}, rows)
			return nil
		},
	}
}
