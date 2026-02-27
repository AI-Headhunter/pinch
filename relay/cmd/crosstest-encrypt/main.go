// Command go_encrypt reads JSON from stdin with Ed25519 seeds and plaintext,
// encrypts using NaCl box, and outputs the sealed bytes as JSON to stdout.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pinch-protocol/pinch/relay/internal/crypto"
)

type input struct {
	SenderSeed    string `json:"ed25519_seed_sender"`
	RecipientSeed string `json:"ed25519_seed_recipient"`
	Plaintext     string `json:"plaintext"`
}

type output struct {
	Sealed string `json:"sealed"`
}

func main() {
	var in input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		fmt.Fprintf(os.Stderr, "failed to decode input: %v\n", err)
		os.Exit(1)
	}

	senderSeed, err := hex.DecodeString(in.SenderSeed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid sender seed hex: %v\n", err)
		os.Exit(1)
	}
	recipientSeed, err := hex.DecodeString(in.RecipientSeed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid recipient seed hex: %v\n", err)
		os.Exit(1)
	}
	plaintext, err := hex.DecodeString(in.Plaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid plaintext hex: %v\n", err)
		os.Exit(1)
	}

	senderPriv := ed25519.NewKeyFromSeed(senderSeed)
	recipientPriv := ed25519.NewKeyFromSeed(recipientSeed)
	recipientPub := recipientPriv.Public().(ed25519.PublicKey)

	senderX25519Priv := crypto.Ed25519PrivateToX25519(senderPriv)
	recipientX25519Pub, err := crypto.Ed25519PublicToX25519(recipientPub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "key conversion error: %v\n", err)
		os.Exit(1)
	}

	var recipPubArr, senderPrivArr [32]byte
	copy(recipPubArr[:], recipientX25519Pub)
	copy(senderPrivArr[:], senderX25519Priv)

	sealed, err := crypto.Encrypt(plaintext, &recipPubArr, &senderPrivArr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encrypt error: %v\n", err)
		os.Exit(1)
	}

	out := output{Sealed: hex.EncodeToString(sealed)}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode output: %v\n", err)
		os.Exit(1)
	}
}
