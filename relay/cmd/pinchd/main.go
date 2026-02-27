package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/pinch-protocol/pinch/relay/internal/hub"
)

func main() {
	port := os.Getenv("PINCH_RELAY_PORT")
	if port == "" {
		port = "8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	h := hub.NewHub()
	go h.Run(ctx)

	r := chi.NewRouter()
	r.Get("/ws", wsHandler(ctx, h))
	r.Get("/health", healthHandler(h))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Start server in a goroutine so we can listen for shutdown signals.
	go func() {
		slog.Info("relay starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	slog.Info("shutting down relay")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("relay stopped")
}

// wsHandler handles WebSocket upgrade requests. The client provides its
// pinch: address via the ?address query parameter (temporary; Phase 2
// replaces this with cryptographic authentication).
func wsHandler(serverCtx context.Context, h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		address := r.URL.Query().Get("address")
		if address == "" {
			http.Error(w, "missing address query parameter", http.StatusBadRequest)
			return
		}

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// Allow connections from any origin in development.
			InsecureSkipVerify: true,
		})
		if err != nil {
			slog.Error("websocket accept error",
				"address", address,
				"error", err,
			)
			return
		}

		client := hub.NewClient(h, conn, address, serverCtx)
		h.Register(client)

		go client.ReadPump()
		go client.WritePump()
		go client.HeartbeatLoop()
	}
}

// healthHandler returns the current health status of the relay,
// including goroutine count and active connection count.
func healthHandler(h *hub.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := map[string]int{
			"goroutines":  runtime.NumGoroutine(),
			"connections": h.ClientCount(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}
