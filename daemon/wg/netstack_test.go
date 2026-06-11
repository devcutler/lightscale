package wg

import (
	"net/netip"
	"testing"

	"gvisor.dev/gvisor/pkg/tcpip"

	"github.com/devcutler/lightscale/daemon/policy"
)

func TestTcpipAddrToNetip(t *testing.T) {
	a4 := tcpip.AddrFrom4([4]byte{10, 6, 0, 2})
	if got := tcpipAddrToNetip(a4); got != netip.MustParseAddr("10.6.0.2") {
		t.Fatalf("v4: got %v", got)
	}

	var b16 [16]byte
	b16[15] = 1
	a16 := tcpip.AddrFrom16(b16)
	if got := tcpipAddrToNetip(a16); got != netip.MustParseAddr("::1") {
		t.Fatalf("v6: got %v", got)
	}

	v4mapped := netip.MustParseAddr("::ffff:10.6.0.7").As16()
	if got := tcpipAddrToNetip(tcpip.AddrFrom16(v4mapped)); got != netip.MustParseAddr("10.6.0.7") {
		t.Fatalf("v4-mapped: got %v", got)
	}

	if got := tcpipAddrToNetip(tcpip.Address{}); got.IsValid() {
		t.Fatalf("zero-length: expected invalid Addr, got %v", got)
	}
}

func TestNewNetDeviceBasics(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 1300)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	defer d.Close()

	if name, _ := d.Name(); name != "lightscale" {
		t.Fatalf("Name = %q", name)
	}
	if mtu, _ := d.MTU(); mtu != 1300 {
		t.Fatalf("MTU = %d, want 1300", mtu)
	}
	if d.BatchSize() != 1 {
		t.Fatalf("BatchSize = %d, want 1", d.BatchSize())
	}
	if d.File() != nil {
		t.Fatal("File() should be nil")
	}
}

func TestNewNetDeviceDefaultMTU(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 0)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	defer d.Close()
	if mtu, _ := d.MTU(); mtu != 1420 {
		t.Fatalf("default MTU = %d, want 1420", mtu)
	}
}

func TestNetDeviceAddRemoveAddress(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 1420)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	defer d.Close()

	if err := d.AddAddress(netip.MustParseAddr("10.6.0.2")); err != nil {
		t.Fatalf("AddAddress v4: %v", err)
	}
	if err := d.AddAddress(netip.MustParseAddr("fd00::1")); err != nil {
		t.Fatalf("AddAddress v6: %v", err)
	}
	if err := d.RemoveAddress(netip.MustParseAddr("10.6.0.2")); err != nil {
		t.Fatalf("RemoveAddress v4: %v", err)
	}
	if err := d.RemoveAddress(netip.MustParseAddr("fd00::1")); err != nil {
		t.Fatalf("RemoveAddress v6: %v", err)
	}
	if err := d.AddAddress(netip.Addr{}); err == nil {
		t.Fatal("AddAddress(zero) should error")
	}
}

func TestNetDeviceInjectReadRoundTrip(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 1420)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	defer d.Close()

	want := []byte{0x45, 0x00, 0x11, 0x22, 0x33, 0x44}
	go d.Inject(want)

	const offset = 4
	bufs := [][]byte{make([]byte, offset+len(want)+10)}
	sizes := make([]int, 1)
	n, err := d.Read(bufs, sizes, offset)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != 1 {
		t.Fatalf("Read returned n=%d, want 1", n)
	}
	got := bufs[0][offset : offset+sizes[0]]
	if string(got) != string(want) {
		t.Fatalf("round-trip mismatch: got %v, want %v", got, want)
	}
}

func TestNetDeviceCloseIdempotent(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 1420)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := d.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNetDeviceReadAfterClose(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 1420)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	_ = d.Close()

	bufs := [][]byte{make([]byte, 64)}
	sizes := make([]int, 1)
	n, err := d.Read(bufs, sizes, 0)
	if err == nil {
		t.Fatal("Read after Close should return an error")
	}
	if n != 0 {
		t.Fatalf("Read after Close returned n=%d, want 0", n)
	}
}

func TestNetDeviceEnqueueAfterClose(t *testing.T) {
	d, err := newNetDevice([]netip.Addr{netip.MustParseAddr("10.6.0.1")}, 1420)
	if err != nil {
		t.Fatalf("newNetDevice: %v", err)
	}
	_ = d.Close()

	d.Inject([]byte{0x45, 0x00})
}

func TestServerOpenAndPeers(t *testing.T) {
	srv, pub, err := Open(Options{
		UDPPort:     0,
		DaemonIP:    netip.MustParseAddr("10.6.0.1"),
		ServiceVIPs: []netip.Addr{netip.MustParseAddr("10.7.0.1")},
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer srv.Close()

	if pub == "" || srv.PublicKey() == "" || srv.PrivateKey() == "" {
		t.Fatal("keys should be non-empty")
	}
	if pub != srv.PublicKey() {
		t.Fatalf("returned pub %q != PublicKey() %q", pub, srv.PublicKey())
	}

	priv, peerPub, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	priv2, peerPub2, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	_ = priv
	_ = priv2

	idx := emptyIndex()
	for _, u := range []policy.UserSnapshot{
		{ID: 1, Name: "alice", IPAddress: "10.6.0.2", PublicKey: peerPub},
		{ID: 2, Name: "bob", IPAddress: "10.6.0.3", PublicKey: peerPub2},
	} {
		idx.UserByID[u.ID] = u
	}

	if err := srv.ApplyPeers(idx); err != nil {
		t.Fatalf("ApplyPeers: %v", err)
	}
	if srv.PeerCount() != 2 {
		t.Fatalf("PeerCount = %d, want 2", srv.PeerCount())
	}

	peers, err := srv.PeerStatus()
	if err != nil {
		t.Fatalf("PeerStatus: %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("PeerStatus returned %d peers, want 2", len(peers))
	}

	if err := srv.AddVIP(netip.MustParseAddr("10.7.0.2")); err != nil {
		t.Fatalf("AddVIP: %v", err)
	}
	if err := srv.RemoveVIP(netip.MustParseAddr("10.7.0.2")); err != nil {
		t.Fatalf("RemoveVIP: %v", err)
	}
}

func TestApplyPeersNilIndex(t *testing.T) {
	srv, _, err := Open(Options{UDPPort: 0, DaemonIP: netip.MustParseAddr("10.6.0.1")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer srv.Close()
	if err := srv.ApplyPeers(nil); err != nil {
		t.Fatalf("ApplyPeers(nil): %v", err)
	}
	if srv.PeerCount() != 0 {
		t.Fatalf("PeerCount = %d, want 0", srv.PeerCount())
	}
}
