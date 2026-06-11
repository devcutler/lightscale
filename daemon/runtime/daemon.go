package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/devcutler/lightscale/daemon/api"
	"github.com/devcutler/lightscale/daemon/docker"
	"github.com/devcutler/lightscale/daemon/policy"
	"github.com/devcutler/lightscale/daemon/proxy"
	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/daemon/wg"
	"github.com/devcutler/lightscale/shared/config"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger, reloadCh <-chan struct{}, reloadFn func() (config.Config, error)) error {
	if logger == nil {
		logger = slog.Default()
	}

	if err := os.MkdirAll(filepath.Dir(cfg.Storage.Database), 0o750); err != nil {
		return fmt.Errorf("daemon: ensure db dir: %w", err)
	}
	st, err := store.Open(cfg.Storage.Database)
	if err != nil {
		return err
	}
	defer st.Close()

	privKey, _, err := ensureServerKeypair(ctx, st)
	if err != nil {
		return err
	}

	holder := &policy.Holder{}
	idx, err := policy.Build(ctx, st)
	if err != nil {
		return fmt.Errorf("daemon: build policy index: %w", err)
	}
	holder.Store(idx)

	daemonIP, err := netip.ParseAddr(cfg.WireGuard.ServerIP)
	if err != nil {
		return fmt.Errorf("daemon: bad server_ip %q: %w", cfg.WireGuard.ServerIP, err)
	}
	wgServer, _, err := wg.Open(wg.Options{
		Logger:     logger,
		UDPPort:    cfg.WireGuard.Port,
		DaemonIP:   daemonIP,
		PrivateKey: privKey,
	})
	if err != nil {
		return err
	}
	defer wgServer.Close()

	if err := st.SetSetting(ctx, "server_public_key", wgServer.PublicKey()); err != nil {
		return err
	}

	if err := wgServer.ApplyPeers(idx); err != nil {
		return err
	}

	relay, err := wg.NewRelay(wgServer, holder, cfg.WireGuard.ClientSubnet, cfg.WireGuard.ServerIP)
	if err != nil {
		return err
	}
	wgServer.SetRelay(relay.Handle)

	dockerClient, dockerErr := docker.New(cfg.Docker.Socket)
	if dockerErr != nil && !errors.Is(dockerErr, docker.ErrNotConfigured) {
		logger.Warn("daemon: docker disabled", "err", dockerErr)
		dockerClient = nil
	}
	defer func() {
		if dockerClient != nil {
			_ = dockerClient.Close()
		}
	}()

	resolver := proxy.NewResolver(dockerClient)
	flows := policy.NewFlowTable()
	tcpHandler := proxy.NewTCPHandler(holder, flows, resolver)
	udpHandler := proxy.NewUDPHandler(holder, flows, resolver)

	listeners := newListenerManager(logger, wgServer, holder, tcpHandler, udpHandler)
	if err := listeners.reconcile(ctx, idx); err != nil {
		return err
	}
	defer listeners.closeAll()

	stat := &statusTracker{
		startedAt: time.Now(),
		wgServer:  wgServer,
		flows:     flows,
		cfg:       &cfg,
	}

	st.Subscribe(func(kind store.ChangeKind) {
		newIdx, err := policy.Build(context.Background(), st)
		if err != nil {
			logger.Error("daemon: rebuild index", "err", err)
			return
		}
		holder.Store(newIdx)
		flows.Reap(newIdx)

		_ = kind
		if err := wgServer.ApplyPeers(newIdx); err != nil {
			logger.Error("daemon: apply peers", "err", err)
		}
		if err := listeners.reconcile(context.Background(), newIdx); err != nil {
			logger.Error("daemon: reconcile listeners", "err", err)
		}
	})

	srv := api.New(api.Deps{
		Store:    st,
		Config:   &cfg,
		Docker:   dockerClient,
		Status:   stat,
		Peers:    &peersAdapter{srv: wgServer},
		Flows:    &flowsAdapter{flows: flows},
		Resolver: &resolverAdapter{holder: holder},
		Now:      time.Now,
	})

	if cfg.Socket.Path == "" {
		return fmt.Errorf("daemon: no socket path configured; set [socket] path in lightscale.toml")
	}
	httpErr := make(chan error, 1)
	_ = os.Remove(cfg.Socket.Path)

	if err := ensureSocketDir(filepath.Dir(cfg.Socket.Path), cfg.Socket); err != nil {
		return fmt.Errorf("daemon: socket dir: %w", err)
	}
	ln, err := net.Listen("unix", cfg.Socket.Path)
	if err != nil {
		return fmt.Errorf("daemon: listen unix %s: %w", cfg.Socket.Path, err)
	}

	if err := applySocketPerms(cfg.Socket); err != nil {
		ln.Close()
		return fmt.Errorf("daemon: socket perms (refusing to serve on an unsecured socket): %w", err)
	}

	authLn, err := newPeerCredListener(ln, cfg.Socket)
	if err != nil {
		ln.Close()
		return fmt.Errorf("daemon: socket auth: %w", err)
	}
	go func() {
		httpErr <- http.Serve(authLn, srv.Handler())
	}()
	defer authLn.Close()
	logger.Info("daemon: serving on unix socket",
		"path", cfg.Socket.Path,
		"mode", cfg.Socket.Mode,
		"group", cfg.Socket.Group)

	logger.Info("daemon: ready",
		"udp_port", cfg.WireGuard.Port,
		"socket", cfg.Socket.Path,
		"public_endpoint", cfg.PublicEndpoint,
	)

	if cfg.PublicEndpoint == "" {
		logger.Warn("public_endpoint is unset; user configs cannot be rendered until set in lightscale.toml")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-httpErr:
			return err
		case <-reloadCh:
			if reloadFn == nil {
				continue
			}
			fresh, err := reloadFn()
			if err != nil {
				logger.Error("daemon: reload config", "err", err)
				continue
			}
			if fresh.Docker.Socket != cfg.Docker.Socket {
				newDocker, derr := docker.New(fresh.Docker.Socket)
				if derr != nil && !errors.Is(derr, docker.ErrNotConfigured) {
					logger.Warn("config: docker reload failed; keeping previous client", "err", derr)
				} else {
					if errors.Is(derr, docker.ErrNotConfigured) {
						newDocker = nil
					}
					old := dockerClient
					dockerClient = newDocker
					resolver.SetDocker(newDocker)
					srv.SetDocker(newDocker)
					if old != nil {
						_ = old.Close()
					}
				}
			}
			applyHotReload(logger, &cfg, fresh, stat)
		}
	}
}

func applyHotReload(logger *slog.Logger, current *config.Config, fresh config.Config, stat *statusTracker) {
	if fresh.WireGuard.Port != current.WireGuard.Port ||
		fresh.Socket.Path != current.Socket.Path ||
		fresh.Storage.Database != current.Storage.Database {
		logger.Warn("config: listener/storage settings can only change on restart")
	}
	current.PublicEndpoint = fresh.PublicEndpoint
	current.Domain = fresh.Domain
	current.Docker.Socket = fresh.Docker.Socket
	current.Log.Level = fresh.Log.Level
	current.Log.Format = fresh.Log.Format
	logger.Info("config: hot-reload applied",
		"public_endpoint", current.PublicEndpoint,
		"domain", current.Domain)
}

func ensureServerKeypair(ctx context.Context, st *store.Store) (priv, pub string, err error) {
	priv, err = st.GetSetting(ctx, "server_private_key")
	if err == nil {
		pub, err = st.GetSetting(ctx, "server_public_key")
		if err == nil {
			return priv, pub, nil
		}
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return "", "", err
	}

	priv, pub, err = wg.GenerateKeypair()
	if err != nil {
		return "", "", err
	}
	if err := st.SetSetting(ctx, "server_private_key", priv); err != nil {
		return "", "", err
	}
	if err := st.SetSetting(ctx, "server_public_key", pub); err != nil {
		return "", "", err
	}
	return priv, pub, nil
}

type statusTracker struct {
	startedAt time.Time
	wgServer  *wg.Server
	flows     *policy.FlowTable
	cfg       *config.Config
}

func (s *statusTracker) Snapshot() api.StatusSnapshot {
	return api.StatusSnapshot{
		Running:      true,
		Peers:        s.wgServer.PeerCount(),
		ActiveFlows:  s.flows.Len(),
		StartedAt:    s.startedAt,
		UptimeSec:    int64(time.Since(s.startedAt).Seconds()),
		WireGuardUDP: fmt.Sprintf("0.0.0.0:%d", s.cfg.WireGuard.Port),
		SocketPath:   s.cfg.Socket.Path,
	}
}

func ensureSocketDir(dir string, s config.SocketConfig) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	if err := os.Chmod(dir, 0o750); err != nil {
		return fmt.Errorf("chmod dir %s: %w", dir, err)
	}
	if s.Group != "" {
		gid, err := lookupGID(s.Group)
		if err != nil {
			return err
		}
		if err := os.Chown(dir, -1, gid); err != nil {
			return fmt.Errorf("chown dir %s: %w", dir, err)
		}
	}
	return nil
}

func applySocketPerms(s config.SocketConfig) error {
	if s.Path == "" {
		return nil
	}
	mode := os.FileMode(0o660)
	if s.Mode != "" {
		parsed, err := strconv.ParseUint(s.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("parse mode %q: %w", s.Mode, err)
		}
		mode = os.FileMode(parsed)
	}
	if err := os.Chmod(s.Path, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", s.Path, err)
	}
	if s.Group != "" {
		gid, err := lookupGID(s.Group)
		if err != nil {
			return err
		}
		if err := os.Chown(s.Path, -1, gid); err != nil {
			return fmt.Errorf("chown %s: %w", s.Path, err)
		}
	}
	return nil
}
func lookupGID(group string) (int, error) {
	grp, err := user.LookupGroup(group)
	if err != nil {
		return 0, fmt.Errorf("lookup group %q: %w", group, err)
	}
	gid, err := strconv.Atoi(grp.Gid)
	if err != nil {
		return 0, fmt.Errorf("parse gid %q: %w", grp.Gid, err)
	}
	return gid, nil
}

type peersAdapter struct{ srv *wg.Server }

func (a *peersAdapter) PeerStatus() ([]api.PeerStatus, error) {
	raw, err := a.srv.PeerStatus()
	if err != nil {
		return nil, err
	}
	out := make([]api.PeerStatus, len(raw))
	for i, p := range raw {
		out[i] = api.PeerStatus{
			PublicKey:         p.PublicKey,
			PresharedKey:      p.PresharedKey,
			AllowedIPs:        p.AllowedIPs,
			Endpoint:          p.Endpoint,
			LastHandshake:     p.LastHandshake,
			KeepaliveInterval: p.KeepaliveInterval,
			RxBytes:           p.RxBytes,
			TxBytes:           p.TxBytes,
		}
	}
	return out, nil
}

type flowsAdapter struct{ flows *policy.FlowTable }

func (a *flowsAdapter) Snapshot() []api.FlowSnapshot {
	raw := a.flows.Snapshot()
	out := make([]api.FlowSnapshot, len(raw))
	for i, f := range raw {
		out[i] = api.FlowSnapshot{
			ID:         f.ID,
			SrcUserID:  f.SrcUserID,
			ObjectType: f.ObjectType,
			ObjectID:   f.ObjectID,
			Port:       f.Port,
			Protocol:   f.Protocol,
		}
	}
	return out
}

type resolverAdapter struct{ holder *policy.Holder }

func (a *resolverAdapter) Users() []api.UserBrief {
	idx := a.holder.Load()
	out := make([]api.UserBrief, 0, len(idx.UserByID))
	for _, u := range idx.UserByID {
		out = append(out, api.UserBrief{ID: u.ID, Name: u.Name, IP: u.IPAddress, PublicKey: u.PublicKey})
	}
	return out
}

func (a *resolverAdapter) UserByID(id int64) (api.UserBrief, bool) {
	idx := a.holder.Load()
	u, ok := idx.UserByID[id]
	if !ok {
		return api.UserBrief{}, false
	}
	return api.UserBrief{ID: u.ID, Name: u.Name, IP: u.IPAddress, PublicKey: u.PublicKey}, true
}

func (a *resolverAdapter) ServiceByID(id int64) (api.ServiceBrief, bool) {
	idx := a.holder.Load()
	sv, ok := idx.ServiceByID[id]
	if !ok {
		return api.ServiceBrief{}, false
	}
	return api.ServiceBrief{ID: sv.ID, Name: sv.Name, IP: sv.IPAddress}, true
}
