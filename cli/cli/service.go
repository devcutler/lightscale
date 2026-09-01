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
		newServiceCheckCmd(opts),
		newServiceContainersCmd(opts),
	)
	return cmd
}

type originFlags struct {
	container string
	ip        string
	hostname  string
	host      bool
	network   string
}

func (o *originFlags) register(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&o.container, "container", "", "back this service with a container, by name")
	f.StringVar(&o.ip, "ip", "", "back this service with a literal IP address")
	f.StringVar(&o.hostname, "hostname", "", "back this service with a DNS name")
	f.BoolVar(&o.host, "host", false, "back this service with the gateway itself (requires --ports)")
	f.StringVar(&o.network, "network", "", "pin container selection to a named container network")
}

func (o *originFlags) resolve(cmd *cobra.Command) (kind, value string, changed bool, err error) {
	type choice struct {
		kind, value, flag string
	}
	var picked []choice
	if o.host {
		picked = append(picked, choice{"host", "", "--host"})
	}
	if o.container != "" {
		picked = append(picked, choice{"container", o.container, "--container"})
	}
	if o.ip != "" {
		picked = append(picked, choice{"ip", o.ip, "--ip"})
	}
	if o.hostname != "" {
		picked = append(picked, choice{"hostname", o.hostname, "--hostname"})
	}

	switch len(picked) {
	case 0:
		if cmd.Flags().Changed("network") {
			return "", "", false, fmt.Errorf("--network needs a container origin (--container <name>)")
		}
		return "", "", false, nil
	case 1:
		return picked[0].kind, picked[0].value, true, nil
	default:
		names := make([]string, 0, len(picked))
		for _, p := range picked {
			names = append(names, p.flag)
		}
		return "", "", false, fmt.Errorf("only one origin may be given, got %s", strings.Join(names, " and "))
	}
}

func newServiceCreateCmd(opts *Options) *cobra.Command {
	var of originFlags
	var ports, domain, internalIP, description string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a service",
		Long:  "Create a service. Pass one of --container, --ip, --hostname or --host.",
		Example: `  lightscale service create jellyfin --container jellyfin --ports 8096/tcp
  lightscale service create nas --ip 192.168.1.50 --ports 5000/tcp
  lightscale service create git --hostname git.internal --ports 22/tcp
  lightscale service create metrics --host --ports 9100/tcp`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()

			kind, value, changed, err := of.resolve(cmd)
			if err != nil {
				return err
			}
			if !changed {
				return fmt.Errorf("an origin is required: pass one of --container, --ip, --hostname, or --host")
			}

			body := map[string]any{
				"name":         args[0],
				"origin_kind":  kind,
				"origin_value": value,
			}
			if of.network != "" {
				body["origin_network"] = of.network
			}
			if ports != "" {
				body["ports"] = ports
			}
			if domain != "" {
				body["hostname"] = domain
			}
			if internalIP != "" {
				body["ip"] = internalIP
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
			fmt.Fprintf(cmd.OutOrStdout(), "created service %s (id %d, internal ip %s, domain %s, origin %s)\n",
				sv.Name, sv.ID, sv.IPAddress, sv.Hostname, formatOrigin(sv))
			return nil
		},
	}
	of.register(cmd)
	cmd.Flags().StringVar(&ports, "ports", "", "comma-separated, e.g. 8096/tcp,19132/udp")
	registerIdentityFlags(cmd, &domain, &internalIP,
		"the domain users open in a browser; defaults to <name>.<domain>",
		"the service's address; auto-assigned if blank")
	cmd.Flags().StringVar(&description, "description", "", "freeform description")
	return withPreview(cmd, opts, previewServiceCreate)
}

func registerIdentityFlags(cmd *cobra.Command, domain, internalIP *string, domainHelp, ipHelp string) {
	f := cmd.Flags()
	f.StringVar(domain, "domain", "", domainHelp)
	f.StringVar(domain, "internal-hostname", "", domainHelp+" (synonym for --domain)")
	f.StringVar(internalIP, "internal-ip", "", ipHelp)
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
					strconv.FormatInt(sv.ID, 10), sv.Name, sv.IPAddress,
					sv.OriginKind, originValueLabel(sv),
					sv.Hostname, formatPorts(sv.Ports),
				})
			}
			table(cmd.OutOrStdout(), []string{"ID", "NAME", "INTERNAL IP", "KIND", "BACKEND", "DOMAIN", "PORTS"}, rows)
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
			out := cmd.OutOrStdout()
			fmt.Fprintf(out,
				"id            %d\nname          %s\ndomain        %s\ninternal ip   %s\n",
				sv.ID, sv.Name, sv.Hostname, sv.IPAddress)
			fmt.Fprintf(out, "origin kind   %s\norigin value  %s\n",
				sv.OriginKind, originValueLabel(sv))
			if sv.OriginNetwork != "" {
				fmt.Fprintf(out, "origin net    %s\n", sv.OriginNetwork)
			}
			fmt.Fprintf(out, "ports         %s\ndescription   %s\n",
				formatPorts(sv.Ports), sv.Description)
			return nil
		},
	}
}

func newServiceUpdateCmd(opts *Options) *cobra.Command {
	var of originFlags
	var newName, ports, domain, internalIP, description string
	cmd := &cobra.Command{
		Use:     "update <name>",
		Short:   "Update a service",
		Example: "  lightscale service update jellyfin --container jellyfin-new\n  lightscale service update nas --ip 192.168.1.51",
		Args:    cobra.ExactArgs(1),
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
			kind, value, originChanged, err := of.resolve(cmd)
			if err != nil {
				return err
			}
			if originChanged {
				body["origin_kind"] = kind
				body["origin_value"] = value
			}
			if cmd.Flags().Changed("network") {
				body["origin_network"] = of.network
			}
			if cmd.Flags().Changed("ports") {
				body["ports"] = ports
			}
			if cmd.Flags().Changed("domain") || cmd.Flags().Changed("internal-hostname") {
				body["hostname"] = domain
			}
			if cmd.Flags().Changed("internal-ip") {
				body["ip"] = internalIP
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
	of.register(cmd)
	cmd.Flags().StringVar(&newName, "name", "", "new name")
	cmd.Flags().StringVar(&ports, "ports", "", "new ports spec")
	registerIdentityFlags(cmd, &domain, &internalIP,
		"new domain users open in a browser",
		"new address inside the mesh")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	return withPreview(cmd, opts, previewServiceUpdate)
}

func newServiceDeleteCmd(opts *Options) *cobra.Command {
	return withPreview(&cobra.Command{
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
	}, opts, previewServiceDelete)
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

func newServiceContainersCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "containers",
		Short: "List containers available as service origins",
		Long:  "Lists running containers. REACHABLE being true means lightscale shares a network with it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var containers []wire.ContainerSummary
			if err := newClient(opts).Get(ctx, "/api/discover/containers", &containers); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), containers)
			}
			if len(containers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(),
					"no containers visible (is a container runtime socket configured and readable?)")
				return nil
			}
			rows := make([][]string, 0, len(containers))
			for _, c := range containers {
				rows = append(rows, []string{
					c.Name, boolStr(c.Shared), strings.Join(c.Networks, ","),
				})
			}
			table(cmd.OutOrStdout(), []string{"NAME", "REACHABLE", "NETWORKS"}, rows)
			return nil
		},
	}
}

func newServiceCheckCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:     "check <name>",
		Short:   "Test whether a service's backend is reachable right now",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"test"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			sv, err := lookupServiceByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			var res wire.OriginCheck
			if err := newClient(opts).Post(ctx,
				fmt.Sprintf("/api/services/%d/check", sv.ID), map[string]any{}, &res); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), res)
			}
			out := cmd.OutOrStdout()
			if !res.Reachable {
				detail := res.Error
				if detail == "" {
					detail = res.Detail
				}
				fmt.Fprintf(out, "%s: NOT reachable\n  %s\n", sv.Name, detail)
				return nil
			}
			fmt.Fprintf(out, "%s: reachable\n  dials    %s\n  resolved %s\n",
				sv.Name, res.DialHost, res.Detail)
			if res.Network != "" {
				fmt.Fprintf(out, "  network  %s\n", res.Network)
			}
			return nil
		},
	}
}

func originValueLabel(sv serviceJSON) string {
	if sv.OriginKind == "host" {
		return "(this machine)"
	}
	return sv.OriginValue
}

func formatOrigin(sv serviceJSON) string {
	if sv.OriginKind == "host" {
		return "host"
	}
	return fmt.Sprintf("%s:%s", sv.OriginKind, sv.OriginValue)
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
