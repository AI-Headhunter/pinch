// Package hub implements a hub-and-spoke WebSocket connection manager.
// The Hub goroutine maintains a routing table mapping pinch: addresses
// to active WebSocket connections, with channels for registration and
// unregistration of clients.
package hub

import (
	"context"
	"log/slog"
	"sync"
)

// Hub maintains the set of active clients and routes messages between them.
// A single Hub goroutine serializes access to the routing table via channels.
type Hub struct {
	// clients maps pinch: addresses to active Client connections.
	clients map[string]*Client

	// register receives clients to add to the routing table.
	register chan *Client

	// unregister receives clients to remove from the routing table.
	unregister chan *Client

	// mu protects external reads of the routing table (e.g., health checks).
	mu sync.RWMutex
}

// NewHub creates a new Hub with initialized channels and routing table.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main event loop. It processes register and unregister
// events until the context is cancelled. Run should be called in its own
// goroutine.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.address] = client
			h.mu.Unlock()
			slog.Info("client registered",
				"address", client.address,
				"connections", h.ClientCount(),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.address]; ok {
				delete(h.clients, client.address)
				close(client.send)
				client.cancel()
			}
			h.mu.Unlock()
			slog.Info("client unregistered",
				"address", client.address,
				"connections", h.ClientCount(),
			)

		case <-ctx.Done():
			h.mu.Lock()
			for addr, client := range h.clients {
				close(client.send)
				client.cancel()
				delete(h.clients, addr)
			}
			h.mu.Unlock()
			slog.Info("hub stopped")
			return
		}
	}
}

// ClientCount returns the number of currently connected clients.
// It is safe for concurrent use.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// LookupClient returns the client registered with the given address.
// Returns the client and true if found, or nil and false otherwise.
// It is safe for concurrent use.
func (h *Hub) LookupClient(address string) (*Client, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[address]
	return c, ok
}

// Register queues a client for registration with the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister queues a client for removal from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}
