package docker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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

func names(eps []Endpoint) []string {
	out := make([]string, 0, len(eps))
	for _, e := range eps {
		out = append(out, e.Network+"="+e.IP)
	}
	return out
}

func TestEndpointsOfOrdersSharedNetworksFirst(t *testing.T) {
	self := map[string]struct{}{"appnet": {}}
	got := endpointsOf(ns(map[string]*network.EndpointSettings{
		"bridge": {IPAddress: "172.17.0.2"},
		"appnet": {IPAddress: "10.5.0.9"},
		"zzznet": {IPAddress: "10.9.0.9"},
	}), self)

	if len(got) != 3 || got[0].Network != "appnet" {
		t.Fatalf("shared network must rank first, got %v", names(got))
	}
	if !got[0].Shared || got[1].Shared || got[2].Shared {
		t.Fatalf("Shared flags wrong: %+v", got)
	}
	if got[1].Network != "bridge" || got[2].Network != "zzznet" {
		t.Fatalf("non-shared candidates must sort by name, got %v", names(got))
	}
}

func TestEndpointsOfIsDeterministic(t *testing.T) {
	in := ns(map[string]*network.EndpointSettings{
		"a": {IPAddress: "10.0.0.1"},
		"b": {IPAddress: "10.0.0.2"},
		"c": {IPAddress: "10.0.0.3"},
		"d": {IPAddress: "10.0.0.4"},
	})
	want := names(endpointsOf(in, nil))
	for i := range 50 {
		if got := names(endpointsOf(in, nil)); !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d differed: got %v want %v", i, got, want)
		}
	}
}

func TestEndpointsOfSkipsEmptyAndNil(t *testing.T) {
	cases := []struct {
		name string
		in   *dockertypes.NetworkSettingsSummary
		want int
	}{
		{"nil settings", nil, 0},
		{"nil networks map", ns(nil), 0},
		{"empty networks map", ns(map[string]*network.EndpointSettings{}), 0},
		{"nil entries", ns(map[string]*network.EndpointSettings{"a": nil, "b": nil}), 0},
		{"all empty IPs", ns(map[string]*network.EndpointSettings{
			"a": {IPAddress: ""}, "b": {IPAddress: ""},
		}), 0},
		{"one usable among empties", ns(map[string]*network.EndpointSettings{
			"a": {IPAddress: ""}, "b": {IPAddress: "10.0.0.7"}, "c": nil,
		}), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := endpointsOf(tc.in, nil); len(got) != tc.want {
				t.Errorf("endpointsOf() = %v, want %d entries", names(got), tc.want)
			}
		})
	}
}

func TestIsShortContainerID(t *testing.T) {
	cases := map[string]bool{
		"3f2a1b9c8d7e":  true,
		"3F2A1B9C8D7E":  false, // docker uses lowercase hex
		"myhostname12":  false,
		"3f2a1b9c8d7":   false, // 11 chars
		"3f2a1b9c8d7e0": false,
		"":              false,
	}
	for in, want := range cases {
		if got := isShortContainerID(in); got != want {
			t.Errorf("isShortContainerID(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestContainerEndpointsNilGuard(t *testing.T) {
	var nilClient *Client
	if _, err := nilClient.ContainerEndpoints(context.Background(), "x", ""); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("nil *Client: expected ErrNotConfigured, got %v", err)
	}
}

func TestPingNilGuard(t *testing.T) {
	var nilClient *Client
	if err := nilClient.Ping(context.Background()); !errors.Is(err, ErrNotConfigured) {
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
	b, err := json.Marshal(ContainerSummary{
		ID: "abc", Name: "web", Networks: []string{"appnet", "bridge"}, Shared: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "name", "networks", "shared"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing json key %q in %s", k, b)
		}
	}
	if m["id"] != "abc" || m["name"] != "web" || m["shared"] != true {
		t.Errorf("unexpected values: %s", b)
	}

	if _, ok := m["ip"]; ok {
		t.Errorf("summary must not expose an ip field: %s", b)
	}

	b2, err := json.Marshal(ContainerSummary{ID: "abc", Name: "web"})
	if err != nil {
		t.Fatal(err)
	}
	var m2 map[string]any
	if err := json.Unmarshal(b2, &m2); err != nil {
		t.Fatal(err)
	}
	if _, ok := m2["networks"]; ok {
		t.Errorf("expected networks omitted when empty, got %s", b2)
	}
	for _, k := range []string{"id", "name", "shared"} {
		if _, ok := m2[k]; !ok {
			t.Errorf("expected key %q present (no omitempty), got %s", k, b2)
		}
	}
}
