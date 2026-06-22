package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/web"
)

const defaultWebPort = "11687"

func newServeCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "serve <bind>",
		Short: "Serve the web UI, proxying API calls to the daemon socket",
		Long: "Serve the lightscale web UI on the given bind address " +
			"(e.g. `lightscale serve 192.168.2.125:11687` or `lightscale serve 127.0.0.1:11687`). " +
			"A bare port binds all interfaces; if no port is given, " + defaultWebPort + " is used.\n\n" +
			"WARNING: the web UI proxies the daemon control socket with NO authentication. " +
			"Anyone who can reach the bind address gets full admin control of the VPN, the same " +
			"as socket access. This should NEVER be exposed to the wider internet.",
		Example: "  lightscale serve 127.0.0.1:11687\n" +
			"  lightscale serve :11687",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bind := normalizeBind(args[0])

			c := newClient(opts)
			handler, err := web.Handler(c.SocketPath())
			if err != nil {
				return fmt.Errorf("build web handler: %w", err)
			}

			ln, err := net.Listen("tcp", bind)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", bind, err)
			}

			srv := &http.Server{
				Handler:           handler,
				ReadHeaderTimeout: 10 * time.Second,
				IdleTimeout:       120 * time.Second,
			}

			ctx, cancel := signalCtx()
			defer cancel()

			go func() {
				<-ctx.Done()
				shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
				defer c()
				_ = srv.Shutdown(shutdownCtx)
			}()

			fmt.Fprintf(cmd.OutOrStdout(), "lightscale web UI listening on http://%s (socket %s)\n", ln.Addr(), c.SocketPath())

			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
}

func normalizeBind(arg string) string {
	if _, _, err := net.SplitHostPort(arg); err == nil {
		return arg
	}
	if isAllDigits(arg) {
		return ":" + arg
	}
	return arg + ":" + defaultWebPort
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	return strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) == -1
}
