package wg

import (
	"net/netip"

	"github.com/devcutler/lightscale/daemon/policy"
)

type Relay struct {
	policy     *policy.Holder
	srv        *Server
	daemonIP   netip.Addr
	clientCIDR netip.Prefix
}

func NewRelay(srv *Server, holder *policy.Holder, clientSubnet, daemonIP string) (*Relay, error) {
	prefix, err := netip.ParsePrefix(clientSubnet)
	if err != nil {
		return nil, err
	}
	addr, err := netip.ParseAddr(daemonIP)
	if err != nil {
		return nil, err
	}
	return &Relay{policy: holder, srv: srv, daemonIP: addr, clientCIDR: prefix}, nil
}

func (r *Relay) Handle(packet []byte) bool {
	if len(packet) < 20 {
		return false
	}
	if packet[0]>>4 != 4 {
		return false
	}
	src, dst, ok := parseIPv4(packet)
	if !ok {
		return false
	}

	if dst == r.daemonIP {
		return false
	}
	if !r.clientCIDR.Contains(dst) {
		return false
	}
	if src == dst {
		return true
	}

	idx := r.policy.Load()
	if _, isPeer := idx.PeerByIP[dst.String()]; !isPeer {
		return true
	}

	decision, _, _ := idx.CheckPeer(src.String(), dst.String())
	if decision != policy.Allow {
		return true
	}

	r.srv.Inject(packet)
	return true
}
func parseIPv4(packet []byte) (src, dst netip.Addr, ok bool) {
	if len(packet) < 20 {
		return netip.Addr{}, netip.Addr{}, false
	}
	src, ok = netip.AddrFromSlice(packet[12:16])
	if !ok {
		return netip.Addr{}, netip.Addr{}, false
	}
	dst, ok = netip.AddrFromSlice(packet[16:20])
	if !ok {
		return netip.Addr{}, netip.Addr{}, false
	}
	return src.Unmap(), dst.Unmap(), true
}
