package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/devcutler/lightscale/daemon/policy"
)

type TCPHandler struct {
	Policy      *policy.Holder
	Flows       *policy.FlowTable
	Resolver    *BackendResolver
	DialTimeout time.Duration
}

func NewTCPHandler(p *policy.Holder, ft *policy.FlowTable, r *BackendResolver) *TCPHandler {
	return &TCPHandler{
		Policy:      p,
		Flows:       ft,
		Resolver:    r,
		DialTimeout: 5 * time.Second,
	}
}

func (h *TCPHandler) Handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	srcIP, _, err := splitHostPort(conn.RemoteAddr())
	if err != nil {
		return
	}
	dstIP, dstPort, err := splitHostPort(conn.LocalAddr())
	if err != nil {
		return
	}

	idx := h.Policy.Load()
	decision, user, svc := idx.CheckService(srcIP, dstIP, dstPort, "tcp")
	if decision != policy.Allow || svc == nil || user == nil {
		return
	}

	backendIP, err := h.Resolver.Resolve(ctx, svc.Origin)
	if err != nil {
		return
	}

	dialCtx, cancel := context.WithTimeout(ctx, h.DialTimeout)
	defer cancel()
	var d net.Dialer
	backend, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(backendIP, strconv.Itoa(dstPort)))
	if err != nil {
		h.Resolver.Invalidate(svc.Origin)
		return
	}
	defer backend.Close()

	flowID := h.Flows.Add(policy.Flow{
		SrcUserID:  user.ID,
		ObjectType: "service",
		ObjectID:   svc.ID,
		Port:       dstPort,
		Protocol:   "tcp",
		Close: func() {
			_ = conn.Close()
			_ = backend.Close()
		},
	})
	defer h.Flows.Remove(flowID)

	splice(conn, backend)
}

func splice(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		closeWriter(b)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		closeWriter(a)
	}()
	wg.Wait()
}

func closeWriter(c net.Conn) {
	type closeWriter interface{ CloseWrite() error }
	if cw, ok := c.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

func splitHostPort(addr net.Addr) (string, int, error) {
	if addr == nil {
		return "", 0, errors.New("proxy: nil addr")
	}
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "", 0, fmt.Errorf("proxy: split %s: %w", addr.String(), err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("proxy: parse port %s: %w", portStr, err)
	}
	return host, port, nil
}
