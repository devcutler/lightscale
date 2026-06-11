package crypto

import (
	"crypto/subtle"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestGenerateKeypair_ValidBase64And32Bytes(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}

	priv, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Fatalf("private key not valid base64: %v", err)
	}
	if len(priv) != 32 {
		t.Fatalf("private key length = %d, want 32", len(priv))
	}

	pub, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Fatalf("public key not valid base64: %v", err)
	}
	if len(pub) != 32 {
		t.Fatalf("public key length = %d, want 32", len(pub))
	}
}

func TestGenerateKeypair_Clamping(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	priv, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if priv[0]&7 != 0 {
		t.Fatalf("priv[0] low 3 bits not cleared: 0x%02x", priv[0])
	}
	if priv[0]&248 != priv[0] {
		t.Fatalf("priv[0]&248 != priv[0]: 0x%02x", priv[0])
	}
	if priv[31]&128 != 0 {
		t.Fatalf("priv[31] high bit not cleared: 0x%02x", priv[31])
	}
	if priv[31]&64 == 0 {
		t.Fatalf("priv[31] bit 6 not set: 0x%02x", priv[31])
	}
}

func TestGenerateKeypair_PublicDerivedFromPrivate(t *testing.T) {
	kp, err := GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	priv, _ := base64.StdEncoding.DecodeString(kp.PrivateKey)
	pub, _ := base64.StdEncoding.DecodeString(kp.PublicKey)

	derived, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		t.Fatalf("X25519: %v", err)
	}
	if subtle.ConstantTimeCompare(derived, pub) != 1 {
		t.Fatalf("public key does not match curve25519.X25519(priv, Basepoint)")
	}
}

func TestGenerateKeypair_Unique(t *testing.T) {
	const n = 100
	seenPriv := make(map[string]struct{}, n)
	seenPub := make(map[string]struct{}, n)
	for i := range n {
		kp, err := GenerateKeypair()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, ok := seenPriv[kp.PrivateKey]; ok {
			t.Fatalf("duplicate private key at iteration %d", i)
		}
		if _, ok := seenPub[kp.PublicKey]; ok {
			t.Fatalf("duplicate public key at iteration %d", i)
		}
		seenPriv[kp.PrivateKey] = struct{}{}
		seenPub[kp.PublicKey] = struct{}{}
	}
}

func TestGeneratePresharedKey_ValidBase64And32Bytes(t *testing.T) {
	psk, err := GeneratePresharedKey()
	if err != nil {
		t.Fatalf("GeneratePresharedKey: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(psk)
	if err != nil {
		t.Fatalf("psk not valid base64: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("psk length = %d, want 32", len(raw))
	}
}

func TestGeneratePresharedKey_Unique(t *testing.T) {
	const n = 100
	seen := make(map[string]struct{}, n)
	for i := range n {
		psk, err := GeneratePresharedKey()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if _, ok := seen[psk]; ok {
			t.Fatalf("duplicate psk at iteration %d", i)
		}
		seen[psk] = struct{}{}
	}
}
