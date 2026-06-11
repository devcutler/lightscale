package docker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dockertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

var ErrNotConfigured = errors.New("docker: not configured")

var ErrContainerNotFound = errors.New("docker: container not found")

type Client struct {
	api *client.Client
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

func (c *Client) ContainerIP(ctx context.Context, name string) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	target := strings.TrimPrefix(name, "/")
	containers, err := c.api.ContainerList(ctx, dockertypes.ListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("docker: list containers: %w", err)
	}
	for _, container := range containers {
		for _, n := range container.Names {
			if strings.TrimPrefix(n, "/") == target {
				ip := pickIP(container.NetworkSettings)
				if ip == "" {
					return "", fmt.Errorf("docker: container %q has no IP", name)
				}
				return ip, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", ErrContainerNotFound, name)
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	containers, err := c.api.ContainerList(ctx, dockertypes.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("docker: list containers: %w", err)
	}
	out := make([]ContainerSummary, 0, len(containers))
	for _, container := range containers {
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		out = append(out, ContainerSummary{
			ID:   container.ID,
			Name: name,
			IP:   pickIP(container.NetworkSettings),
		})
	}
	return out, nil
}

type ContainerSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip,omitempty"`
}

func pickIP(ns *dockertypes.NetworkSettingsSummary) string {
	if ns == nil {
		return ""
	}
	if br, ok := ns.Networks["bridge"]; ok && br != nil && br.IPAddress != "" {
		return br.IPAddress
	}
	for _, net := range ns.Networks {
		if net != nil && net.IPAddress != "" {
			return net.IPAddress
		}
	}
	return ""
}
