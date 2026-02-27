package store_test

import (
	"os"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/pinch-protocol/pinch/relay/internal/store"
)

func openTestDB(t *testing.T) *bolt.DB {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "keyregistry-*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	f.Close()
	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewKeyRegistry_CreatesSuccessfully(t *testing.T) {
	db := openTestDB(t)
	_, err := store.NewKeyRegistry(db)
	if err != nil {
		t.Fatalf("NewKeyRegistry: %v", err)
	}
}

func TestRegisterPending_ReturnsEightCharHexCode(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	code, err := kr.RegisterPending("dGVzdHB1YmtleQ==", "pinch:abc@relay.test")
	if err != nil {
		t.Fatalf("RegisterPending: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("expected 8-char code, got %q (len %d)", code, len(code))
	}
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char in claim code: %q", code)
			break
		}
	}
}

func TestRegisterPending_ProducesDifferentCodes(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	code1, _ := kr.RegisterPending("a", "addr1")
	code2, _ := kr.RegisterPending("b", "addr2")
	if code1 == code2 {
		t.Error("expected different claim codes for different registrations")
	}
}

func TestClaim_ApprovesAndRemovesPending(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	pubKeyB64 := "dGVzdHB1YmtleQ=="
	address := "pinch:abc@relay.test"

	code, err := kr.RegisterPending(pubKeyB64, address)
	if err != nil {
		t.Fatalf("RegisterPending: %v", err)
	}

	got, err := kr.Claim(code)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if got != address {
		t.Errorf("expected address %q, got %q", address, got)
	}

	// Key should now be approved.
	if !kr.IsApproved(pubKeyB64) {
		t.Error("expected key to be approved after claim")
	}

	// Claim code should be gone — second claim must fail.
	_, err = kr.Claim(code)
	if err == nil {
		t.Error("expected error on second claim of same code, got nil")
	}
}

func TestClaim_InvalidCodeReturnsError(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	_, err := kr.Claim("deadbeef")
	if err == nil {
		t.Error("expected error for unknown claim code, got nil")
	}
}

func TestIsApproved_FalseBeforeClaim(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	pubKeyB64 := "dGVzdHB1YmtleQ=="
	_, _ = kr.RegisterPending(pubKeyB64, "pinch:abc@relay.test")

	if kr.IsApproved(pubKeyB64) {
		t.Error("key should not be approved before claim")
	}
}

func TestIsApproved_TrueAfterClaim(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	pubKeyB64 := "dGVzdHB1YmtleQ=="
	address := "pinch:abc@relay.test"
	code, _ := kr.RegisterPending(pubKeyB64, address)
	_, _ = kr.Claim(code)

	if !kr.IsApproved(pubKeyB64) {
		t.Error("key should be approved after claim")
	}
}

func TestSweepPending_RemovesExpiredEntries(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	pubKeyB64 := "dGVzdHB1YmtleQ=="
	code, err := kr.RegisterPending(pubKeyB64, "pinch:abc@relay.test")
	if err != nil {
		t.Fatalf("RegisterPending: %v", err)
	}

	// Sweep with a TTL of 0 — all existing entries are immediately expired.
	if err := kr.SweepPending(0); err != nil {
		t.Fatalf("SweepPending: %v", err)
	}

	_, err = kr.Claim(code)
	if err == nil {
		t.Error("expected claim to fail after sweep removed the pending entry")
	}

	// Key must NOT appear as approved (it was swept, not claimed).
	if kr.IsApproved(pubKeyB64) {
		t.Error("swept key should not be approved")
	}
}

func TestSweepPending_PreservesNonExpiredEntries(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	pubKeyB64 := "dGVzdHB1YmtleQ=="
	address := "pinch:abc@relay.test"
	code, _ := kr.RegisterPending(pubKeyB64, address)

	// Sweep with a generous TTL — entry should survive.
	if err := kr.SweepPending(24 * time.Hour); err != nil {
		t.Fatalf("SweepPending: %v", err)
	}

	got, err := kr.Claim(code)
	if err != nil {
		t.Fatalf("expected claim to succeed after non-expiring sweep: %v", err)
	}
	if got != address {
		t.Errorf("expected address %q, got %q", address, got)
	}
}

func TestSweepPending_ReturnsErrorWhenDBClosed(t *testing.T) {
	db := openTestDB(t)
	kr, _ := store.NewKeyRegistry(db)

	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	if err := kr.SweepPending(time.Hour); err == nil {
		t.Fatal("expected error from SweepPending on closed database")
	}
}
