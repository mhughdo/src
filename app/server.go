package main

import (
	"context"
	"flag"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server"
)

var (
	listen = flag.String("listen", ":6379", "listen address")
)

func run(_ context.Context, _ io.Writer, _ []string) error {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)
	server := server.NewServer(server.Config{ListenAddr: *listen})
	go func() {
		if err := server.Listen(); err != nil {
			slog.Error("failed to listen", "err", err)
		}
		cancel()
	}()

	select {
	case <-ctx.Done():
		slog.Info("context done, shutting down")
	case <-sigCh:
		slog.Info("received signal, shutting down")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Close(ctx); err != nil {
		slog.Error("failed to close server", "err", err)
	}

	slog.Info("server shutdown")
	return nil
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Stdout, os.Args); err != nil {
		log.Fatalf("error: %v", err)
	}
}
