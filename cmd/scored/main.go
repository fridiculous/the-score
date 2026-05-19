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
	"github.com/fridiculous/the-score/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("scored %s api=%s sourcePacks=%s commit=%s\n", version.Version, version.APIVersion, version.SourcePackVersion, version.BuildCommit)
		return
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	server, err := daemon.New(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scored: store initialization failed: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

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
