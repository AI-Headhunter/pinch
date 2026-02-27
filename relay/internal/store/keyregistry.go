package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	pendingRegistryBucket = []byte("pending_registry")
	keyRegistryBucket     = []byte("key_registry")

	ErrClaimNotFound = errors.New("claim code not found or expired")
)

type pendingEntry struct {
	PubKeyB64    string `json:"pubKeyB64"`
	Address      string `json:"address"`
	RegisteredAt int64  `json:"registeredAt"` // Unix seconds
}

// KeyRegistry is a bbolt-backed store for pending and approved agent key registrations.
type KeyRegistry struct {
	db *bolt.DB
}

// NewKeyRegistry creates or opens the key registry buckets in the given database.
func NewKeyRegistry(db *bolt.DB) (*KeyRegistry, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(pendingRegistryBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(keyRegistryBucket)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &KeyRegistry{db: db}, nil
}

// RegisterPending stores a pending registration and returns an 8-character hex claim code.
func (kr *KeyRegistry) RegisterPending(pubKeyB64, address string) (string, error) {
	var code [4]byte
	if _, err := rand.Read(code[:]); err != nil {
		return "", err
	}
	claimCode := hex.EncodeToString(code[:])

	entry := pendingEntry{
		PubKeyB64:    pubKeyB64,
		Address:      address,
		RegisteredAt: time.Now().Unix(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}

	err = kr.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(pendingRegistryBucket)
		return b.Put([]byte(claimCode), data)
	})
	if err != nil {
		return "", err
	}
	return claimCode, nil
}

// Claim approves a pending registration by claim code, moving it to the approved registry.
// Returns the approved address or ErrClaimNotFound if the claim code does not exist.
func (kr *KeyRegistry) Claim(claimCode string) (string, error) {
	var address string

	err := kr.db.Update(func(tx *bolt.Tx) error {
		pending := tx.Bucket(pendingRegistryBucket)
		data := pending.Get([]byte(claimCode))
		if data == nil {
			return ErrClaimNotFound
		}

		var entry pendingEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}
		address = entry.Address

		approved := tx.Bucket(keyRegistryBucket)
		if err := approved.Put([]byte(entry.PubKeyB64), []byte(entry.Address)); err != nil {
			return err
		}
		return pending.Delete([]byte(claimCode))
	})
	if err != nil {
		return "", err
	}
	return address, nil
}

// IsApproved reports whether the given base64-encoded public key is in the approved registry.
func (kr *KeyRegistry) IsApproved(pubKeyB64 string) bool {
	var found bool
	_ = kr.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(keyRegistryBucket)
		found = b.Get([]byte(pubKeyB64)) != nil
		return nil
	})
	return found
}

// SweepPending removes pending registrations older than the given TTL.
// Call once on startup to clean up stale entries.
func (kr *KeyRegistry) SweepPending(ttl time.Duration) {
	cutoff := time.Now().Add(-ttl).Unix()
	_ = kr.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(pendingRegistryBucket)
		var toDelete [][]byte
		_ = b.ForEach(func(k, v []byte) error {
			var entry pendingEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				// Malformed entry — delete it.
				toDelete = append(toDelete, append([]byte{}, k...))
				return nil
			}
			if entry.RegisteredAt <= cutoff {
				toDelete = append(toDelete, append([]byte{}, k...))
			}
			return nil
		})
		for _, k := range toDelete {
			_ = b.Delete(k)
		}
		return nil
	})
}
