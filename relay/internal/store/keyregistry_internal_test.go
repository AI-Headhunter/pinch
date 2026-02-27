package store

import (
	"os"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func openInternalTestDB(t *testing.T) *bolt.DB {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "keyregistry-internal-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()

	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRegisterPending_RetriesWhenClaimCodeCollides(t *testing.T) {
	db := openInternalTestDB(t)
	kr, err := NewKeyRegistry(db)
	if err != nil {
		t.Fatalf("NewKeyRegistry: %v", err)
	}

	original := claimCodeGenerator
	t.Cleanup(func() {
		claimCodeGenerator = original
	})

	codes := []string{"deadbeef", "deadbeef", "cafebabe"}
	idx := 0
	claimCodeGenerator = func() (string, error) {
		code := codes[idx]
		idx++
		return code, nil
	}

	first, err := kr.RegisterPending("pubkey-1", "addr-1")
	if err != nil {
		t.Fatalf("RegisterPending first: %v", err)
	}
	if first != "deadbeef" {
		t.Fatalf("expected first code deadbeef, got %q", first)
	}

	second, err := kr.RegisterPending("pubkey-2", "addr-2")
	if err != nil {
		t.Fatalf("RegisterPending second: %v", err)
	}
	if second != "cafebabe" {
		t.Fatalf("expected second code cafebabe after collision retry, got %q", second)
	}

	gotFirst, err := kr.Claim("deadbeef")
	if err != nil {
		t.Fatalf("Claim deadbeef: %v", err)
	}
	if gotFirst != "addr-1" {
		t.Fatalf("expected addr-1, got %q", gotFirst)
	}

	gotSecond, err := kr.Claim("cafebabe")
	if err != nil {
		t.Fatalf("Claim cafebabe: %v", err)
	}
	if gotSecond != "addr-2" {
		t.Fatalf("expected addr-2, got %q", gotSecond)
	}
}
