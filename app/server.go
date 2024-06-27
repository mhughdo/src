package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server"
	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/codecrafters-io/redis-starter-go/pkg/telemetry/logger"
)

var (
	listen     = flag.String("listen", ":6379", "listen address")
	dir        = flag.String("dir", "/tmp/redis", "data directory")
	dbFilename = flag.String("dbfilename", "dump.rdb", "database filename")
)

func run(ctx context.Context, _ io.Writer, _ []string) error {
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	logger.Info(ctx, "oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo")
	flag.Parse()
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)
	cfg := config.NewConfig()
	err := cfg.SetBatch(map[string]string{
		config.ListenAddrKey: *listen,
		config.DirKey:        *dir,
		config.DBFilenameKey: *dbFilename,
	})
	if err != nil {
		return err
	}
	server := server.NewServer(cfg)
	go func() {
		if err := server.Start(ctx); err != nil {
			logger.Error(ctx, "failed to start the server, err: %v", err)
		}
		cancel()
	}()

	select {
	case <-ctx.Done():
		logger.Info(ctx, "context done, shutting down")
	case <-sigCh:
		logger.Info(ctx, "Received signal, shutting down")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Close(ctx); err != nil {
		logger.Error(ctx, "failed to close server, err: %v", err)
	}

	logger.Info(ctx, "Redis is now ready to exit, bye bye...")
	return nil
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Stdout, os.Args); err != nil {
		log.Fatalf("error: %v", err)
	}
}
