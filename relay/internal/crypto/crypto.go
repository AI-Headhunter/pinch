// Package crypto provides NaCl box encryption/decryption and Ed25519-to-X25519
// key conversion for the Pinch protocol.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"fmt"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/nacl/box"
)

// Ed25519PublicToX25519 converts an Ed25519 public key to the birationally
// equivalent X25519 (Curve25519) public key using the Montgomery form.
func Ed25519PublicToX25519(pub ed25519.PublicKey) ([]byte, error) {
	point, err := new(edwards25519.Point).SetBytes(pub)
	if err != nil {
		return nil, fmt.Errorf("invalid Ed25519 public key: %w", err)
	}
	return point.BytesMontgomery(), nil
}

// Ed25519PrivateToX25519 derives an X25519 private key from an Ed25519 private
// key by hashing the seed with SHA-512 and clamping per RFC 7748.
func Ed25519PrivateToX25519(priv ed25519.PrivateKey) []byte {
	h := sha512.New()
	h.Write(priv.Seed())
	digest := h.Sum(nil)
	// Clamp per RFC 7748
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64
	return digest[:32]
}

// Encrypt encrypts plaintext using NaCl box with a random 24-byte nonce.
// The nonce is prepended to the ciphertext in the returned byte slice.
func Encrypt(plaintext []byte, recipientX25519Pub, senderX25519Priv *[32]byte) ([]byte, error) {
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}
	sealed := box.Seal(nonce[:], plaintext, &nonce, recipientX25519Pub, senderX25519Priv)
	return sealed, nil
}

// Decrypt decrypts sealed bytes produced by Encrypt. It expects the first
// 24 bytes to be the nonce, followed by the NaCl box ciphertext.
func Decrypt(sealed []byte, senderX25519Pub, recipientX25519Priv *[32]byte) ([]byte, error) {
	if len(sealed) < 24+box.Overhead {
		return nil, errors.New("sealed data too short")
	}
	var nonce [24]byte
	copy(nonce[:], sealed[:24])
	plaintext, ok := box.Open(nil, sealed[24:], &nonce, senderX25519Pub, recipientX25519Priv)
	if !ok {
		return nil, errors.New("decryption failed: authentication error")
	}
	return plaintext, nil
}

// EncryptWithNonce encrypts plaintext with a specified nonce. This is intended
// for test vector validation only. Production code should use Encrypt which
// generates a random nonce.
func EncryptWithNonce(plaintext []byte, recipientX25519Pub, senderX25519Priv *[32]byte, nonce *[24]byte) []byte {
	return box.Seal(nonce[:], plaintext, nonce, recipientX25519Pub, senderX25519Priv)
}
