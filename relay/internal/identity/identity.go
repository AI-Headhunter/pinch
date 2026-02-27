// Package identity provides Ed25519 address generation and validation for the
// Pinch protocol. Addresses follow the format: pinch:<base58(pubkey+checksum)>@<host>
package identity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"

	"github.com/mr-tron/base58"
)

var addressRegex = regexp.MustCompile(`^pinch:([1-9A-HJ-NP-Za-km-z]+)@(.+)$`)

// GenerateAddress creates a Pinch address from an Ed25519 public key and relay
// host. The address format is: pinch:<base58(pubkey + sha256(pubkey)[0:4])>@<host>
func GenerateAddress(pubKey ed25519.PublicKey, relayHost string) string {
	hash := sha256.Sum256(pubKey)
	checksum := hash[:4]
	payload := make([]byte, 36)
	copy(payload[:32], pubKey)
	copy(payload[32:], checksum)
	encoded := base58.Encode(payload)
	return fmt.Sprintf("pinch:%s@%s", encoded, relayHost)
}

// ValidateAddress parses and validates a Pinch address, returning the embedded
// Ed25519 public key and relay host. Returns an error if the address format is
// invalid or the checksum does not match.
func ValidateAddress(addr string) (ed25519.PublicKey, string, error) {
	payload, host, err := ParseAddress(addr)
	if err != nil {
		return nil, "", err
	}

	decoded, err := base58.Decode(payload)
	if err != nil {
		return nil, "", fmt.Errorf("invalid base58 in address: %w", err)
	}

	if len(decoded) != 36 {
		return nil, "", fmt.Errorf("invalid address payload length: expected 36, got %d", len(decoded))
	}

	pubKey := decoded[:32]
	checksum := decoded[32:36]

	hash := sha256.Sum256(pubKey)
	expectedChecksum := hash[:4]

	for i := 0; i < 4; i++ {
		if checksum[i] != expectedChecksum[i] {
			return nil, "", errors.New("address checksum mismatch")
		}
	}

	return ed25519.PublicKey(pubKey), host, nil
}

// ParseAddress extracts the base58 payload and host from a Pinch address string.
// It validates the format but does not verify the checksum.
func ParseAddress(addr string) (payload string, host string, err error) {
	matches := addressRegex.FindStringSubmatch(addr)
	if matches == nil {
		return "", "", fmt.Errorf("invalid address format: %q", addr)
	}
	return matches[1], matches[2], nil
}
