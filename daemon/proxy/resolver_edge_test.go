package proxy

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

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
			err := safeBackendIP(c.ip)
			if c.wantErr && err == nil {
				t.Fatalf("want error for %q, got nil", c.ip)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("want nil for %q, got %v", c.ip, err)
			}
		})
	}
}

func TestResolveOriginPaths(t *testing.T) {
	r := NewResolver(nil)
	ctx := context.Background()

	if got, err := r.Resolve(ctx, "host"); err != nil || got != "127.0.0.1" {
		t.Fatalf("host: got %q err=%v", got, err)
	}
	if got, err := r.Resolve(ctx, "  host  "); err != nil || got != "127.0.0.1" {
		t.Fatalf("whitespace-trimmed host: got %q err=%v", got, err)
	}

	if got, err := r.Resolve(ctx, ""); err != nil || got != "127.0.0.1" {
		t.Fatalf("empty origin defaults to host: got %q err=%v", got, err)
	}
	if got, err := r.Resolve(ctx, "   "); err != nil || got != "127.0.0.1" {
		t.Fatalf("whitespace origin defaults to host: got %q err=%v", got, err)
	}

	if got, err := r.Resolve(ctx, "10.0.0.9"); err != nil || got != "10.0.0.9" {
		t.Fatalf("ip literal: got %q err=%v", got, err)
	}

	if _, err := r.Resolve(ctx, "127.0.0.1"); err == nil {
		t.Fatal("loopback literal: want rejection")
	}
	if _, err := r.Resolve(ctx, "169.254.169.254"); err == nil {
		t.Fatal("metadata literal: want rejection")
	}
}

func TestResolveIPLiteralNotCached(t *testing.T) {

	r := NewResolver(nil)
	ctx := context.Background()
	if _, err := r.Resolve(ctx, "10.0.0.9"); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	n := len(r.cache)
	r.mu.Unlock()
	if n != 0 {
		t.Fatalf("ip-literal path should not populate cache, got %d entries", n)
	}
}

func TestInvalidateTTLExpiry(t *testing.T) {

	r := NewResolver(nil)
	r.cache["stale.example"] = cacheEntry{ip: "10.1.1.1", expires: time.Now().Add(-time.Minute)}

	r.Invalidate("stale.example")
	r.mu.Lock()
	_, ok := r.cache["stale.example"]
	r.mu.Unlock()
	if ok {
		t.Fatal("entry should be gone after Invalidate")
	}
}

func TestResolverConcurrency(t *testing.T) {
	r := NewResolver(nil)
	ctx := context.Background()
	origins := []string{"10.0.0.1", "10.0.0.2", "192.168.5.5", "172.16.0.9"}

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o := origins[i%len(origins)]
			if got, err := r.Resolve(ctx, o); err != nil || got != o {
				t.Errorf("resolve %s: got %q err=%v", o, got, err)
			}
		}(i)
	}

	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o := origins[i%len(origins)]
			r.mu.Lock()
			r.cache[o] = cacheEntry{ip: o, expires: time.Now().Add(time.Hour)}
			r.mu.Unlock()
			r.Invalidate(o)
		}(i)
	}
	wg.Wait()
}

func TestSafeBackendIPMessageMentionsHost(t *testing.T) {

	err := safeBackendIP("127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("want loopback error mentioning host, got %v", err)
	}
}
