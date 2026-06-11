package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

type Keypair struct {
	PrivateKey string
	PublicKey  string
}

func GenerateKeypair() (Keypair, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return Keypair{}, fmt.Errorf("crypto: read entropy: %w", err)
	}
	// curve25519 clamp (RFC 7748): clear low 3 bits, clear top bit, set bit 254
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return Keypair{}, fmt.Errorf("crypto: derive public key: %w", err)
	}

	return Keypair{
		PrivateKey: base64.StdEncoding.EncodeToString(priv[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(pub),
	}, nil
}
