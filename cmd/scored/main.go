package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fridiculous/the-score/internal/daemon"
	"github.com/fridiculous/the-score/internal/ipc"
)

func main() {
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	listener, address, err := ipc.ListenDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "scored: listen failed: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		<-signals
		cancel()
		_ = listener.Close()
	}()
	defer cancel()

	server := daemon.New(logger)
	server.SetShutdown(func() {
		cancel()
		_ = listener.Close()
	})
	logger.Info("scored listening", "address", address)
	if err := server.Serve(ctx, listener); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "scored: %v\n", err)
		os.Exit(1)
	}
}
