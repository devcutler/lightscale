package docker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	dockertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

func TestNewBlankHostReturnsNotConfigured(t *testing.T) {
	c, err := New("")
	if c != nil {
		t.Fatalf("expected nil client, got %v", c)
	}
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestNewConstructsClient(t *testing.T) {
	hosts := []string{
		"/var/run/docker.sock",
		"unix:///var/run/docker.sock",
		"tcp://127.0.0.1:2375",
	}
	for _, h := range hosts {
		t.Run(h, func(t *testing.T) {
			c, err := New(h)
			if err != nil {
				t.Fatalf("New(%q) unexpected error: %v", h, err)
			}
			if c == nil || c.api == nil {
				t.Fatalf("New(%q) returned nil/empty client", h)
			}
			if err := c.Close(); err != nil {
				t.Fatalf("Close() error: %v", err)
			}
		})
	}
}

func TestCloseNilClientDoesNotPanic(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatalf("Close() on nil *Client: %v", err)
	}
}

func TestNormalizeDockerHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"/path", "unix:///path"},
		{"unix:///x", "unix:///x"},
		{"tcp://x", "tcp://x"},
		{"npipe://x", "npipe://x"},
		{"npipe:////./pipe/docker_engine", "npipe:////./pipe/docker_engine"},
	}
	for _, tc := range cases {
		if got := normalizeDockerHost(tc.in); got != tc.want {
			t.Errorf("normalizeDockerHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func ns(networks map[string]*network.EndpointSettings) *dockertypes.NetworkSettingsSummary {
	return &dockertypes.NetworkSettingsSummary{Networks: networks}
}

func TestPickIP(t *testing.T) {
	cases := []struct {
		name string
		in   *dockertypes.NetworkSettingsSummary
		want string
	}{
		{"nil settings", nil, ""},
		{
			"bridge preferred",
			ns(map[string]*network.EndpointSettings{
				"bridge": {IPAddress: "172.17.0.2"},
				"other":  {IPAddress: "10.0.0.5"},
			}),
			"172.17.0.2",
		},
		{
			"single non-bridge network",
			ns(map[string]*network.EndpointSettings{
				"mynet": {IPAddress: "10.0.0.9"},
			}),
			"10.0.0.9",
		},
		{
			"bridge empty falls back to other",
			ns(map[string]*network.EndpointSettings{
				"bridge": {IPAddress: ""},
				"other":  {IPAddress: "10.0.0.7"},
			}),
			"10.0.0.7",
		},
		{
			"all empty",
			ns(map[string]*network.EndpointSettings{
				"bridge": {IPAddress: ""},
				"other":  {IPAddress: ""},
			}),
			"",
		},
		{
			"nil entries",
			ns(map[string]*network.EndpointSettings{
				"bridge": nil,
				"other":  nil,
			}),
			"",
		},
		{
			"empty networks map",
			ns(map[string]*network.EndpointSettings{}),
			"",
		},
		{
			"nil networks map",
			ns(nil),
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickIP(tc.in); got != tc.want {
				t.Errorf("pickIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContainerIPNilGuard(t *testing.T) {
	var nilClient *Client
	if _, err := nilClient.ContainerIP(context.Background(), "x"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil *Client: expected ErrNotConfigured, got %v", err)
	}
}

func TestListContainersNilGuard(t *testing.T) {
	var nilClient *Client
	if _, err := nilClient.ListContainers(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil *Client: expected ErrNotConfigured, got %v", err)
	}
}

func TestContainerSummaryJSONShape(t *testing.T) {
	b, err := json.Marshal(ContainerSummary{ID: "abc", Name: "web", IP: "172.17.0.2"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "name", "ip"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing json key %q in %s", k, b)
		}
	}
	if m["id"] != "abc" || m["name"] != "web" || m["ip"] != "172.17.0.2" {
		t.Errorf("unexpected values: %s", b)
	}

	b2, err := json.Marshal(ContainerSummary{ID: "abc", Name: "web"})
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]any
	if err := json.Unmarshal(b2, &m2); err != nil {
		t.Fatal(err)
	}
	if _, ok := m2["ip"]; ok {
		t.Errorf("expected ip omitted when empty, got %s", b2)
	}
	for _, k := range []string{"id", "name"} {
		if _, ok := m2[k]; !ok {
			t.Errorf("expected key %q present (no omitempty), got %s", k, b2)
		}
	}
}
