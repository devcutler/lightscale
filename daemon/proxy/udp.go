package proxy

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/devcutler/lightscale/daemon/policy"
)

type UDPHandler struct {
	Policy      *policy.Holder
	Flows       *policy.FlowTable
	Resolver    *BackendResolver
	Logger      *slog.Logger
	IdleTimeout time.Duration

	mu    sync.Mutex
	flows map[udpKey]*udpFlow
}

type udpKey struct {
	srcIP   string
	srcPort int
	dstVIP  string
	dstPort int
}

type udpFlow struct {
	out      *net.UDPConn
	lastSeen time.Time
	cancel   context.CancelFunc
	flowID   uint64
}

func NewUDPHandler(p *policy.Holder, ft *policy.FlowTable, r *BackendResolver, logger *slog.Logger) *UDPHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &UDPHandler{
		Policy:      p,
		Flows:       ft,
		Resolver:    r,
		Logger:      logger,
		IdleTimeout: 60 * time.Second,
		flows:       map[udpKey]*udpFlow{},
	}
}

func (h *UDPHandler) HandlePacket(ctx context.Context, inbound net.PacketConn, srcAddr net.Addr, dstVIP string, dstPort int, data []byte) {
	srcIP, srcPort, err := splitHostPort(srcAddr)
	if err != nil {
		return
	}

	idx := h.Policy.Load()
	decision, user, svc := idx.CheckService(srcIP, dstVIP, dstPort, "udp")
	if decision != policy.Allow || svc == nil || user == nil {
		return
	}

	key := udpKey{srcIP: srcIP, srcPort: srcPort, dstVIP: dstVIP, dstPort: dstPort}

	h.mu.Lock()
	flow, ok := h.flows[key]
	if !ok {
		target, err := h.Resolver.Resolve(ctx, svc.Origin, dstPort, "udp")
		if err != nil {
			h.mu.Unlock()
			h.Logger.Warn("proxy: udp resolve failed",
				"service", svc.Name, "origin", svc.Origin.String(), "port", dstPort, "err", err)
			return
		}
		backend := net.JoinHostPort(target.DialHost, strconv.Itoa(dstPort))
		raddr, err := net.ResolveUDPAddr("udp", backend)
		if err != nil {
			h.mu.Unlock()
			h.Logger.Warn("proxy: udp address resolve failed",
				"service", svc.Name, "origin", svc.Origin.String(), "backend", backend, "err", err)
			return
		}
		out, err := net.DialUDP("udp", nil, raddr)
		if err != nil {
			h.Resolver.Invalidate(svc.Origin)
			h.mu.Unlock()
			h.Logger.Warn("proxy: udp dial failed",
				"service", svc.Name, "origin", svc.Origin.String(),
				"backend", backend, "via", target.Detail, "err", err)
			return
		}
		flowCtx, cancel := context.WithCancel(ctx)
		flow = &udpFlow{out: out, cancel: cancel, lastSeen: time.Now()}
		flow.flowID = h.Flows.Add(policy.Flow{
			SrcUserID:  user.ID,
			ObjectType: "service",
			ObjectID:   svc.ID,
			Port:       dstPort,
			Protocol:   "udp",
			Close: func() {
				cancel()
				_ = out.Close()
			},
		})
		h.flows[key] = flow
		go h.pumpReplies(flowCtx, key, inbound, srcAddr)
	}
	flow.lastSeen = time.Now()
	h.mu.Unlock()

	_, _ = flow.out.Write(data)
}

func (h *UDPHandler) pumpReplies(ctx context.Context, key udpKey, inbound net.PacketConn, srcAddr net.Addr) {
	defer h.removeFlow(key)
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		h.mu.Lock()
		flow := h.flows[key]
		h.mu.Unlock()
		if flow == nil {
			return
		}
		_ = flow.out.SetReadDeadline(time.Now().Add(h.IdleTimeout))
		n, err := flow.out.Read(buf)
		if err != nil {

			if ctx.Err() != nil {
				return
			}
			h.mu.Lock()
			idle := time.Since(flow.lastSeen) > h.IdleTimeout
			h.mu.Unlock()
			if idle {
				return
			}
			continue
		}
		_, _ = inbound.WriteTo(buf[:n], srcAddr)
	}
}

func (h *UDPHandler) removeFlow(key udpKey) {
	h.mu.Lock()
	flow := h.flows[key]
	delete(h.flows, key)
	h.mu.Unlock()
	if flow != nil {
		flow.cancel()
		_ = flow.out.Close()
		h.Flows.Remove(flow.flowID)
	}
}
