package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type previewFn func(ctx context.Context, p *previewer, args []string) ([]string, error)

type previewer struct {
	opts *Options
	cmd  *cobra.Command

	offline bool
	reason  string
}

func (p *previewer) get(ctx context.Context, path string, out any) bool {
	if p.offline {
		return false
	}
	if err := newClient(p.opts).Get(ctx, path, out); err != nil {
		p.offline = true
		p.reason = err.Error()
		return false
	}
	return true
}

func (p *previewer) flag(name string) string {
	f := p.cmd.Flags().Lookup(name)
	if f == nil {
		return ""
	}
	return f.Value.String()
}

func (p *previewer) changed(name string) bool { return p.cmd.Flags().Changed(name) }

func withPreview(cmd *cobra.Command, opts *Options, fn previewFn) *cobra.Command {
	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		if err := runPreview(c, opts, fn, c.Flags().Args()); err != nil {
			c.PrintErrln(err)
		}
	})
	return cmd
}

func runPreview(cmd *cobra.Command, opts *Options, fn previewFn, args []string) error {
	ctx, cancel := signalCtx()
	defer cancel()

	out := cmd.OutOrStdout()
	writeStaticHelp(out, cmd)

	p := &previewer{opts: opts, cmd: cmd}
	lines, err := fn(ctx, p, args)

	fmt.Fprintf(out, "\nWhat this would do\n")
	if err != nil || p.offline {
		reason := p.reason
		if err != nil {
			reason = err.Error()
		}
		fmt.Fprintf(out, "  (could not reach the daemon, so this is the static description only)\n")
		fmt.Fprintf(out, "  %s\n", reason)
		return nil
	}
	for _, l := range lines {
		fmt.Fprintf(out, "  %s\n", l)
	}
	fmt.Fprintf(out, "\nNothing was changed. Re-run without --help to apply.\n")
	return nil
}

func writeStaticHelp(out io.Writer, cmd *cobra.Command) {
	desc := cmd.Long
	if desc == "" {
		desc = cmd.Short
	}
	if desc != "" {
		fmt.Fprintf(out, "%s\n\n", desc)
	}
	fmt.Fprintf(out, "Usage:\n  %s\n", cmd.UseLine())
	if cmd.Example != "" {
		fmt.Fprintf(out, "\nExamples:\n%s\n", cmd.Example)
	}
	if usage := cmd.LocalFlags().FlagUsages(); strings.TrimSpace(usage) != "" {
		fmt.Fprintf(out, "\nFlags:\n%s", usage)
	}
}

func missingArg(args []string, i int, placeholder string) string {
	if i < len(args) && args[i] != "" {
		return args[i]
	}
	return "<" + placeholder + ">"
}
