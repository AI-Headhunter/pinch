package identity_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pinch-protocol/pinch/relay/internal/identity"
)

type identityVector struct {
	Ed25519Seed       string `json:"ed25519_seed"`
	Ed25519PublicKey  string `json:"ed25519_public_key"`
	Ed25519PrivateKey string `json:"ed25519_private_key"`
	X25519PublicKey   string `json:"x25519_public_key"`
	X25519PrivateKey  string `json:"x25519_private_key"`
	Address           string `json:"address"`
}

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "testdata")
}

func loadIdentityVectors(t *testing.T) []identityVector {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), "identity_vectors.json"))
	if err != nil {
		t.Fatalf("failed to read identity_vectors.json: %v", err)
	}
	var result struct {
		Vectors []identityVector `json:"vectors"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse identity_vectors.json: %v", err)
	}
	return result.Vectors
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("failed to decode hex %q: %v", s, err)
	}
	return b
}

func TestGenerateAddress(t *testing.T) {
	vectors := loadIdentityVectors(t)
	for _, v := range vectors {
		seed := mustDecodeHex(t, v.Ed25519Seed)
		priv := ed25519.NewKeyFromSeed(seed)
		pub := priv.Public().(ed25519.PublicKey)

		got := identity.GenerateAddress(pub, "test.relay.example.com")
		if got != v.Address {
			t.Errorf("GenerateAddress mismatch for seed %s:\n  got  %s\n  want %s", v.Ed25519Seed, got, v.Address)
		}
	}
}

func TestValidateAddress(t *testing.T) {
	vectors := loadIdentityVectors(t)

	// Valid addresses should validate
	for _, v := range vectors {
		pub, host, err := identity.ValidateAddress(v.Address)
		if err != nil {
			t.Errorf("ValidateAddress failed for valid address %s: %v", v.Address, err)
			continue
		}
		expectedPub := mustDecodeHex(t, v.Ed25519PublicKey)
		if hex.EncodeToString(pub) != hex.EncodeToString(expectedPub) {
			t.Errorf("ValidateAddress returned wrong public key for %s", v.Address)
		}
		if host != "test.relay.example.com" {
			t.Errorf("ValidateAddress returned wrong host: %s", host)
		}
	}

	// Tampered addresses should fail
	tamperedCases := []string{
		"pinch:INVALID@test.relay.example.com",
		"pinch:111111111111111111111111111111111111111111111111111@test.relay.example.com",
		"not-an-address",
		"",
		"pinch:@test.relay.example.com",
		"pinch:abc",
	}
	for _, addr := range tamperedCases {
		_, _, err := identity.ValidateAddress(addr)
		if err == nil {
			t.Errorf("ValidateAddress should have failed for tampered address %q", addr)
		}
	}
}

func TestParseAddress(t *testing.T) {
	vectors := loadIdentityVectors(t)

	for _, v := range vectors {
		payload, host, err := identity.ParseAddress(v.Address)
		if err != nil {
			t.Errorf("ParseAddress failed for %s: %v", v.Address, err)
			continue
		}
		if payload == "" {
			t.Errorf("ParseAddress returned empty payload for %s", v.Address)
		}
		if host != "test.relay.example.com" {
			t.Errorf("ParseAddress returned wrong host for %s: %s", v.Address, host)
		}
	}

	// Invalid formats
	invalidCases := []string{
		"not-an-address",
		"pinch:abc",
		"pinch:@host",
		"",
	}
	for _, addr := range invalidCases {
		_, _, err := identity.ParseAddress(addr)
		if err == nil {
			t.Errorf("ParseAddress should have failed for %q", addr)
		}
	}
}
