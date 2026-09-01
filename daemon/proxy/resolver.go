package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/devcutler/lightscale/daemon/docker"
	"github.com/devcutler/lightscale/shared/origin"
)

var ErrOriginUnreachable = errors.New("proxy: origin unreachable")

type Target struct {
	DialHost string
	Network  string
	Detail   string
}

type nameLookup func(ctx context.Context, host string) ([]string, error)

type dialProbe func(ctx context.Context, network, addr string) error

type BackendResolver struct {
	logger *slog.Logger

	nameTTL     time.Duration
	addrTTL     time.Duration
	hostnameTTL time.Duration

	lookupTimeout time.Duration
	probeTimeout  time.Duration

	lookupHost nameLookup
	probe      dialProbe

	mu     sync.Mutex
	docker *docker.Client
	cache  map[string]cacheEntry
}

type cacheEntry struct {
	target  Target
	expires time.Time
}

func NewResolver(dockerClient *docker.Client, logger *slog.Logger) *BackendResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackendResolver{
		logger:        logger,
		nameTTL:       30 * time.Second,
		addrTTL:       10 * time.Second,
		hostnameTTL:   60 * time.Second,
		lookupTimeout: time.Second,
		probeTimeout:  750 * time.Millisecond,
		lookupHost:    defaultLookupHost,
		probe:         defaultProbe,
		docker:        dockerClient,
		cache:         map[string]cacheEntry{},
	}
}

func defaultLookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

func defaultProbe(ctx context.Context, network, addr string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return err
	}
	return conn.Close()
}

func (r *BackendResolver) Resolve(ctx context.Context, spec origin.Spec, port int, proto string) (Target, error) {
	spec, err := origin.Validate(spec)
	if err != nil {
		r.logger.Warn("proxy: invalid origin", "origin", spec.String(), "err", err)
		return Target{}, fmt.Errorf("%w: %s", ErrOriginUnreachable, err)
	}

	switch spec.Kind {
	case origin.Host:
		return Target{DialHost: "127.0.0.1", Detail: "gateway loopback"}, nil

	case origin.IP:
		if err := origin.SafeBackendIP(spec.Value); err != nil {
			r.logger.Warn("proxy: refused backend ip", "origin", spec.String(), "err", err)
			return Target{}, err
		}
		return Target{DialHost: spec.Value, Detail: "literal address"}, nil

	case origin.Hostname:
		return r.resolveHostname(ctx, spec)

	case origin.Container:
		return r.resolveContainer(ctx, spec, port, proto)
	}

	return Target{}, fmt.Errorf("%w: unhandled kind %q", ErrOriginUnreachable, spec.Kind)
}

func (r *BackendResolver) resolveHostname(ctx context.Context, spec origin.Spec) (Target, error) {
	if t, ok := r.cached(spec); ok {
		return t, nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, r.lookupTimeout)
	defer cancel()
	addrs, err := r.lookupHost(lookupCtx, spec.Value)
	if err != nil || len(addrs) == 0 {
		r.logger.Warn("proxy: hostname did not resolve", "origin", spec.String(), "err", err)
		return Target{}, fmt.Errorf("%w: hostname %q did not resolve: %v", ErrOriginUnreachable, spec.Value, err)
	}
	for _, a := range addrs {
		if err := origin.SafeBackendIP(a); err != nil {
			r.logger.Warn("proxy: refused hostname target", "origin", spec.String(), "addr", a, "err", err)
			return Target{}, err
		}
	}
	t := Target{DialHost: spec.Value, Detail: "dns name"}
	r.store(spec, t, r.hostnameTTL)
	return t, nil
}

func (r *BackendResolver) resolveContainer(ctx context.Context, spec origin.Spec, port int, proto string) (Target, error) {
	if t, ok := r.cached(spec); ok {
		return t, nil
	}

	if spec.Network == "" {
		lookupCtx, cancel := context.WithTimeout(ctx, r.lookupTimeout)
		addrs, err := r.lookupHost(lookupCtx, spec.Value)
		cancel()
		if err == nil && len(addrs) > 0 {
			t := Target{DialHost: spec.Value, Detail: "runtime dns (shared network)"}
			r.store(spec, t, r.nameTTL)
			return t, nil
		}
	}

	r.mu.Lock()
	dockerClient := r.docker
	r.mu.Unlock()

	if dockerClient == nil {
		r.logger.Warn("proxy: cannot resolve container origin",
			"origin", spec.String(),
			"reason", "name does not resolve and no container runtime socket is configured")
		return Target{}, fmt.Errorf(
			"%w: container %q is not resolvable by name and no runtime socket is configured; "+
				"attach the daemon to the container's network, mount the runtime socket, or use an ip/hostname origin",
			ErrOriginUnreachable, spec.Value)
	}

	endpoints, err := dockerClient.ContainerEndpoints(ctx, spec.Value, spec.Network)
	if err != nil {
		r.logger.Warn("proxy: container inspect failed", "origin", spec.String(), "err", err)
		return Target{}, fmt.Errorf("%w: %s", ErrOriginUnreachable, err)
	}

	target, err := r.selectEndpoint(ctx, spec, endpoints, port, proto)
	if err != nil {
		return Target{}, err
	}
	r.store(spec, target, r.addrTTL)
	return target, nil
}

func (r *BackendResolver) selectEndpoint(ctx context.Context, spec origin.Spec, endpoints []docker.Endpoint, port int, proto string) (Target, error) {
	var firstSafe *docker.Endpoint
	var lastErr error

	for i := range endpoints {
		ep := endpoints[i]
		if err := origin.SafeBackendIP(ep.IP); err != nil {
			lastErr = err
			continue
		}
		if firstSafe == nil {
			firstSafe = &endpoints[i]
		}
		if proto != "tcp" || port <= 0 {
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, r.probeTimeout)
		err := r.probe(probeCtx, "tcp", net.JoinHostPort(ep.IP, strconv.Itoa(port)))
		cancel()
		if err == nil {
			return Target{
				DialHost: ep.IP,
				Network:  ep.Network,
				Detail:   "runtime socket, probe confirmed",
			}, nil
		}
		lastErr = err
	}

	if firstSafe == nil {
		r.logger.Warn("proxy: no usable container address", "origin", spec.String(), "err", lastErr)
		return Target{}, fmt.Errorf("%w: container %q exposes no usable address: %v",
			ErrOriginUnreachable, spec.Value, lastErr)
	}

	detail := "runtime socket, deterministic pick"
	if proto == "tcp" && port > 0 {
		detail = "runtime socket, no candidate answered probe"
	}
	return Target{DialHost: firstSafe.IP, Network: firstSafe.Network, Detail: detail}, nil
}

func (r *BackendResolver) cached(spec origin.Spec) (Target, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.cache[spec.CacheKey()]
	if !ok || !time.Now().Before(entry.expires) {
		return Target{}, false
	}
	return entry.target, true
}

func (r *BackendResolver) store(spec origin.Spec, t Target, ttl time.Duration) {
	r.mu.Lock()
	r.cache[spec.CacheKey()] = cacheEntry{target: t, expires: time.Now().Add(ttl)}
	r.mu.Unlock()
}

func (r *BackendResolver) Invalidate(spec origin.Spec) {
	r.mu.Lock()
	delete(r.cache, spec.CacheKey())
	r.mu.Unlock()
}

func (r *BackendResolver) SetDocker(d *docker.Client) {
	r.mu.Lock()
	r.docker = d
	r.cache = map[string]cacheEntry{}
	r.mu.Unlock()
}
