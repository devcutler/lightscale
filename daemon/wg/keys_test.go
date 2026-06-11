package wg

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestGenerateKeypair(t *testing.T) {
	priv, pub, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	rawPriv, err := base64.StdEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("priv not valid base64: %v", err)
	}
	if len(rawPriv) != 32 {
		t.Fatalf("priv decodes to %d bytes, want 32", len(rawPriv))
	}
	rawPub, err := base64.StdEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("pub not valid base64: %v", err)
	}
	if len(rawPub) != 32 {
		t.Fatalf("pub decodes to %d bytes, want 32", len(rawPub))
	}

	privHex := hex.EncodeToString(rawPriv)
	derived, err := derivePublicKey(privHex)
	if err != nil {
		t.Fatalf("derivePublicKey: %v", err)
	}
	if base64.StdEncoding.EncodeToString(derived) != pub {
		t.Fatalf("derived pub %q != returned pub %q",
			base64.StdEncoding.EncodeToString(derived), pub)
	}
}

func TestGenerateKeypairUnique(t *testing.T) {
	seenPriv := map[string]bool{}
	seenPub := map[string]bool{}
	for i := range 200 {
		priv, pub, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("GenerateKeypair: %v", err)
		}
		if seenPriv[priv] {
			t.Fatalf("duplicate private key at iteration %d", i)
		}
		if seenPub[pub] {
			t.Fatalf("duplicate public key at iteration %d", i)
		}
		seenPriv[priv] = true
		seenPub[pub] = true
	}
}

func TestGeneratePrivateKeyClamping(t *testing.T) {
	seen := map[string]bool{}
	for i := range 200 {
		priv, err := generatePrivateKey()
		if err != nil {
			t.Fatalf("generatePrivateKey: %v", err)
		}
		raw, err := base64.StdEncoding.DecodeString(priv)
		if err != nil {
			t.Fatalf("not valid base64: %v", err)
		}
		if len(raw) != 32 {
			t.Fatalf("decodes to %d bytes, want 32", len(raw))
		}
		if raw[0]&7 != 0 {
			t.Fatalf("byte[0] low 3 bits not cleared: %08b", raw[0])
		}
		if raw[31]&128 != 0 {
			t.Fatalf("byte[31] high bit not cleared: %08b", raw[31])
		}
		if raw[31]&64 == 0 {
			t.Fatalf("byte[31] bit 6 not set: %08b", raw[31])
		}
		if seen[priv] {
			t.Fatalf("duplicate private key at iteration %d", i)
		}
		seen[priv] = true
	}
}

func TestBase64ToHexValid(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i)
	}
	b64 := base64.StdEncoding.EncodeToString(key[:])

	h, err := Base64ToHex(b64)
	if err != nil {
		t.Fatalf("Base64ToHex: %v", err)
	}
	if len(h) != 64 {
		t.Fatalf("hex len %d, want 64", len(h))
	}
	if h != hex.EncodeToString(key[:]) {
		t.Fatalf("hex %q != expected %q", h, hex.EncodeToString(key[:]))
	}
}

func TestBase64ToHexWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err := base64ToHex(short)
	if err == nil {
		t.Fatal("expected error for 16-byte key")
	}
	if !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("error %q does not mention '32 bytes'", err.Error())
	}
}

func TestBase64ToHexInvalidBase64(t *testing.T) {
	_, err := base64ToHex("not valid base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDerivePublicKeyBadLength(t *testing.T) {
	_, err := derivePublicKey(hex.EncodeToString(make([]byte, 16)))
	if err == nil {
		t.Fatal("expected error for 16-byte hex")
	}
	if _, err := derivePublicKey("zzzz"); err == nil {
		t.Fatal("expected error for non-hex string")
	}
}

func TestDerivePublicKeyMatchesCurve25519(t *testing.T) {
	priv, err := generatePrivateKey()
	if err != nil {
		t.Fatalf("generatePrivateKey: %v", err)
	}
	rawPriv, _ := base64.StdEncoding.DecodeString(priv)
	privHex := hex.EncodeToString(rawPriv)

	got, err := derivePublicKey(privHex)
	if err != nil {
		t.Fatalf("derivePublicKey: %v", err)
	}
	want, err := curve25519.X25519(rawPriv, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("curve25519.X25519: %v", err)
	}
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("derived %x != curve25519 %x", got, want)
	}
}
