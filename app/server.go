package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
)

var (
	listen = flag.String("listen", ":6379", "listen address")
)

func run(_ context.Context, _ io.Writer, _ []string) error {
	flag.Parse()
	l, err := net.Listen("tcp", *listen)
	defer l.Close()
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	slog.Info("listening", "port", l.Addr())
	c, err := l.Accept()
	if err != nil {
		return fmt.Errorf("failed to accept connection: %w", err)
	}
	defer c.Close()
	buf := make([]byte, 128)
	_, err = c.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read command: %w", err)
	}
	slog.Info("received command", "command", string(buf))
	_, err = c.Write([]byte("+PONG\r\n"))
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}
func main() {
	ctx := context.Background()
	if err := run(ctx, os.Stdout, os.Args); err != nil {
		log.Fatalf("error: %v", err)
	}
}
