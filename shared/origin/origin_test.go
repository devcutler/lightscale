package origin

import (
	"strings"
	"testing"
)

func TestValidateAccepts(t *testing.T) {
	cases := map[string]struct {
		in   Spec
		want Spec
	}{
		"host": {
			Spec{Kind: Host},
			Spec{Kind: Host},
		},
		"container": {
			Spec{Kind: Container, Value: "jellyfin"},
			Spec{Kind: Container, Value: "jellyfin"},
		},
		"container with network": {
			Spec{Kind: Container, Value: "jellyfin", Network: "appnet"},
			Spec{Kind: Container, Value: "jellyfin", Network: "appnet"},
		},
		"ipv4": {
			Spec{Kind: IP, Value: "192.168.1.50"},
			Spec{Kind: IP, Value: "192.168.1.50"},
		},
		"ipv6": {
			Spec{Kind: IP, Value: "fd00::1"},
			Spec{Kind: IP, Value: "fd00::1"},
		},
		"hostname": {
			Spec{Kind: Hostname, Value: "nas.internal"},
			Spec{Kind: Hostname, Value: "nas.internal"},
		},
		"trims whitespace": {
			Spec{Kind: "  container  ", Value: "  jellyfin  "},
			Spec{Kind: Container, Value: "jellyfin"},
		},
		"lowercases the kind": {
			Spec{Kind: "CONTAINER", Value: "jellyfin"},
			Spec{Kind: Container, Value: "jellyfin"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Validate(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]Spec{
		"no kind":                  {},
		"unknown kind":             {Kind: "nonsense", Value: "x"},
		"container without value":  {Kind: Container},
		"hostname without value":   {Kind: Hostname},
		"ip without value":         {Kind: IP},
		"host with a value":        {Kind: Host, Value: "jellyfin"},
		"malformed ip":             {Kind: IP, Value: "999.1.1.1"},
		"loopback ip":              {Kind: IP, Value: "127.0.0.1"},
		"ipv6 loopback":            {Kind: IP, Value: "::1"},
		"link-local ip":            {Kind: IP, Value: "169.254.169.254"},
		"unspecified ip":           {Kind: IP, Value: "0.0.0.0"},
		"multicast ip":             {Kind: IP, Value: "224.0.0.1"},
		"hostname as ip":           {Kind: IP, Value: "nas.internal"},
		"network on ip kind":       {Kind: IP, Value: "10.0.0.1", Network: "appnet"},
		"network on hostname kind": {Kind: Hostname, Value: "a.b", Network: "appnet"},
		"network on host kind":     {Kind: Host, Network: "appnet"},
		"whitespace-only value":    {Kind: Container, Value: "   "},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Validate(in); err == nil {
				t.Fatalf("want rejection for %+v", in)
			}
		})
	}
}

func TestCacheKeyDistinguishes(t *testing.T) {
	specs := []Spec{
		{Kind: Container, Value: "web"},
		{Kind: Hostname, Value: "web"},
		{Kind: IP, Value: "web"},
		{Kind: Container, Value: "web", Network: "a"},
		{Kind: Container, Value: "web", Network: "b"},
		{Kind: Host},
	}
	seen := map[string]Spec{}
	for _, s := range specs {
		k := s.CacheKey()
		if prev, dup := seen[k]; dup {
			t.Fatalf("cache key collision: %+v and %+v", prev, s)
		}
		seen[k] = s
	}
}

func TestStringIsReadable(t *testing.T) {
	cases := map[Spec]string{
		{Kind: Host}:                            "host",
		{Kind: Container, Value: "jellyfin"}:    "container:jellyfin",
		{Kind: IP, Value: "10.0.0.1"}:           "ip:10.0.0.1",
		{Kind: Hostname, Value: "nas.internal"}: "hostname:nas.internal",
	}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", in, got, want)
		}
	}
}

func TestSafeBackendIP(t *testing.T) {
	cases := []struct {
		ip      string
		wantErr bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"169.254.0.1", true},
		{"169.254.169.254", true},
		{"fe80::1", true},
		{"0.0.0.0", true},
		{"::", true},
		{"224.0.0.1", true},
		{"239.255.255.250", true},
		{"ff02::1", true},
		{"not-an-ip", true},
		{"10.0.0.5", false},
		{"172.16.3.4", false},
		{"192.168.1.50", false},
		{"8.8.8.8", false},
		{"2001:4860:4860::8888", false},
	}
	for _, c := range cases {
		t.Run(c.ip, func(t *testing.T) {
			err := SafeBackendIP(c.ip)
			if c.wantErr && err == nil {
				t.Fatalf("want error for %q, got nil", c.ip)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("want nil for %q, got %v", c.ip, err)
			}
		})
	}
}

func TestSafeBackendIPMessageMentionsHost(t *testing.T) {
	err := SafeBackendIP("127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("want loopback error mentioning host kind, got %v", err)
	}
}

func TestValidRejectsUnknownKinds(t *testing.T) {
	if Valid("") || Valid("nonsense") {
		t.Error("Valid() must reject unknown kinds")
	}
}
