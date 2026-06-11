package wg

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"

	"golang.zx2c4.com/wireguard/tun"
)

const nicID tcpip.NICID = 1

type netDevice struct {
	ep             *channel.Endpoint
	stack          *stack.Stack
	events         chan tun.Event
	notifyHandle   *channel.NotificationHandle
	incomingPacket chan *buffer.View
	done           chan struct{}
	mtu            int

	mu     sync.Mutex
	relay  RelayHook
	closed bool
}

type RelayHook func(packet []byte) bool

func newNetDevice(localAddresses []netip.Addr, mtu int) (*netDevice, error) {
	if mtu == 0 {
		mtu = 1420
	}

	dev := &netDevice{
		ep: channel.New(1024, uint32(mtu), ""),
		stack: stack.New(stack.Options{
			NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
			TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol, icmp.NewProtocol4, icmp.NewProtocol6},
			HandleLocal:        true,
		}),
		events:         make(chan tun.Event, 10),
		incomingPacket: make(chan *buffer.View),
		done:           make(chan struct{}),
		mtu:            mtu,
	}

	sackEnabled := tcpip.TCPSACKEnabled(true)
	if e := dev.stack.SetTransportProtocolOption(tcp.ProtocolNumber, &sackEnabled); e != nil {
		return nil, fmt.Errorf("wg: enable SACK: %v", e)
	}

	dev.notifyHandle = dev.ep.AddNotify(dev)
	if e := dev.stack.CreateNIC(nicID, dev.ep); e != nil {
		return nil, fmt.Errorf("wg: CreateNIC: %v", e)
	}

	for _, ip := range localAddresses {
		if err := dev.AddAddress(ip); err != nil {
			return nil, err
		}
	}

	dev.stack.AddRoute(tcpip.Route{Destination: header.IPv4EmptySubnet, NIC: nicID})
	dev.stack.AddRoute(tcpip.Route{Destination: header.IPv6EmptySubnet, NIC: nicID})

	dev.events <- tun.EventUp
	return dev, nil
}
func (d *netDevice) SetRelay(hook RelayHook) {
	d.mu.Lock()
	d.relay = hook
	d.mu.Unlock()
}

func (d *netDevice) AddAddress(ip netip.Addr) error {
	var proto tcpip.NetworkProtocolNumber
	switch {
	case ip.Is4():
		proto = ipv4.ProtocolNumber
	case ip.Is6():
		proto = ipv6.ProtocolNumber
	default:
		return fmt.Errorf("wg: address %s is neither v4 nor v6", ip)
	}
	pa := tcpip.ProtocolAddress{
		Protocol:          proto,
		AddressWithPrefix: tcpip.AddrFromSlice(ip.AsSlice()).WithPrefix(),
	}
	if e := d.stack.AddProtocolAddress(nicID, pa, stack.AddressProperties{}); e != nil {
		return fmt.Errorf("wg: add address %s: %v", ip, e)
	}
	return nil
}

func (d *netDevice) RemoveAddress(ip netip.Addr) error {
	addr := tcpip.AddrFromSlice(ip.AsSlice())
	_ = d.stack.RemoveAddress(nicID, addr)
	return nil
}

func (d *netDevice) Name() (string, error)    { return "lightscale", nil }
func (d *netDevice) File() *os.File           { return nil }
func (d *netDevice) Events() <-chan tun.Event { return d.events }
func (d *netDevice) MTU() (int, error)        { return d.mtu, nil }
func (d *netDevice) BatchSize() int           { return 1 }

func (d *netDevice) Read(buf [][]byte, sizes []int, offset int) (int, error) {
	var view *buffer.View
	select {
	case view = <-d.incomingPacket:
	case <-d.done:
		return 0, os.ErrClosed
	}
	n, err := view.Read(buf[0][offset:])
	if err != nil {
		return 0, err
	}
	sizes[0] = n
	return 1, nil
}

func (d *netDevice) Write(buf [][]byte, offset int) (int, error) {
	for _, b := range buf {
		packet := b[offset:]
		if len(packet) == 0 {
			continue
		}

		d.mu.Lock()
		hook := d.relay
		d.mu.Unlock()
		if hook != nil && hook(packet) {
			continue
		}

		pkb := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(packet),
		})
		switch packet[0] >> 4 {
		case 4:
			d.ep.InjectInbound(header.IPv4ProtocolNumber, pkb)
		case 6:
			d.ep.InjectInbound(header.IPv6ProtocolNumber, pkb)
		default:
			pkb.DecRef()
		}
	}
	return len(buf), nil
}

func (d *netDevice) WriteNotify() {
	pkt := d.ep.Read()
	if pkt == nil {
		return
	}
	view := pkt.ToView()
	pkt.DecRef()
	if !d.enqueue(view) {
		view.Release()
	}
}

func (d *netDevice) Inject(packet []byte) {
	view := buffer.NewViewWithData(append([]byte(nil), packet...))
	if !d.enqueue(view) {
		view.Release()
	}
}

func (d *netDevice) enqueue(view *buffer.View) bool {
	select {
	case <-d.done:
		return false
	default:
	}
	select {
	case d.incomingPacket <- view:
		return true
	case <-d.done:
		return false
	}
}
func (d *netDevice) Close() error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	d.mu.Unlock()

	d.stack.RemoveNIC(nicID)
	d.stack.Close()
	d.ep.RemoveNotify(d.notifyHandle)
	d.ep.Close()
	close(d.events)

	close(d.done)
	return nil
}

func (d *netDevice) listenTCP(addr netip.AddrPort) (*gonet.TCPListener, error) {
	fa, pn := fullAddr(addr)
	return gonet.ListenTCP(d.stack, fa, pn)
}

func (d *netDevice) listenUDP(addr netip.AddrPort) (*gonet.UDPConn, error) {
	fa, pn := fullAddr(addr)
	return gonet.DialUDP(d.stack, &fa, nil, pn)
}

func (d *netDevice) listenUDPAddr(addr netip.AddrPort) (net.PacketConn, error) {
	fa, pn := fullAddr(addr)
	conn, err := gonet.DialUDP(d.stack, &fa, nil, pn)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func fullAddr(a netip.AddrPort) (tcpip.FullAddress, tcpip.NetworkProtocolNumber) {
	var proto tcpip.NetworkProtocolNumber
	if a.Addr().Is4() {
		proto = ipv4.ProtocolNumber
	} else {
		proto = ipv6.ProtocolNumber
	}
	return tcpip.FullAddress{
		NIC:  nicID,
		Addr: tcpip.AddrFromSlice(a.Addr().AsSlice()),
		Port: a.Port(),
	}, proto
}
