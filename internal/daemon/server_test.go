package daemon

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"
)

func TestServeExitsWhenContextCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- New(slog.Default()).Serve(ctx, listener)
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
