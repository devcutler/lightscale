package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type policyJSON = wire.Policy

func newPolicyCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{Use: "policy", Short: "Manage policy rules"}
	cmd.AddCommand(
		newPolicyAddCmd(opts),
		newPolicyListCmd(opts),
		newPolicyDeleteCmd(opts),
	)
	return cmd
}

func newPolicyAddCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "add <subject> <object> <allow|deny>",
		Short: "Add a policy rule",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			action := args[2]
			if action != "allow" && action != "deny" {
				return fmt.Errorf("action must be 'allow' or 'deny'")
			}
			body := map[string]any{
				"subject_name": args[0],
				"object_name":  args[1],
				"action":       action,
			}
			var p policyJSON
			if err := newClient(opts).Post(ctx, "/api/policies", body, &p); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), p)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"added policy #%d: %s -> %s = %s\n", p.ID, p.SubjectName, p.ObjectName, p.Action)
			return nil
		},
	}
}

func newPolicyListCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List policy rules (ascending id; highest id wins)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var rules []policyJSON
			if err := newClient(opts).Get(ctx, "/api/policies", &rules); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), rules)
			}
			rows := make([][]string, 0, len(rules))
			for _, r := range rules {
				rows = append(rows, []string{
					strconv.FormatInt(r.ID, 10),
					fmt.Sprintf("%s:%s", r.SubjectType, r.SubjectName),
					fmt.Sprintf("%s:%s", r.ObjectType, r.ObjectName),
					r.Action,
				})
			}
			table(cmd.OutOrStdout(), []string{"ID", "SUBJECT", "OBJECT", "ACTION"}, rows)
			return nil
		},
	}
}

func newPolicyDeleteCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a policy rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid id")
			}
			if err := newClient(opts).Delete(ctx, fmt.Sprintf("/api/policies/%d", id)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted policy #%d\n", id)
			return nil
		},
	}
}
