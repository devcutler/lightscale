package daemon

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"sync"

	"github.com/devcutler/lightscale/daemon/policy"
	"github.com/devcutler/lightscale/daemon/proxy"
	"github.com/devcutler/lightscale/daemon/wg"
)

type listenerManager struct {
	logger     *slog.Logger
	srv        *wg.Server
	tcp        *proxy.TCPHandler
	udp        *proxy.UDPHandler
	dispatcher *wg.Dispatcher

	mu      sync.Mutex
	current map[serviceKey]netip.Addr
}

type serviceKey int64

func newListenerManager(logger *slog.Logger, srv *wg.Server, holder *policy.Holder, tcp *proxy.TCPHandler, udp *proxy.UDPHandler) *listenerManager {
	m := &listenerManager{
		logger:  logger,
		srv:     srv,
		tcp:     tcp,
		udp:     udp,
		current: map[serviceKey]netip.Addr{},
	}
	checker := &dispatcherChecker{holder: holder}
	m.dispatcher = wg.NewDispatcher(srv, checker, m.acceptTCP, m.acceptUDP)
	return m
}

type dispatcherChecker struct{ holder *policy.Holder }

func (c *dispatcherChecker) CheckTCP(src, dst string, port int) bool {
	idx := c.holder.Load()
	d, _, _ := idx.CheckService(src, dst, port, "tcp")
	return d == policy.Allow
}

func (c *dispatcherChecker) CheckUDP(src, dst string, port int) bool {
	idx := c.holder.Load()
	d, _, _ := idx.CheckService(src, dst, port, "udp")
	return d == policy.Allow
}
func (m *listenerManager) reconcile(ctx context.Context, idx *policy.Index) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()

	want := map[serviceKey]policy.ServiceSnapshot{}
	for id, svc := range idx.ServiceByID {
		want[serviceKey(id)] = svc
	}

	for key, vip := range m.current {
		if _, keep := want[key]; !keep {
			if err := m.srv.RemoveVIP(vip); err != nil {
				m.logger.Warn("listeners: remove vip", "vip", vip, "err", err)
			}
			delete(m.current, key)
		}
	}

	for key, svc := range want {
		if _, present := m.current[key]; present {
			continue
		}
		vip, err := netip.ParseAddr(svc.IPAddress)
		if err != nil {
			m.logger.Warn("listeners: bad VIP", "service", svc.Name, "ip", svc.IPAddress)
			continue
		}
		if err := m.srv.AddVIP(vip); err != nil {
			m.logger.Warn("listeners: add vip", "service", svc.Name, "err", err)
			continue
		}
		m.current[key] = vip
	}
	return nil
}
func (m *listenerManager) acceptTCP(conn net.Conn, vip netip.Addr, port int) {
	go m.tcp.Handle(context.Background(), conn)
	_ = vip
	_ = port
}
func (m *listenerManager) acceptUDP(pc net.PacketConn, src net.Addr, vip netip.Addr, port int, data []byte) {
	go m.udp.HandlePacket(context.Background(), pc, src, vip.String(), port, data)
}

func (m *listenerManager) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, vip := range m.current {
		_ = m.srv.RemoveVIP(vip)
		delete(m.current, key)
	}
}
