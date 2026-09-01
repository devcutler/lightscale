package proxy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devcutler/lightscale/daemon/docker"
	"github.com/devcutler/lightscale/shared/origin"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestResolveRejectsInvalidSpecs(t *testing.T) {
	r := testResolver(t)
	ctx := context.Background()
	bad := []origin.Spec{
		{},
		{Kind: "nonsense", Value: "x"},
		{Kind: origin.Container},
		{Kind: origin.Host, Value: "something"},
		{Kind: origin.IP, Value: "999.999.999.999"},
		{Kind: origin.IP, Value: "127.0.0.1"},
		{Kind: origin.IP, Value: "169.254.169.254"},
		{Kind: origin.Hostname, Value: "x", Network: "n"},
	}
	for _, spec := range bad {
		if _, err := r.Resolve(ctx, spec, 80, "tcp"); err == nil {
			t.Errorf("want rejection for %+v", spec)
		}
	}
}

func TestSelectEndpointProbePicksLiveCandidate(t *testing.T) {
	r := testResolver(t)
	r.probe = func(_ context.Context, _, addr string) error {
		if strings.HasPrefix(addr, "10.9.0.4:") {
			return nil
		}
		return errors.New("connection refused")
	}
	eps := []docker.Endpoint{
		{Network: "a", IP: "10.5.0.4"},
		{Network: "b", IP: "10.9.0.4"},
	}
	got, err := r.selectEndpoint(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "web"}, eps, 8080, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.DialHost != "10.9.0.4" || got.Network != "b" {
		t.Fatalf("probe should select the answering endpoint, got %+v", got)
	}
}

func TestSelectEndpointFallsBackWhenNoProbeSucceeds(t *testing.T) {
	r := testResolver(t)
	eps := []docker.Endpoint{
		{Network: "a", IP: "10.5.0.4", Shared: true},
		{Network: "b", IP: "10.9.0.4"},
	}
	got, err := r.selectEndpoint(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "web"}, eps, 8080, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.DialHost != "10.5.0.4" {
		t.Fatalf("want first ranked candidate, got %+v", got)
	}
}

func TestSelectEndpointSkipsProbeForUDP(t *testing.T) {
	r := testResolver(t)
	probed := false
	r.probe = func(context.Context, string, string) error {
		probed = true
		return nil
	}
	eps := []docker.Endpoint{
		{Network: "a", IP: "10.5.0.4"},
		{Network: "b", IP: "10.9.0.4"},
	}
	got, err := r.selectEndpoint(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "web"}, eps, 9001, "udp")
	if err != nil {
		t.Fatal(err)
	}
	if probed {
		t.Error("UDP selection must not probe")
	}
	if got.DialHost != "10.5.0.4" {
		t.Fatalf("want deterministic first candidate, got %+v", got)
	}
}

func TestSelectEndpointSkipsUnsafeAddresses(t *testing.T) {
	r := testResolver(t)
	eps := []docker.Endpoint{
		{Network: "bad", IP: "127.0.0.1"},
		{Network: "ok", IP: "10.9.0.4"},
	}
	got, err := r.selectEndpoint(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "web"}, eps, 0, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if got.DialHost != "10.9.0.4" {
		t.Fatalf("unsafe candidate must be skipped, got %+v", got)
	}
}

func TestSelectEndpointAllUnsafeFails(t *testing.T) {
	r := testResolver(t)
	eps := []docker.Endpoint{{Network: "bad", IP: "127.0.0.1"}}
	if _, err := r.selectEndpoint(context.Background(),
		origin.Spec{Kind: origin.Container, Value: "web"}, eps, 0, "tcp"); !errors.Is(err, ErrOriginUnreachable) {
		t.Fatalf("want ErrOriginUnreachable, got %v", err)
	}
}

func TestResolverConcurrency(t *testing.T) {
	r := testResolver(t)
	r.lookupHost = func(context.Context, string) ([]string, error) {
		return []string{"10.0.0.1"}, nil
	}
	ctx := context.Background()
	specs := []origin.Spec{
		{Kind: origin.IP, Value: "10.0.0.1"},
		{Kind: origin.IP, Value: "10.0.0.2"},
		{Kind: origin.Hostname, Value: "a.example"},
		{Kind: origin.Container, Value: "web"},
		{Kind: origin.Host},
	}

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := specs[i%len(specs)]
			if _, err := r.Resolve(ctx, s, 80, "tcp"); err != nil {
				t.Errorf("resolve %+v: %v", s, err)
			}
		}(i)
	}
	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := specs[i%len(specs)]
			r.store(s, Target{DialHost: "x"}, time.Hour)
			r.Invalidate(s)
		}(i)
	}
	wg.Wait()
}
