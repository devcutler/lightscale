package wg

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

type PolicyChecker interface {
	CheckTCP(srcIP, dstIP string, port int) bool
	CheckUDP(srcIP, dstIP string, port int) bool
}

type Dispatcher struct {
	srv     *Server
	checker PolicyChecker

	tcpAccept func(net.Conn, netip.Addr, int)
	udpAccept func(net.PacketConn, net.Addr, netip.Addr, int, []byte)
}

func NewDispatcher(srv *Server, checker PolicyChecker,
	tcpAccept func(conn net.Conn, vip netip.Addr, port int),
	udpAccept func(pc net.PacketConn, src net.Addr, vip netip.Addr, port int, data []byte),
) *Dispatcher {
	d := &Dispatcher{
		srv:       srv,
		checker:   checker,
		tcpAccept: tcpAccept,
		udpAccept: udpAccept,
	}

	tcpFwd := tcp.NewForwarder(srv.netDevice.stack, 0, 4096, d.onTCP)
	srv.netDevice.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpFwd.HandlePacket)

	udpFwd := udp.NewForwarder(srv.netDevice.stack, d.onUDP)
	srv.netDevice.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpFwd.HandlePacket)

	return d
}

func (d *Dispatcher) onTCP(req *tcp.ForwarderRequest) {
	id := req.ID()
	dst := tcpipAddrToNetip(id.LocalAddress)
	src := tcpipAddrToNetip(id.RemoteAddress)
	if !d.checker.CheckTCP(src.String(), dst.String(), int(id.LocalPort)) {
		req.Complete(false)
		return
	}

	var wq waiter.Queue
	ep, e := req.CreateEndpoint(&wq)
	if e != nil {
		req.Complete(false)
		return
	}
	req.Complete(false)
	conn := gonet.NewTCPConn(&wq, ep)
	go d.tcpAccept(conn, dst, int(id.LocalPort))
}
func (d *Dispatcher) onUDP(req *udp.ForwarderRequest) {
	id := req.ID()
	dst := tcpipAddrToNetip(id.LocalAddress)
	src := tcpipAddrToNetip(id.RemoteAddress)
	if !d.checker.CheckUDP(src.String(), dst.String(), int(id.LocalPort)) {
		return
	}

	var wq waiter.Queue
	ep, e := req.CreateEndpoint(&wq)
	if e != nil {
		return
	}
	conn := gonet.NewUDPConn(&wq, ep)
	go d.serveUDPConn(conn, dst, int(id.LocalPort), id)
}

func (d *Dispatcher) serveUDPConn(conn net.PacketConn, vip netip.Addr, port int, id stack.TransportEndpointID) {
	defer conn.Close()
	buf := make([]byte, 64*1024)
	srcAddr := &net.UDPAddr{
		IP:   net.IP(tcpipAddrToNetip(id.RemoteAddress).AsSlice()),
		Port: int(id.RemotePort),
	}
	idle := 60 * time.Second
	for {
		_ = conn.SetReadDeadline(time.Now().Add(idle))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			return
		}
		data := append([]byte(nil), buf[:n]...)
		d.udpAccept(conn, srcAddr, vip, port, data)
	}
}
func dbgf(format string, a ...any) {
	if false {
		fmt.Fprintf(os.Stderr, "[wg] "+format+"\n", a...)
	}
}
func tcpipAddrToNetip(a tcpip.Address) netip.Addr {
	switch a.Len() {
	case 4:
		var b [4]byte
		copy(b[:], a.AsSlice())
		return netip.AddrFrom4(b)
	case 16:
		var b [16]byte
		copy(b[:], a.AsSlice())
		return netip.AddrFrom16(b).Unmap()
	}
	return netip.Addr{}
}
