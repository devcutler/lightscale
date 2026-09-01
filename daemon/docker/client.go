package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	dockertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/devcutler/lightscale/shared/wire"
)

var ErrNotConfigured = errors.New("docker: not configured")

var ErrContainerNotFound = errors.New("docker: container not found")

var ErrNoUsableAddress = errors.New("docker: container has no reachable address")

type Client struct {
	api *client.Client

	selfOnce sync.Once
	selfNets map[string]struct{}
}

func New(host string) (*Client, error) {
	if host == "" {
		return nil, ErrNotConfigured
	}
	host = normalizeDockerHost(host)
	api, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker: connect %s: %w", host, err)
	}
	return &Client{api: api}, nil
}

func normalizeDockerHost(h string) string {
	if strings.HasPrefix(h, "/") {
		return "unix://" + h
	}
	return h
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.api.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil {
		return ErrNotConfigured
	}
	if _, err := c.api.Ping(ctx); err != nil {
		return fmt.Errorf("docker: ping: %w", err)
	}
	return nil
}

type Endpoint struct {
	Network string
	IP      string
	Shared  bool
}

func (c *Client) ContainerEndpoints(ctx context.Context, name, preferNetwork string) ([]Endpoint, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	target := strings.TrimPrefix(name, "/")

	containers, err := c.api.ContainerList(ctx, dockertypes.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}

	for _, container := range containers {
		if !matchesName(container.Names, target) {
			continue
		}
		eps := endpointsOf(container.NetworkSettings, c.selfNetworks(ctx))
		if preferNetwork != "" {
			for _, ep := range eps {
				if ep.Network == preferNetwork {
					return []Endpoint{ep}, nil
				}
			}
			return nil, fmt.Errorf("%w: container %q is not attached to network %q",
				ErrNoUsableAddress, name, preferNetwork)
		}
		if len(eps) == 0 {
			return nil, fmt.Errorf("%w: %q (host-networked, or no endpoints)", ErrNoUsableAddress, name)
		}
		return eps, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrContainerNotFound, name)
}

func matchesName(names []string, target string) bool {
	for _, n := range names {
		if strings.TrimPrefix(n, "/") == target {
			return true
		}
	}
	return false
}

func endpointsOf(ns *dockertypes.NetworkSettingsSummary, self map[string]struct{}) []Endpoint {
	if ns == nil {
		return nil
	}
	var out []Endpoint
	for netName, ep := range ns.Networks {
		if ep == nil || ep.IPAddress == "" {
			continue
		}
		_, shared := self[netName]
		out = append(out, Endpoint{Network: netName, IP: ep.IPAddress, Shared: shared})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Shared != out[j].Shared {
			return out[i].Shared
		}
		return out[i].Network < out[j].Network
	})
	return out
}

func (c *Client) selfNetworks(ctx context.Context) map[string]struct{} {
	c.selfOnce.Do(func() {
		c.selfNets = map[string]struct{}{}
		id := selfContainerID()
		if id == "" {
			return
		}
		info, err := c.api.ContainerInspect(ctx, id)
		if err != nil || info.NetworkSettings == nil {
			return
		}
		for netName, ep := range info.NetworkSettings.Networks {
			if ep != nil {
				c.selfNets[netName] = struct{}{}
			}
		}
	})
	return c.selfNets
}

func (c *Client) SelfNetworks(ctx context.Context) []string {
	if c == nil {
		return nil
	}
	nets := c.selfNetworks(ctx)
	out := make([]string, 0, len(nets))
	for n := range nets {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

var cgroupContainerID = regexp.MustCompile(`[0-9a-f]{64}`)

func selfContainerID() string {
	if h, err := os.Hostname(); err == nil && isShortContainerID(h) {
		return h
	}
	for _, path := range []string{"/proc/self/cgroup", "/proc/self/mountinfo"} {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if id := cgroupContainerID.FindString(string(body)); id != "" {
			return id
		}
	}
	return ""
}

func isShortContainerID(s string) bool {
	if len(s) != 12 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	containers, err := c.api.ContainerList(ctx, dockertypes.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}
	self := c.selfNetworks(ctx)
	out := make([]ContainerSummary, 0, len(containers))
	for _, container := range containers {
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		eps := endpointsOf(container.NetworkSettings, self)
		nets := make([]string, 0, len(eps))
		shared := false
		for _, ep := range eps {
			nets = append(nets, ep.Network)
			if ep.Shared {
				shared = true
			}
		}
		out = append(out, ContainerSummary{
			ID:       container.ID,
			Name:     name,
			Networks: nets,
			Shared:   shared,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

type ContainerSummary = wire.ContainerSummary
