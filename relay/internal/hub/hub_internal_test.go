package hub

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pinch-protocol/pinch/relay/internal/store"
)

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
