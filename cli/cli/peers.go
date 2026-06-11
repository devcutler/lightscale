package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/devcutler/lightscale/shared/wire"
)

type peerJSON = wire.Peer

func newPeersCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "peers",
		Short: "Show live wireguard peer state (handshake, transfer, endpoint)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signalCtx()
			defer cancel()
			var peers []peerJSON
			if err := newClient(opts).Get(ctx, "/api/peers", &peers); err != nil {
				return err
			}
			if opts.JSON {
				return emitJSON(cmd.OutOrStdout(), peers)
			}
			sort.SliceStable(peers, func(i, j int) bool {
				return ipLess(peers[i].IPAddress, peers[j].IPAddress)
			})
			rows := make([][]string, 0, len(peers))
			for _, p := range peers {
				rows = append(rows, []string{
					nonEmpty(p.Name, shortKey(p.PublicKey)),
					nonEmpty(p.IPAddress, "---"),
					formatHandshake(p.LastHandshake, p.LastHandshakeAgoS),
					formatBytes(p.RxBytes) + " ↓ / " + formatBytes(p.TxBytes) + " ↑",
					nonEmpty(p.Endpoint, "---"),
				})
			}
			table(cmd.OutOrStdout(),
				[]string{"NAME", "VPN_IP", "HANDSHAKE", "TRANSFER", "ENDPOINT"}, rows)
			return nil
		},
	}
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func shortKey(b64 string) string {
	if len(b64) <= 10 {
		return b64
	}
	return b64[:6] + "…" + b64[len(b64)-4:]
}

func formatHandshake(t time.Time, agoSec int64) string {
	if t.IsZero() {
		return "(never)"
	}
	if agoSec < 0 {
		agoSec = 0
	}
	switch {
	case agoSec < 60:
		return fmt.Sprintf("%ds ago", agoSec)
	case agoSec < 3600:
		return fmt.Sprintf("%dm %ds ago", agoSec/60, agoSec%60)
	case agoSec < 86400:
		return fmt.Sprintf("%dh %dm ago", agoSec/3600, (agoSec%3600)/60)
	default:
		return fmt.Sprintf("%dd %dh ago", agoSec/86400, (agoSec%86400)/3600)
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KiB", "MiB", "GiB", "TiB"}[exp]
	return fmt.Sprintf("%.2f %s", float64(b)/float64(div), suffix)
}
