package proxy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devcutler/lightscale/shared/origin"
)

func testResolver(t *testing.T) *BackendResolver {
	t.Helper()
	r := NewResolver(nil, discardLogger())
	r.lookupHost = func(context.Context, string) ([]string, error) {
		return nil, errors.New("no such host")
	}
	r.probe = func(context.Context, string, string) error {
		return errors.New("refused")
	}
	return r
}

func TestResolveHost(t *testing.T) {
	r := testResolver(t)
	got, err := r.Resolve(context.Background(), origin.Spec{Kind: origin.Host}, 0, "tcp")
	if err != nil || got.DialHost != "127.0.0.1" {
		t.Fatalf("host: got %+v err=%v", got, err)
	}
}

func TestResolveIPLiteral(t *testing.T) {
	r := testResolver(t)
	got, err := r.Resolve(context.Background(), origin.Spec{Kind: origin.IP, Value: "192.168.1.50"}, 0, "tcp")
	if err != nil || got.DialHost != "192.168.1.50" {
		t.Fatalf("ip: got %+v err=%v", got, err)
	}
}

func TestResolveIPLiteralNotCached(t *testing.T) {
	r := testResolver(t)
	if _, err := r.Resolve(context.Background(), origin.Spec{Kind: origin.IP, Value: "10.0.0.9"}, 0, "tcp"); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	n := len(r.cache)
	r.mu.Unlock()
	if n != 0 {
		t.Fatalf("ip-literal path should not populate cache, got %d entries", n)
	}
}

func TestResolveContainerPrefersName(t *testing.T) {
	r := testResolver(t)
	r.lookupHost = func(_ context.Context, host string) ([]string, error) {
		if host != "jellyfin" {
			t.Fatalf("looked up %q, want jellyfin", host)
		}
		return []string{"10.5.0.4"}, nil
	}
	got, err := r.Resolve(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "jellyfin"}, 8096, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.DialHost != "jellyfin" {
		t.Fatalf("want the name back so runtime DNS resolves it, got %q", got.DialHost)
	}
}

func TestResolveContainerWithoutNameOrSocketFails(t *testing.T) {
	r := testResolver(t)
	_, err := r.Resolve(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "jellyfin"}, 8096, "tcp")
	if !errors.Is(err, ErrOriginUnreachable) {
		t.Fatalf("want ErrOriginUnreachable, got %v", err)
	}
}

func TestResolveHostnameRejectsForbiddenTarget(t *testing.T) {
	r := testResolver(t)
	r.lookupHost = func(context.Context, string) ([]string, error) {
		return []string{"169.254.169.254"}, nil
	}
	if _, err := r.Resolve(context.Background(),
		origin.Spec{Kind: origin.Hostname, Value: "metadata.evil"}, 80, "tcp"); err == nil {
		t.Fatal("want rejection of a hostname resolving to link-local")
	}
}

func TestResolveHostnameReturnsName(t *testing.T) {
	r := testResolver(t)
	r.lookupHost = func(context.Context, string) ([]string, error) {
		return []string{"192.168.1.9"}, nil
	}
	got, err := r.Resolve(context.Background(),
		origin.Spec{Kind: origin.Hostname, Value: "nas.internal"}, 5000, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.DialHost != "nas.internal" {
		t.Fatalf("hostname should be dialed by name, got %q", got.DialHost)
	}
}

func TestInvalidateDropsCachedTarget(t *testing.T) {
	r := testResolver(t)
	spec := origin.Spec{Kind: origin.Hostname, Value: "foo.example"}
	r.store(spec, Target{DialHost: "foo.example"}, time.Hour)
	r.Invalidate(spec)
	if _, ok := r.cached(spec); ok {
		t.Fatal("cache entry should be gone")
	}
}

func TestCacheEntryExpires(t *testing.T) {
	r := testResolver(t)
	spec := origin.Spec{Kind: origin.Hostname, Value: "stale.example"}
	r.store(spec, Target{DialHost: "stale.example"}, -time.Minute)
	if _, ok := r.cached(spec); ok {
		t.Fatal("expired entry must not be served")
	}
}

func TestCacheKeysAreDistinctPerKindAndNetwork(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range []origin.Spec{
		{Kind: origin.Container, Value: "web"},
		{Kind: origin.Hostname, Value: "web"},
		{Kind: origin.Container, Value: "web", Network: "appnet"},
		{Kind: origin.Container, Value: "web", Network: "other"},
	} {
		k := s.CacheKey()
		if seen[k] {
			t.Fatalf("cache key collision on %+v", s)
		}
		seen[k] = true
	}
}
