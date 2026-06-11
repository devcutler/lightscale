package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func GeneratePresharedKey() (string, error) {
	var psk [32]byte
	if _, err := rand.Read(psk[:]); err != nil {
		return "", fmt.Errorf("crypto: read entropy: %w", err)
	}
	return base64.StdEncoding.EncodeToString(psk[:]), nil
}
