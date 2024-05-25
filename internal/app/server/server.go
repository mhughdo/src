package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/codecrafters-io/redis-starter-go/pkg/command"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
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
	defer c.Close()

	cFactory := command.NewCommandFactory()
	reader := bufio.NewReader(c)
	buffer := &bytes.Buffer{}
	for {
		part, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				slog.Info("connection closed", "addr", c.RemoteAddr())
				return
			}
			slog.Error("failed to receive input", "err", err)
			continue
		}
		buffer.Write(part)
		reader := resp.NewResp(bytes.NewReader(buffer.Bytes()))
		for {
			r, err := reader.ParseResp()
			if err != nil {
				if errors.Is(err, resp.ErrEmptyLine) {
					break
				}
				if errors.Is(err, resp.ErrIncompleteInput) || errors.Is(err, resp.ErrUnknownType) || errors.Is(err, io.EOF) {
					break
				}
				err := resp.ErrResponse(c, buffer, err.Error())
				if err != nil {
					slog.Error("failed to write response", "err", err)
					return
				}
				break
			}
			if r.Type != resp.Array || len(r.Data.([]*resp.Resp)) < 1 {
				err := resp.ErrResponse(c, buffer, "invalid command format")
				if err != nil {
					slog.Error("failed to write response", "err", err)
					return
				}
				continue
			}
			cmdName := r.Data.([]*resp.Resp)[0].String()
			args := r.Data.([]*resp.Resp)[1:]
			cmd, err := cFactory.GetCommand(cmdName)

			if err != nil {
				// resp.ErrRespone(c, err.Error())
				// TODO: handle error
				_, err := c.Write([]byte("+OK\r\n"))
				buffer.Reset()
				if err != nil {
					slog.Error("failed to write response", "err", err)
					return
				}
				continue
			}
			res, err := cmd.Execute(args)
			if err != nil {
				err := resp.ErrResponse(c, buffer, fmt.Sprintf("failed to execute command: %s", err))
				if err != nil {
					slog.Error("failed to write response", "err", err)
					return
				}
				continue
			}
			_, err = c.Write(res.ToResponse())
			buffer.Reset()
			if err != nil {
				slog.Error("failed to write response", "err", err)
				return
			}
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
