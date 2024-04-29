package server

import (
	"context"
	"fmt"
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
	cfg Config
	ln  net.Listener
}

func NewServer(cfg Config) *Server {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	return &Server{
		cfg: cfg,
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
	c, err := s.ln.Accept()
	if err != nil {
		return fmt.Errorf("failed to accept connection: %w", err)
	}
	buf := make([]byte, 0, 1024)
	for {
		_, err = c.Read(buf)
		if err != nil {
			slog.Error("failed to read command", "err", err)
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
	if err := s.ln.Close(); err != nil {
		return err
	}
	return nil
}
