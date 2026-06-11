package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type serviceJSON = wire.Service

type portJSON = wire.ServicePort

func newServiceCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "service", Short: "Manage services"}
	cmd.AddCommand(
		newServiceCreateCmd(opts),
		newServiceListCmd(opts),
		newServiceGetCmd(opts),
		newServiceUpdateCmd(opts),
		newServiceDeleteCmd(opts),
	)
	return cmd
}

func newServiceCreateCmd(opts *Options) *cobra.Command {
	var origin, ports, hostname, ip, description string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			body := map[string]any{"name": args[0]}
			if origin != "" {
				body["origin"] = origin
			}
			if ports != "" {
				body["ports"] = ports
			}
			if hostname != "" {
				body["hostname"] = hostname
			}
			if ip != "" {
				body["ip"] = ip
			}
			if description != "" {
				body["description"] = description
			}
			var sv serviceJSON
			if err := newClient(opts).Post(ctx, "/api/services", body, &sv); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), sv)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created service %s (id %d, ip %s, hostname %s)\n",
				sv.Name, sv.ID, sv.IPAddress, sv.Hostname)
			return nil
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "host | container-name | ip | hostname")
	cmd.Flags().StringVar(&ports, "ports", "", "comma-separated, e.g. 8096/tcp,19132/udp")
	cmd.Flags().StringVar(&hostname, "hostname", "", "FQDN; defaults to <name>.<domain>")
	cmd.Flags().StringVar(&ip, "ip", "", "service VIP (optional)")
	cmd.Flags().StringVar(&description, "description", "", "freeform description")
	return cmd
}

func newServiceListCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List services",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var services []serviceJSON
			if err := newClient(opts).Get(ctx, "/api/services", &services); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), services)
			}
			rows := make([][]string, 0, len(services))
			for _, sv := range services {
				rows = append(rows, []string{
					strconv.FormatInt(sv.ID, 10), sv.Name, sv.IPAddress, sv.Origin,
					sv.Hostname, formatPorts(sv.Ports),
				})
			}
			table(cmd.OutOrStdout(), []string{"ID", "NAME", "IP", "ORIGIN", "HOSTNAME", "PORTS"}, rows)
			return nil
		},
	}
}

func newServiceGetCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show one service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			sv, err := lookupServiceByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), sv)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"id            %d\nname          %s\nhostname      %s\norigin        %s\nip            %s\nports         %s\ndescription   %s\n",
				sv.ID, sv.Name, sv.Hostname, sv.Origin, sv.IPAddress, formatPorts(sv.Ports), sv.Description)
			return nil
		},
	}
}

func newServiceUpdateCmd(opts *Options) *cobra.Command {
	var newName, origin, ports, hostname, ip, description string
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			sv, err := lookupServiceByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = newName
			}
			if cmd.Flags().Changed("origin") {
				body["origin"] = origin
			}
			if cmd.Flags().Changed("ports") {
				body["ports"] = ports
			}
			if cmd.Flags().Changed("hostname") {
				body["hostname"] = hostname
			}
			if cmd.Flags().Changed("ip") {
				body["ip"] = ip
			}
			if cmd.Flags().Changed("description") {
				body["description"] = description
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update")
			}
			var out serviceJSON
			if err := newClient(opts).Patch(ctx, fmt.Sprintf("/api/services/%d", sv.ID), body, &out); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated service %s\n", out.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "new name")
	cmd.Flags().StringVar(&origin, "origin", "", "new origin")
	cmd.Flags().StringVar(&ports, "ports", "", "new ports spec")
	cmd.Flags().StringVar(&hostname, "hostname", "", "new hostname")
	cmd.Flags().StringVar(&ip, "ip", "", "new IP")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	return cmd
}

func newServiceDeleteCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			sv, err := lookupServiceByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if err := newClient(opts).Delete(ctx, fmt.Sprintf("/api/services/%d", sv.ID)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted service %s\n", sv.Name)
			return nil
		},
	}
}

func lookupServiceByName(ctx context.Context, opts *Options, name string) (serviceJSON, error) {
	var services []serviceJSON
	if err := newClient(opts).Get(ctx, "/api/services", &services); err != nil {
		return serviceJSON{}, err
	}
	for _, sv := range services {
		if sv.Name == name {
			return sv, nil
		}
	}
	return serviceJSON{}, fmt.Errorf("service %q not found", name)
}

func formatPorts(ports []portJSON) string {
	if len(ports) == 0 {
		return "all"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
	}
	return strings.Join(parts, ",")
}
