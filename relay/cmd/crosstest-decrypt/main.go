// Command go_decrypt reads JSON from stdin with Ed25519 seeds and sealed bytes,
// decrypts using NaCl box, and outputs the plaintext as JSON to stdout.
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
	Sealed        string `json:"sealed"`
}

type output struct {
	Plaintext string `json:"plaintext"`
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
	sealed, err := hex.DecodeString(in.Sealed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid sealed hex: %v\n", err)
		os.Exit(1)
	}

	senderPriv := ed25519.NewKeyFromSeed(senderSeed)
	senderPub := senderPriv.Public().(ed25519.PublicKey)
	recipientPriv := ed25519.NewKeyFromSeed(recipientSeed)

	senderX25519Pub, err := crypto.Ed25519PublicToX25519(senderPub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "key conversion error: %v\n", err)
		os.Exit(1)
	}
	recipientX25519Priv := crypto.Ed25519PrivateToX25519(recipientPriv)

	var senderPubArr, recipPrivArr [32]byte
	copy(senderPubArr[:], senderX25519Pub)
	copy(recipPrivArr[:], recipientX25519Priv)

	plaintext, err := crypto.Decrypt(sealed, &senderPubArr, &recipPrivArr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decrypt error: %v\n", err)
		os.Exit(1)
	}

	out := output{Plaintext: hex.EncodeToString(plaintext)}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode output: %v\n", err)
		os.Exit(1)
	}
}
