package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
)

const (
	defaultListenAddr = ":6379"
)

type Config struct {
	ListenAddr string
}

type Server struct {
	cfg  Config
	ln   net.Listener
	done chan struct{}
}

func NewServer(cfg Config) *Server {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	return &Server{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

func (s *Server) Listen() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	slog.Info("listening", "port", ln.Addr())
	if err != nil {
		return err
	}
	s.ln = ln
	return s.loop()
}

func (s *Server) loop() error {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				slog.Info("shutting down listener")
				return nil
			default:
				return fmt.Errorf("failed to accept connection: %w", err)
			}
		}
		go s.handleConn(c)
	}
}

func (s *Server) handleConn(c net.Conn) {
	for {
		buf := make([]byte, 128)
		_, err := c.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				slog.Info("connection closed")
				return
			}
			slog.Error("failed to read command", "err", err)
			continue
		}
		if len(buf) == 0 {
			continue
		}
		slog.Info("received command", "command", string(buf))
		_, err = c.Write([]byte("+PONG\r\n"))
		if err != nil {
			slog.Error("failed to write response", "err", err)
			continue
		}
	}
}

func (s *Server) Close(_ context.Context) error {
	close(s.done)
	if err := s.ln.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}
	return nil
}
