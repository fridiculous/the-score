package daemon

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/fridiculous/the-score/internal/store"
)

func TestServeExitsWhenContextCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	server := NewWithStore(slog.Default(), store.New())
	go func() {
		done <- server.Serve(ctx, listener)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not exit after context cancellation")
	}
}
