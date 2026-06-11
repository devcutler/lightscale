package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type serviceGroupJSON = wire.ServiceGroup

func newServiceGroupCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "service-group", Short: "Manage service groups"}
	cmd.AddCommand(
		newServiceGroupCreateCmd(opts),
		newServiceGroupListCmd(opts),
		newServiceGroupGetCmd(opts),
		newServiceGroupUpdateCmd(opts),
		newServiceGroupDeleteCmd(opts),
		newServiceGroupJoinCmd(opts),
		newServiceGroupLeaveCmd(opts),
		newServiceGroupMembersCmd(opts),
	)
	return cmd
}

func newServiceGroupMembersCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "members <group>",
		Short: "List the services in a group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupServiceGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			var services []serviceJSON
			if err := newClient(opts).Get(ctx, fmt.Sprintf("/api/service-groups/%d/members", g.ID), &services); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), services)
			}
			rows := make([][]string, 0, len(services))
			for _, sv := range services {
				rows = append(rows, []string{strconv.FormatInt(sv.ID, 10), sv.Name, sv.IPAddress, sv.Origin, sv.Hostname})
			}
			table(cmd.OutOrStdout(), []string{"ID", "NAME", "IP", "ORIGIN", "HOSTNAME"}, rows)
			return nil
		},
	}
}

func newServiceGroupCreateCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a service group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var g serviceGroupJSON
			if err := newClient(opts).Post(ctx, "/api/service-groups",
				map[string]any{"name": args[0]}, &g); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), g)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created service-group %s\n", g.Name)
			return nil
		},
	}
}

func newServiceGroupListCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List service groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var groups []serviceGroupJSON
			if err := newClient(opts).Get(ctx, "/api/service-groups", &groups); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), groups)
			}
			rows := make([][]string, 0, len(groups))
			for _, g := range groups {
				rows = append(rows, []string{strconv.FormatInt(g.ID, 10), g.Name})
			}
			table(cmd.OutOrStdout(), []string{"ID", "NAME"}, rows)
			return nil
		},
	}
}

func newServiceGroupGetCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show one service group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupServiceGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), g)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "id    %d\nname  %s\n", g.ID, g.Name)
			return nil
		},
	}
}

func newServiceGroupUpdateCmd(opts *Options) *cobra.Command {
	var newName string
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a service group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupServiceGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if newName == "" {
				return fmt.Errorf("--name required")
			}
			var out serviceGroupJSON
			if err := newClient(opts).Patch(ctx,
				fmt.Sprintf("/api/service-groups/%d", g.ID),
				map[string]any{"name": newName}, &out); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated service-group %s\n", out.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "new name")
	return cmd
}

func newServiceGroupDeleteCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a service group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupServiceGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if err := newClient(opts).Delete(ctx, fmt.Sprintf("/api/service-groups/%d", g.ID)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted service-group %s\n", g.Name)
			return nil
		},
	}
}

func newServiceGroupJoinCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "join <group> <service>",
		Short: "Add a service to a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupServiceGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			sv, err := lookupServiceByName(ctx, opts, args[1])
			if err != nil {
				return err
			}
			if err := newClient(opts).Post(ctx,
				fmt.Sprintf("/api/service-groups/%d/members", g.ID),
				map[string]any{"service_id": sv.ID}, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s to %s\n", sv.Name, g.Name)
			return nil
		},
	}
}

func newServiceGroupLeaveCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "leave <group> <service>",
		Short: "Remove a service from a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupServiceGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			sv, err := lookupServiceByName(ctx, opts, args[1])
			if err != nil {
				return err
			}
			if err := newClient(opts).Delete(ctx,
				fmt.Sprintf("/api/service-groups/%d/members/%d", g.ID, sv.ID)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s from %s\n", sv.Name, g.Name)
			return nil
		},
	}
}

func lookupServiceGroupByName(ctx context.Context, opts *Options, name string) (serviceGroupJSON, error) {
	var groups []serviceGroupJSON
	if err := newClient(opts).Get(ctx, "/api/service-groups", &groups); err != nil {
		return serviceGroupJSON{}, err
	}
	for _, g := range groups {
		if g.Name == name {
			return g, nil
		}
	}
	return serviceGroupJSON{}, fmt.Errorf("service-group %q not found", name)
}
