package wg

import (
	"net/netip"
	"testing"

	"github.com/devcutler/lightscale/daemon/policy"
)

func makeIPv4(src, dst netip.Addr) []byte {
	p := make([]byte, 20)
	p[0] = 0x45
	s := src.As4()
	d := dst.As4()
	copy(p[12:16], s[:])
	copy(p[16:20], d[:])
	return p
}

func emptyIndex() *policy.Index {
	return &policy.Index{
		PeerByIP:     map[string]policy.UserSnapshot{},
		UserByID:     map[int64]policy.UserSnapshot{},
		GroupsByUser: map[int64][]int64{},
		UserGroups:   map[int64]policy.UserGroupSnapshot{},
	}
}

func newTestRelay(t *testing.T, holder *policy.Holder) *Relay {
	t.Helper()
	r, err := NewRelay(nil, holder, "10.6.0.0/23", "10.6.0.1")
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}
	return r
}

func TestParseIPv4(t *testing.T) {
	src := netip.MustParseAddr("10.6.0.2")
	dst := netip.MustParseAddr("10.6.0.3")
	p := makeIPv4(src, dst)
	gs, gd, ok := parseIPv4(p)
	if !ok {
		t.Fatal("parseIPv4 returned ok=false")
	}
	if gs != src || gd != dst {
		t.Fatalf("got src=%v dst=%v", gs, gd)
	}
	if gs.Is4In6() || gd.Is4In6() {
		t.Fatal("addresses should be Unmap'd to v4")
	}
}

func TestParseIPv4TooShort(t *testing.T) {
	if _, _, ok := parseIPv4(make([]byte, 19)); ok {
		t.Fatal("19-byte packet should return ok=false")
	}
}

func TestRelayHandleTooShort(t *testing.T) {
	r := newTestRelay(t, &policy.Holder{})
	if r.Handle(make([]byte, 10)) {
		t.Fatal("short packet should return false")
	}
}

func TestRelayHandleWrongVersion(t *testing.T) {
	r := newTestRelay(t, &policy.Holder{})
	p := makeIPv4(netip.MustParseAddr("10.6.0.2"), netip.MustParseAddr("10.6.0.3"))
	p[0] = 0x65
	if r.Handle(p) {
		t.Fatal("non-v4 version should return false")
	}
}

func TestRelayHandleDstIsDaemon(t *testing.T) {
	r := newTestRelay(t, &policy.Holder{})
	p := makeIPv4(netip.MustParseAddr("10.6.0.2"), netip.MustParseAddr("10.6.0.1"))
	if r.Handle(p) {
		t.Fatal("dst==daemonIP should return false (handled by gateway)")
	}
}

func TestRelayHandleDstOutsideCIDR(t *testing.T) {
	r := newTestRelay(t, &policy.Holder{})
	p := makeIPv4(netip.MustParseAddr("10.6.0.2"), netip.MustParseAddr("10.99.0.3"))
	if r.Handle(p) {
		t.Fatal("dst outside client CIDR should return false")
	}
}

func TestRelayHandleSrcEqualsDst(t *testing.T) {
	r := newTestRelay(t, &policy.Holder{})
	p := makeIPv4(netip.MustParseAddr("10.6.0.5"), netip.MustParseAddr("10.6.0.5"))
	if !r.Handle(p) {
		t.Fatal("src==dst should return true (consumed)")
	}
}

func TestRelayHandleDstNotKnownPeer(t *testing.T) {
	holder := &policy.Holder{}
	holder.Store(emptyIndex())
	r := newTestRelay(t, holder)
	p := makeIPv4(netip.MustParseAddr("10.6.0.2"), netip.MustParseAddr("10.6.0.3"))
	if !r.Handle(p) {
		t.Fatal("dst in CIDR but not a known peer should return true (consumed)")
	}
}

func TestRelayHandlePeerDenied(t *testing.T) {
	idx := emptyIndex()

	for _, u := range []policy.UserSnapshot{
		{ID: 1, IPAddress: "10.6.0.2"},
		{ID: 2, IPAddress: "10.6.0.3"},
	} {
		idx.PeerByIP[u.IPAddress] = u
		idx.UserByID[u.ID] = u
	}
	holder := &policy.Holder{}
	holder.Store(idx)
	r := newTestRelay(t, holder)
	p := makeIPv4(netip.MustParseAddr("10.6.0.2"), netip.MustParseAddr("10.6.0.3"))

	if !r.Handle(p) {
		t.Fatal("known peer, CheckPeer denies should return true (consumed, not injected)")
	}
}

func TestRelayHandlePeerAllowedInjects(t *testing.T) {
	srv, _, err := Open(Options{
		UDPPort:  0,
		DaemonIP: netip.MustParseAddr("10.6.0.1"),
	})
	if err != nil {
		t.Fatalf("Open server: %v", err)
	}
	defer srv.Close()

	idx := emptyIndex()
	for _, u := range []policy.UserSnapshot{
		{ID: 1, IPAddress: "10.6.0.2"},
		{ID: 2, IPAddress: "10.6.0.3"},
	} {
		idx.PeerByIP[u.IPAddress] = u
		idx.UserByID[u.ID] = u
	}
	idx.GroupsByUser[1] = []int64{10}
	idx.GroupsByUser[2] = []int64{10}
	idx.UserGroups[10] = policy.UserGroupSnapshot{ID: 10, LANMode: true, Members: []int64{1, 2}}

	if d, _, _ := idx.CheckPeer("10.6.0.2", "10.6.0.3"); d != policy.Allow {
		t.Fatalf("fixture CheckPeer = %v, want Allow", d)
	}

	holder := &policy.Holder{}
	holder.Store(idx)
	r, err := NewRelay(srv, holder, "10.6.0.0/23", "10.6.0.1")
	if err != nil {
		t.Fatalf("NewRelay: %v", err)
	}
	p := makeIPv4(netip.MustParseAddr("10.6.0.2"), netip.MustParseAddr("10.6.0.3"))
	if !r.Handle(p) {
		t.Fatal("allowed peer should return true")
	}

}
