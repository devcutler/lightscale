package wg

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func hexKey(seed byte) string {
	var k [32]byte
	k[0] = seed
	for i := 1; i < 32; i++ {
		k[i] = byte(i)
	}
	return hex.EncodeToString(k[:])
}

func b64FromHex(t *testing.T, h string) string {
	t.Helper()
	raw, err := hex.DecodeString(h)
	if err != nil {
		t.Fatalf("bad hex: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func TestParsePeerStatusEmpty(t *testing.T) {
	if got := parsePeerStatus(""); len(got) != 0 {
		t.Fatalf("empty dump: want 0 peers, got %d", len(got))
	}
}

func TestParsePeerStatusSingleAllFields(t *testing.T) {
	pk := hexKey(0xAA)
	dump := strings.Join([]string{
		"private_key=" + hexKey(0x01),
		"public_key=" + pk,
		"endpoint=1.2.3.4:51820",
		"allowed_ip=10.6.0.2/32",
		"allowed_ip=10.6.0.3/32",
		"last_handshake_time_sec=1700000000",
		"rx_bytes=1234",
		"tx_bytes=5678",
		"persistent_keepalive_interval=25",
	}, "\n") + "\n"

	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	p := peers[0]
	if p.PublicKey != b64FromHex(t, pk) {
		t.Fatalf("PublicKey %q, want %q", p.PublicKey, b64FromHex(t, pk))
	}
	if p.Endpoint != "1.2.3.4:51820" {
		t.Fatalf("Endpoint %q", p.Endpoint)
	}
	if len(p.AllowedIPs) != 2 || p.AllowedIPs[0] != "10.6.0.2/32" || p.AllowedIPs[1] != "10.6.0.3/32" {
		t.Fatalf("AllowedIPs %v", p.AllowedIPs)
	}
	if !p.LastHandshake.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("LastHandshake %v", p.LastHandshake)
	}
	if p.RxBytes != 1234 || p.TxBytes != 5678 {
		t.Fatalf("rx=%d tx=%d", p.RxBytes, p.TxBytes)
	}
	if p.KeepaliveInterval != 25 {
		t.Fatalf("keepalive=%d", p.KeepaliveInterval)
	}
}

func TestParsePeerStatusZeroHandshake(t *testing.T) {
	dump := "public_key=" + hexKey(0x01) + "\nlast_handshake_time_sec=0\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	if !peers[0].LastHandshake.IsZero() {
		t.Fatalf("LastHandshake should be zero, got %v", peers[0].LastHandshake)
	}
}

func TestParsePeerStatusZeroPSK(t *testing.T) {
	zeroPSK := hex.EncodeToString(make([]byte, 32))
	dump := "public_key=" + hexKey(0x01) + "\npreshared_key=" + zeroPSK + "\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	if peers[0].PresharedKey != "" {
		t.Fatalf("zero PSK should stay empty, got %q", peers[0].PresharedKey)
	}
}

func TestParsePeerStatusNonZeroPSK(t *testing.T) {
	psk := hexKey(0x99)
	dump := "public_key=" + hexKey(0x01) + "\npreshared_key=" + psk + "\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	if peers[0].PresharedKey != b64FromHex(t, psk) {
		t.Fatalf("PSK %q, want %q", peers[0].PresharedKey, b64FromHex(t, psk))
	}
}

func TestParsePeerStatusMultiplePeers(t *testing.T) {
	dump := strings.Join([]string{
		"public_key=" + hexKey(0x01),
		"rx_bytes=10",
		"public_key=" + hexKey(0x02),
		"rx_bytes=20",
		"public_key=" + hexKey(0x03),
		"rx_bytes=30",
	}, "\n") + "\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 3 {
		t.Fatalf("want 3 peers, got %d", len(peers))
	}
	if peers[0].RxBytes != 10 || peers[1].RxBytes != 20 || peers[2].RxBytes != 30 {
		t.Fatalf("rx mismatch: %d %d %d", peers[0].RxBytes, peers[1].RxBytes, peers[2].RxBytes)
	}
}

func TestParsePeerStatusMalformedLines(t *testing.T) {
	dump := strings.Join([]string{
		"no_equals_sign_here",
		"rx_bytes=999",
		"public_key=" + hexKey(0x05),
		"=emptykey",
		"allowed_ip=10.6.0.9/32",
	}, "\n") + "\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	if len(peers[0].AllowedIPs) != 1 || peers[0].AllowedIPs[0] != "10.6.0.9/32" {
		t.Fatalf("AllowedIPs %v", peers[0].AllowedIPs)
	}
}

func TestParsePeerStatusBadPublicKeyHexSkipped(t *testing.T) {
	dump := strings.Join([]string{
		"public_key=nothexatall",
		"rx_bytes=42",
		"public_key=" + hexKey(0x07),
		"rx_bytes=7",
	}, "\n") + "\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	if peers[0].RxBytes != 7 {
		t.Fatalf("rx=%d, want 7", peers[0].RxBytes)
	}
}

func TestParsePeerStatusCRLF(t *testing.T) {
	dump := "public_key=" + hexKey(0x08) + "\r\nendpoint=5.6.7.8:1234\r\n"
	peers := parsePeerStatus(dump)
	if len(peers) != 1 {
		t.Fatalf("want 1 peer, got %d", len(peers))
	}
	if peers[0].Endpoint != "5.6.7.8:1234" {
		t.Fatalf("Endpoint %q (CR not trimmed?)", peers[0].Endpoint)
	}
}
