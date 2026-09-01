package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestE2E_AllowAndDeny(t *testing.T) {
	h := startHarness(t)

	backendPort := startEchoBackend(t)
	h.apiPost("/api/services", map[string]any{
		"name":        "echo",
		"origin_kind": "host",
		"ports":       fmt.Sprintf("%d/tcp", backendPort),
	})
	svc := getServiceByName(t, h, "echo")
	vip := mustParseAddr(t, svc["ip_address"].(string))
	time.Sleep(200 * time.Millisecond)

	alice := h.startPeer("alice")
	conn, err := alice.Dial(netip.AddrPortFrom(vip, uint16(backendPort)), 1500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatalf("expected connection without policy to fail")
	}
	h.apiPost("/api/policies", map[string]any{
		"subject_name": "alice",
		"object_name":  "echo",
		"action":       "allow",
	})
	time.Sleep(150 * time.Millisecond)

	conn = mustDial(t, alice, vip, backendPort, 4*time.Second)
	defer conn.Close()
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := readN(t, conn, 5, 2*time.Second)
	if !bytes.Equal(got, []byte("hello")) {
		t.Fatalf("echo got %q", got)
	}
}
func TestE2E_PortAllowlist(t *testing.T) {
	h := startHarness(t)
	backendPort := startEchoBackend(t)
	alice := h.startPeer("alice")
	h.apiPost("/api/services", map[string]any{
		"name":        "echo",
		"origin_kind": "host",
		"ports":       "9999/tcp",
	})
	svc := getServiceByName(t, h, "echo")
	vip := mustParseAddr(t, svc["ip_address"].(string))
	time.Sleep(200 * time.Millisecond)

	h.apiPost("/api/policies", map[string]any{
		"subject_name": "alice",
		"object_name":  "echo",
		"action":       "allow",
	})
	conn, err := alice.Dial(netip.AddrPortFrom(vip, uint16(backendPort)), 1500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatalf("expected disallowed port to fail dial")
	}
}

func TestE2E_KillOnRevoke(t *testing.T) {
	h := startHarness(t)
	backendPort := startEchoBackend(t)
	alice := h.startPeer("alice")

	h.apiPost("/api/services", map[string]any{
		"name":        "echo",
		"origin_kind": "host",
		"ports":       fmt.Sprintf("%d/tcp", backendPort),
	})
	svc := getServiceByName(t, h, "echo")
	vip := mustParseAddr(t, svc["ip_address"].(string))
	time.Sleep(200 * time.Millisecond)

	h.apiPost("/api/policies", map[string]any{
		"subject_name": "alice",
		"object_name":  "echo",
		"action":       "allow",
	})
	conn := mustDial(t, alice, vip, backendPort, 4*time.Second)
	defer conn.Close()
	conn.Write([]byte("ping"))
	if got := readN(t, conn, 4, 1*time.Second); !bytes.Equal(got, []byte("ping")) {
		t.Fatalf("echo got %q", got)
	}
	h.apiPost("/api/policies", map[string]any{
		"subject_name": "alice",
		"object_name":  "echo",
		"action":       "deny",
	})
	time.Sleep(200 * time.Millisecond)

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 4)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected the connection to be killed after policy revoke")
	}
}

func TestE2E_DynamicServiceAdd(t *testing.T) {
	h := startHarness(t)
	alice := h.startPeer("alice")

	backendPort := startEchoBackend(t)
	h.apiPost("/api/services", map[string]any{
		"name":        "lateweb",
		"origin_kind": "host",
		"ports":       fmt.Sprintf("%d/tcp", backendPort),
	})
	svc := getServiceByName(t, h, "lateweb")
	vip := mustParseAddr(t, svc["ip_address"].(string))

	h.apiPost("/api/policies", map[string]any{
		"subject_name": "alice",
		"object_name":  "lateweb",
		"action":       "allow",
	})
	time.Sleep(200 * time.Millisecond)

	conn := mustDial(t, alice, vip, backendPort, 4*time.Second)
	defer conn.Close()
	conn.Write([]byte("late"))
	if got := readN(t, conn, 4, 2*time.Second); !bytes.Equal(got, []byte("late")) {
		t.Fatalf("echo got %q", got)
	}
}
func TestE2E_PeerToPeerLanMode(t *testing.T) {
	h := startHarness(t)

	h.apiPost("/api/user-groups", map[string]any{"name": "house", "lan_mode": true})

	alice := h.startPeer("alice")
	bob := h.startPeer("bob")

	h.apiPost("/api/user-groups/1/members", map[string]any{"user_name": "alice"})
	h.apiPost("/api/user-groups/1/members", map[string]any{"user_name": "bob"})

	bobLn, err := bob.tnet.ListenTCP(&net.TCPAddr{IP: bob.addr.AsSlice(), Port: 4242})
	if err != nil {
		t.Fatalf("bob listen: %v", err)
	}
	defer bobLn.Close()
	go func() {
		for {
			c, err := bobLn.Accept()
			if err != nil {
				return
			}
			go io.Copy(c, c)
		}
	}()

	time.Sleep(300 * time.Millisecond)

	conn, err := alice.Dial(netip.AddrPortFrom(bob.addr, 4242), 4*time.Second)
	if err != nil {
		t.Fatalf("alice -> bob via lan-mode failed: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("p2p")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := readN(t, conn, 3, 2*time.Second); !bytes.Equal(got, []byte("p2p")) {
		t.Fatalf("p2p echo got %q", got)
	}
}

func TestE2E_WildcardPorts(t *testing.T) {
	h := startHarness(t)
	backendPort := startEchoBackend(t)
	alice := h.startPeer("alice")

	h.apiPost("/api/services", map[string]any{
		"name":        "wild",
		"origin_kind": "host",
		"ports":       fmt.Sprintf("%d/tcp", backendPort),
	})
	svc := getServiceByName(t, h, "wild")
	vip := mustParseAddr(t, svc["ip_address"].(string))
	time.Sleep(200 * time.Millisecond)

	h.apiPost("/api/policies", map[string]any{
		"subject_name": "alice",
		"object_name":  "wild",
		"action":       "allow",
	})
	conn := mustDial(t, alice, vip, backendPort, 4*time.Second)
	defer conn.Close()
	conn.Write([]byte("any"))
	if got := readN(t, conn, 3, 2*time.Second); !bytes.Equal(got, []byte("any")) {
		t.Fatalf("wildcard echo got %q", got)
	}
}

func mustDial(t *testing.T, peer *PeerHandle, ip netip.Addr, port int, timeout time.Duration) net.Conn {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := peer.Dial(netip.AddrPortFrom(ip, uint16(port)), 500*time.Millisecond)
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("dial %s:%d after %v: %v", ip, port, timeout, lastErr)
	return nil
}

func readN(t *testing.T, r net.Conn, n int, timeout time.Duration) []byte {
	t.Helper()
	r.SetReadDeadline(time.Now().Add(timeout))
	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		read, err := r.Read(buf[:n-len(out)])
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("read: %v", err)
		}
		out = append(out, buf[:read]...)
	}
	return out
}

func mustParseAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return a
}

func getServiceByName(t *testing.T, h *e2eHarness, name string) map[string]any {
	t.Helper()
	resp, err := h.client.Get(h.apiURL + "/api/services")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("unmarshal services: %v", err)
	}
	for _, s := range arr {
		if s["name"] == name {
			return s
		}
	}
	t.Fatalf("service %s not found", name)
	return nil
}
