package crypto_test

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pinch-protocol/pinch/relay/internal/crypto"
)

type identityVector struct {
	Ed25519Seed       string `json:"ed25519_seed"`
	Ed25519PublicKey  string `json:"ed25519_public_key"`
	Ed25519PrivateKey string `json:"ed25519_private_key"`
	X25519PublicKey   string `json:"x25519_public_key"`
	X25519PrivateKey  string `json:"x25519_private_key"`
	Address           string `json:"address"`
}

type cryptoVector struct {
	Description         string `json:"description"`
	SenderSeed          string `json:"sender_ed25519_seed"`
	RecipientSeed       string `json:"recipient_ed25519_seed"`
	SenderX25519Pub     string `json:"sender_x25519_pub"`
	SenderX25519Priv    string `json:"sender_x25519_priv"`
	RecipientX25519Pub  string `json:"recipient_x25519_pub"`
	RecipientX25519Priv string `json:"recipient_x25519_priv"`
	Nonce               string `json:"nonce"`
	Plaintext           string `json:"plaintext"`
	Ciphertext          string `json:"ciphertext"`
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

func loadCryptoVectors(t *testing.T) []cryptoVector {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), "crypto_vectors.json"))
	if err != nil {
		t.Fatalf("failed to read crypto_vectors.json: %v", err)
	}
	var result struct {
		Vectors []cryptoVector `json:"vectors"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse crypto_vectors.json: %v", err)
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

func TestEd25519PublicToX25519(t *testing.T) {
	vectors := loadIdentityVectors(t)
	for _, v := range vectors {
		seed := mustDecodeHex(t, v.Ed25519Seed)
		priv := ed25519.NewKeyFromSeed(seed)
		pub := priv.Public().(ed25519.PublicKey)
		expectedX25519Pub := mustDecodeHex(t, v.X25519PublicKey)

		got, err := crypto.Ed25519PublicToX25519(pub)
		if err != nil {
			t.Fatalf("Ed25519PublicToX25519 error: %v", err)
		}
		if hex.EncodeToString(got) != hex.EncodeToString(expectedX25519Pub) {
			t.Errorf("Ed25519PublicToX25519 mismatch for seed %s:\n  got  %x\n  want %x", v.Ed25519Seed, got, expectedX25519Pub)
		}
	}
}

func TestEd25519PrivateToX25519(t *testing.T) {
	vectors := loadIdentityVectors(t)
	for _, v := range vectors {
		seed := mustDecodeHex(t, v.Ed25519Seed)
		priv := ed25519.NewKeyFromSeed(seed)
		expectedX25519Priv := mustDecodeHex(t, v.X25519PrivateKey)

		got := crypto.Ed25519PrivateToX25519(priv)
		if hex.EncodeToString(got) != hex.EncodeToString(expectedX25519Priv) {
			t.Errorf("Ed25519PrivateToX25519 mismatch for seed %s:\n  got  %x\n  want %x", v.Ed25519Seed, got, expectedX25519Priv)
		}
	}
}

func TestEncryptWithKnownNonce(t *testing.T) {
	vectors := loadCryptoVectors(t)
	for _, v := range vectors {
		t.Run(v.Description, func(t *testing.T) {
			senderPriv := mustDecodeHex(t, v.SenderX25519Priv)
			recipPub := mustDecodeHex(t, v.RecipientX25519Pub)
			nonce := mustDecodeHex(t, v.Nonce)
			plaintext := mustDecodeHex(t, v.Plaintext)
			expectedCiphertext := mustDecodeHex(t, v.Ciphertext)

			var recipPubArr, senderPrivArr [32]byte
			copy(recipPubArr[:], recipPub)
			copy(senderPrivArr[:], senderPriv)

			var nonceArr [24]byte
			copy(nonceArr[:], nonce)

			got := crypto.EncryptWithNonce(plaintext, &recipPubArr, &senderPrivArr, &nonceArr)
			if hex.EncodeToString(got) != hex.EncodeToString(expectedCiphertext) {
				t.Errorf("EncryptWithNonce mismatch:\n  got  %x\n  want %x", got, expectedCiphertext)
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	vectors := loadCryptoVectors(t)
	for _, v := range vectors {
		t.Run(v.Description, func(t *testing.T) {
			senderPub := mustDecodeHex(t, v.SenderX25519Pub)
			recipPriv := mustDecodeHex(t, v.RecipientX25519Priv)
			ciphertext := mustDecodeHex(t, v.Ciphertext)
			expectedPlaintext := mustDecodeHex(t, v.Plaintext)

			var senderPubArr, recipPrivArr [32]byte
			copy(senderPubArr[:], senderPub)
			copy(recipPrivArr[:], recipPriv)

			got, err := crypto.Decrypt(ciphertext, &senderPubArr, &recipPrivArr)
			if err != nil {
				t.Fatalf("Decrypt error: %v", err)
			}
			if hex.EncodeToString(got) != hex.EncodeToString(expectedPlaintext) {
				t.Errorf("Decrypt mismatch:\n  got  %x\n  want %x", got, expectedPlaintext)
			}
		})
	}
}

func TestEncryptRandomNonce(t *testing.T) {
	var recipPub, senderPriv [32]byte
	// Use first crypto vector's keys
	vectors := loadCryptoVectors(t)
	copy(recipPub[:], mustDecodeHex(t, vectors[0].RecipientX25519Pub))
	copy(senderPriv[:], mustDecodeHex(t, vectors[0].SenderX25519Priv))

	plaintext := []byte("same plaintext")
	c1, err := crypto.Encrypt(plaintext, &recipPub, &senderPriv)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	c2, err := crypto.Encrypt(plaintext, &recipPub, &senderPriv)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if hex.EncodeToString(c1) == hex.EncodeToString(c2) {
		t.Error("encrypting same plaintext twice produced identical ciphertexts (nonce reuse!)")
	}
}

func TestNoncePrependedToCiphertext(t *testing.T) {
	vectors := loadCryptoVectors(t)
	v := vectors[0]

	senderPriv := mustDecodeHex(t, v.SenderX25519Priv)
	recipPub := mustDecodeHex(t, v.RecipientX25519Pub)
	nonce := mustDecodeHex(t, v.Nonce)
	plaintext := mustDecodeHex(t, v.Plaintext)

	var recipPubArr, senderPrivArr [32]byte
	copy(recipPubArr[:], recipPub)
	copy(senderPrivArr[:], senderPriv)

	var nonceArr [24]byte
	copy(nonceArr[:], nonce)

	sealed := crypto.EncryptWithNonce(plaintext, &recipPubArr, &senderPrivArr, &nonceArr)

	// Verify first 24 bytes are the nonce
	if len(sealed) < 24 {
		t.Fatalf("sealed too short: %d bytes", len(sealed))
	}
	if hex.EncodeToString(sealed[:24]) != hex.EncodeToString(nonce) {
		t.Errorf("first 24 bytes are not the nonce:\n  got  %x\n  want %x", sealed[:24], nonce)
	}
}
