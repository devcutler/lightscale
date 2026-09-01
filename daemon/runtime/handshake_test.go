package daemon

import (
	"strings"
	"testing"
	"time"
)

func TestE2E_HandshakeAndPing(t *testing.T) {
	h := startHarness(t)
	alice := h.startPeer("alice")
	t.Logf("alice registered as %s", alice.addr)

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		dump, err := alice.device.IpcGet()
		if err == nil && handshakeCompleted(dump) {
			t.Log("handshake completed")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("wireguard handshake never completed within 6s")
}

func handshakeCompleted(dump string) bool {
	for line := range strings.SplitSeq(dump, "\n") {
		v, ok := strings.CutPrefix(line, "last_handshake_time_sec=")
		if ok && strings.TrimSpace(v) != "0" {
			return true
		}
	}
	return false
}
