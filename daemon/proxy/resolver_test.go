package proxy

import (
	"context"
	"testing"
	"time"
)

func TestResolverHostAndIP(t *testing.T) {
	r := NewResolver(nil)
	got, err := r.Resolve(context.Background(), "host")
	if err != nil || got != "127.0.0.1" {
		t.Fatalf("host: got %q err=%v", got, err)
	}
	got, err = r.Resolve(context.Background(), "192.168.1.50")
	if err != nil || got != "192.168.1.50" {
		t.Fatalf("ip: got %q err=%v", got, err)
	}
}

func TestResolverInvalidate(t *testing.T) {
	r := NewResolver(nil)
	r.cache["foo.example"] = cacheEntry{ip: "1.2.3.4", expires: time.Now().Add(time.Hour)}
	r.Invalidate("foo.example")
	if _, ok := r.cache["foo.example"]; ok {
		t.Fatal("cache entry should be gone")
	}
}
