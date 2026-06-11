package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/devcutler/lightscale/daemon/docker"
)

type BackendResolver struct {
	docker *docker.Client

	containerTTL time.Duration

	hostnameTTL time.Duration

	mu    sync.Mutex
	cache map[string]cacheEntry
}
type cacheEntry struct {
	ip      string
	expires time.Time
}

func NewResolver(dockerClient *docker.Client) *BackendResolver {
	return &BackendResolver{
		docker:       dockerClient,
		containerTTL: 30 * time.Second,
		hostnameTTL:  60 * time.Second,
		cache:        map[string]cacheEntry{},
	}
}

func (r *BackendResolver) Resolve(ctx context.Context, origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" || origin == "host" {
		return "127.0.0.1", nil
	}
	if _, err := netip.ParseAddr(origin); err == nil {
		if err := safeBackendIP(origin); err != nil {
			return "", err
		}
		return origin, nil
	}

	r.mu.Lock()
	if entry, ok := r.cache[origin]; ok && time.Now().Before(entry.expires) {
		r.mu.Unlock()
		return entry.ip, nil
	}
	r.mu.Unlock()

	ip, ttl, err := r.lookup(ctx, origin)
	if err != nil {
		return "", err
	}

	if err := safeBackendIP(ip); err != nil {
		return "", err
	}
	r.mu.Lock()
	r.cache[origin] = cacheEntry{ip: ip, expires: time.Now().Add(ttl)}
	r.mu.Unlock()
	return ip, nil
}

func safeBackendIP(ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("proxy: unparseable backend ip %q: %w", ip, err)
	}
	switch {
	case addr.IsLoopback():
		return fmt.Errorf("proxy: refusing loopback backend %s (use origin \"host\" to target the gateway intentionally)", ip)
	case addr.IsLinkLocalUnicast(), addr.IsLinkLocalMulticast():
		return fmt.Errorf("proxy: refusing link-local backend %s", ip)
	case addr.IsUnspecified():
		return fmt.Errorf("proxy: refusing unspecified backend %s", ip)
	case addr.IsMulticast():
		return fmt.Errorf("proxy: refusing multicast backend %s", ip)
	}
	return nil
}

func (r *BackendResolver) Invalidate(origin string) {
	r.mu.Lock()
	delete(r.cache, origin)
	r.mu.Unlock()
}

func (r *BackendResolver) SetDocker(d *docker.Client) {
	r.mu.Lock()
	r.docker = d
	r.cache = map[string]cacheEntry{}
	r.mu.Unlock()
}

func (r *BackendResolver) lookup(ctx context.Context, origin string) (string, time.Duration, error) {
	r.mu.Lock()
	dockerClient := r.docker
	r.mu.Unlock()
	if dockerClient != nil {
		ip, err := dockerClient.ContainerIP(ctx, origin)
		if err == nil {
			return ip, r.containerTTL, nil
		}
		if !errors.Is(err, docker.ErrContainerNotFound) && !errors.Is(err, docker.ErrNotConfigured) {
			return "", 0, fmt.Errorf("proxy: docker resolve %s: %w", origin, err)
		}
	}
	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", origin)
	if err != nil || len(addrs) == 0 {
		return "", 0, fmt.Errorf("proxy: resolve %s: %w", origin, err)
	}
	return addrs[0].String(), r.hostnameTTL, nil
}
