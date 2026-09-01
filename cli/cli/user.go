package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type userJSON = wire.User

func newUserCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "Manage users"}
	cmd.AddCommand(
		newUserCreateCmd(opts),
		newUserListCmd(opts),
		newUserGetCmd(opts),
		newUserUpdateCmd(opts),
		newUserDeleteCmd(opts),
		newUserConfigCmd(opts),
	)
	return cmd
}

func newUserCreateCmd(opts *Options) *cobra.Command {
	var email, ip, endpoint string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			body := map[string]any{"name": args[0]}
			if email != "" {
				body["email"] = email
			}
			if ip != "" {
				body["ip"] = ip
			}
			if endpoint != "" {
				body["endpoint"] = endpoint
			}
			var u userJSON
			if err := newClient(opts).Post(ctx, "/api/users", body, &u); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), u)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created user %s (id %d, ip %s)\n", u.Name, u.ID, u.IPAddress)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "email address")
	cmd.Flags().StringVar(&ip, "ip", "", "specific IP (optional)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "per-user public endpoint override")
	return withPreview(cmd, opts, previewUserCreate)
}

func newUserListCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var users []userJSON
			if err := newClient(opts).Get(ctx, "/api/users", &users); err != nil {
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

func newUserGetCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Show one user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			u, err := lookupUserByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), u)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"id            %d\nname          %s\nemail         %s\nip            %s\nendpoint      %s\npublic_key    %s\n",
				u.ID, u.Name, u.Email, u.IPAddress, u.Endpoint, u.PublicKey)
			return nil
		},
	}
}

func newUserUpdateCmd(opts *Options) *cobra.Command {
	var newName, email, endpoint string
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			u, err := lookupUserByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = newName
			}
			if cmd.Flags().Changed("email") {
				body["email"] = email
			}
			if cmd.Flags().Changed("endpoint") {
				body["endpoint"] = endpoint
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update")
			}
			var out userJSON
			if err := newClient(opts).Patch(ctx, fmt.Sprintf("/api/users/%d", u.ID), body, &out); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated user %s\n", out.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "new name")
	cmd.Flags().StringVar(&email, "email", "", "new email")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "new endpoint override")
	return withPreview(cmd, opts, previewUserUpdate)
}

func newUserDeleteCmd(opts *Options) *cobra.Command {
	return withPreview(&cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			u, err := lookupUserByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			if err := newClient(opts).Delete(ctx, fmt.Sprintf("/api/users/%d", u.ID)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted user %s\n", u.Name)
			return nil
		},
	}, opts, previewUserDelete)
}

func newUserConfigCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "config <name>",
		Short: "Print the user's wireguard .conf to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			u, err := lookupUserByName(ctx, opts, args[0])
			if err != nil {
				return err
			}
			body, err := newClient(opts).GetText(ctx, fmt.Sprintf("/api/users/%d/config", u.ID))
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), body)
			return nil
		},
	}
}

func lookupUserByName(ctx context.Context, opts *Options, name string) (userJSON, error) {
	var users []userJSON
	if err := newClient(opts).Get(ctx, "/api/users", &users); err != nil {
		return userJSON{}, err
	}
	for _, u := range users {
		if u.Name == name {
			return u, nil
		}
	}
	return userJSON{}, fmt.Errorf("user %q not found", name)
}
