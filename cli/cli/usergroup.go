package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type userGroupJSON = wire.UserGroup

func newUserGroupCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "user-group", Short: "Manage user groups"}
	cmd.AddCommand(
		newUserGroupCreateCmd(opts),
		newUserGroupListCmd(opts),
		newUserGroupGetCmd(opts),
		newUserGroupUpdateCmd(opts),
		newUserGroupDeleteCmd(opts),
		newUserGroupJoinCmd(opts),
		newUserGroupLeaveCmd(opts),
		newUserGroupMembersCmd(opts),
	)
	return cmd
}

func newUserGroupMembersCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "members <group>",
		Short: "List the users in a group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupUserGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			var users []userJSON
			if err := newClient(opts).Get(ctx, fmt.Sprintf("/api/user-groups/%d/members", g.ID), &users); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), users)
			}
			rows := make([][]string, 0, len(users))
			for _, u := range users {
				rows = append(rows, []string{strconv.FormatInt(u.ID, 10), u.Name, u.IPAddress, u.Email})
			}
			table(cmd.OutOrStdout(), []string{"ID", "NAME", "IP", "EMAIL"}, rows)
			return nil
		},
	}
}

func newUserGroupCreateCmd(opts *Options) *cobra.Command {
	var lanMode bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a user group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			body := map[string]any{"name": args[0], "lan_mode": lanMode}
			var g userGroupJSON
			if err := newClient(opts).Post(ctx, "/api/user-groups", body, &g); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), g)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created user-group %s (lan_mode=%v)\n", g.Name, g.LANMode)
			return nil
		},
	}
	cmd.Flags().BoolVar(&lanMode, "lan-mode", false, "members can reach each other on all ports")
	return cmd
}

func newUserGroupListCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List user groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var groups []userGroupJSON
			if err := newClient(opts).Get(ctx, "/api/user-groups", &groups); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), groups)
			}
			rows := make([][]string, 0, len(groups))
			for _, g := range groups {
				rows = append(rows, []string{strconv.FormatInt(g.ID, 10), g.Name, boolStr(g.LANMode)})
			}
			table(cmd.OutOrStdout(), []string{"ID", "NAME", "LAN_MODE"}, rows)
			return nil
		},
	}
}

func newUserGroupGetCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show one user group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupUserGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), g)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "id        %d\nname      %s\nlan_mode  %v\n", g.ID, g.Name, g.LANMode)
			return nil
		},
	}
}

func newUserGroupUpdateCmd(opts *Options) *cobra.Command {
	var newName string
	var lanMode, noLAN bool
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a user group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupUserGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = newName
			}
			if lanMode && noLAN {
				return fmt.Errorf("--lan-mode and --no-lan-mode are mutually exclusive")
			}
			if lanMode {
				body["lan_mode"] = true
			}
			if noLAN {
				body["lan_mode"] = false
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update")
			}
			var out userGroupJSON
			if err := newClient(opts).Patch(ctx, fmt.Sprintf("/api/user-groups/%d", g.ID), body, &out); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated user-group %s\n", out.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "new name")
	cmd.Flags().BoolVar(&lanMode, "lan-mode", false, "enable LAN mode")
	cmd.Flags().BoolVar(&noLAN, "no-lan-mode", false, "disable LAN mode")
	return cmd
}

func newUserGroupDeleteCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a user group",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupUserGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if err := newClient(opts).Delete(ctx, fmt.Sprintf("/api/user-groups/%d", g.ID)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted user-group %s\n", g.Name)
			return nil
		},
	}
}

func newUserGroupJoinCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "join <group> <user>",
		Short: "Add a user to a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupUserGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			u, err := lookupUserByName(ctx, opts, args[1])
			if err != nil {
				return err
			}
			if err := newClient(opts).Post(ctx,
				fmt.Sprintf("/api/user-groups/%d/members", g.ID),
				map[string]any{"user_id": u.ID}, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s to %s\n", u.Name, g.Name)
			return nil
		},
	}
}

func newUserGroupLeaveCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "leave <group> <user>",
		Short: "Remove a user from a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			g, err := lookupUserGroupByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			u, err := lookupUserByName(ctx, opts, args[1])
			if err != nil {
				return err
			}
			if err := newClient(opts).Delete(ctx,
				fmt.Sprintf("/api/user-groups/%d/members/%d", g.ID, u.ID)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s from %s\n", u.Name, g.Name)
			return nil
		},
	}
}

func lookupUserGroupByName(ctx context.Context, opts *Options, name string) (userGroupJSON, error) {
	var groups []userGroupJSON
	if err := newClient(opts).Get(ctx, "/api/user-groups", &groups); err != nil {
		return userGroupJSON{}, err
	}
	for _, g := range groups {
		if g.Name == name {
			return g, nil
		}
	}
	return userGroupJSON{}, fmt.Errorf("user-group %q not found", name)
}
