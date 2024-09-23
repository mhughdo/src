package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mhughdo/src/internal/app/server"
	"github.com/mhughdo/src/internal/app/server/config"
	"github.com/mhughdo/src/pkg/telemetry/logger"
)

var (
	port       = flag.String("port", "6379", "listening port")
	dir        = flag.String("dir", "/tmp/redis", "data directory")
	dbFilename = flag.String("dbfilename", "dump.rdb", "database filename")
	replicaOf  = flag.String("replicaof", "", "replica of another redis server")
)

func run(ctx context.Context, _ io.Writer, _ []string) error {
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	logger.Info(ctx, "oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo")
	flag.Parse()
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer signal.Stop(sigCh)
	cfg := config.NewConfig()
	if *replicaOf != "" && len(strings.Split(*replicaOf, " ")) != 2 {
		return fmt.Errorf("invalid replicaof flag, expected format: replicaof <host> <port>")
	}
	err := cfg.SetBatch(map[string]string{
		config.ListenAddrKey: fmt.Sprintf(":%s", *port),
		config.DirKey:        *dir,
		config.DBFilenameKey: *dbFilename,
		config.ReplicaOfKey:  *replicaOf,
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
