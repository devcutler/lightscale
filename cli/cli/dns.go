package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDNSCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "dns",
		Short: "Print the BIND-format zone file to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			body, err := newClient(opts).GetText(ctx, "/api/dns")
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), body)
			return nil
		},
	}
}
