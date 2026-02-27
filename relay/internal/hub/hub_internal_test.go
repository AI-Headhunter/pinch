package hub

import (
	"context"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	pinchv1 "github.com/pinch-protocol/pinch/gen/go/pinch/v1"
	"github.com/pinch-protocol/pinch/relay/internal/identity"
	"github.com/pinch-protocol/pinch/relay/internal/store"
	"google.golang.org/protobuf/proto"
)

func marshalMessageEnvelope(t *testing.T, fromAddr, toAddr string) []byte {
	t.Helper()
	env := &pinchv1.Envelope{
		Version:     1,
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Type:        pinchv1.MessageType_MESSAGE_TYPE_MESSAGE,
	}
	data, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	return data
}

func TestFlushQueuedMessagesDoesNotRemoveWhenSendBufferIsFull(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "flush-buffer-full.db")
	db, err := store.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mq, err := store.NewMessageQueue(db, 1000, time.Hour)
	if err != nil {
		t.Fatalf("NewMessageQueue: %v", err)
	}

	if err := mq.Enqueue("pinch:bob@localhost", "pinch:alice@localhost", []byte("queued-message")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	h := NewHub(nil, mq, nil)
	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()

	client := &Client{
		hub:     h,
		address: "pinch:bob@localhost",
		send:    make(chan []byte, 1),
		ctx:     clientCtx,
		cancel:  clientCancel,
	}

	// Saturate outbound buffer so flush cannot enqueue immediately.
	client.send <- []byte("buffer-occupied")

	go h.flushQueuedMessages(client)

	time.Sleep(150 * time.Millisecond)
	if got := mq.Count("pinch:bob@localhost"); got != 1 {
		t.Fatalf("expected queued message to remain while send buffer is full, got count=%d", got)
	}

	// Free one slot and wait for flush to enqueue+remove the entry.
	<-client.send

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mq.Count("pinch:bob@localhost") == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected queued message to be removed after buffer space became available")
}

func TestRouteMessageDropsInvalidRecipientAddressWhenRelayHostConfigured(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "route-invalid-recipient.db")
	db, err := store.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mq, err := store.NewMessageQueue(db, 1000, time.Hour)
	if err != nil {
		t.Fatalf("NewMessageQueue: %v", err)
	}

	h := NewHub(nil, mq, nil)
	h.SetRelayHost("localhost")
	from := &Client{address: "pinch:sender@localhost"}

	invalidAddr := "pinch:abc@localhost"
	if err := h.RouteMessage(from, marshalMessageEnvelope(t, from.address, invalidAddr)); err != nil {
		t.Fatalf("RouteMessage invalid: %v", err)
	}
	if got := mq.Count(invalidAddr); got != 0 {
		t.Fatalf("expected invalid recipient to be dropped, got queued=%d", got)
	}

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	wrongHostAddr := identity.GenerateAddress(pub, "otherhost")
	if err := h.RouteMessage(from, marshalMessageEnvelope(t, from.address, wrongHostAddr)); err != nil {
		t.Fatalf("RouteMessage wrong-host: %v", err)
	}
	if got := mq.Count(wrongHostAddr); got != 0 {
		t.Fatalf("expected wrong-host recipient to be dropped, got queued=%d", got)
	}
}

func TestRouteMessageEnqueuesValidRecipientAddressWhenRelayHostConfigured(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "route-valid-recipient.db")
	db, err := store.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mq, err := store.NewMessageQueue(db, 1000, time.Hour)
	if err != nil {
		t.Fatalf("NewMessageQueue: %v", err)
	}

	h := NewHub(nil, mq, nil)
	h.SetRelayHost("localhost")
	from := &Client{address: "pinch:sender@localhost"}

	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	validAddr := identity.GenerateAddress(pub, "localhost")
	if err := h.RouteMessage(from, marshalMessageEnvelope(t, from.address, validAddr)); err != nil {
		t.Fatalf("RouteMessage valid: %v", err)
	}
	if got := mq.Count(validAddr); got != 1 {
		t.Fatalf("expected valid recipient to enqueue once, got queued=%d", got)
	}
}
