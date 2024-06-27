package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/command"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/rdb"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
	"github.com/codecrafters-io/redis-starter-go/pkg/telemetry/logger"
)

const (
	defaultListenAddr = ":6379"
)

type Server struct {
	ln       net.Listener
	mu       sync.Mutex
	cfg      *config.Config
	done     chan struct{}
	store    keyval.KV
	clients  map[*client.Client]struct{}
	cFactory *command.CommandFactory
}

func NewServer(cfg *config.Config) *Server {
	store := keyval.NewStore()
	return &Server{
		mu:       sync.Mutex{},
		cfg:      cfg,
		done:     make(chan struct{}),
		store:    store,
		cFactory: command.NewCommandFactory(store, cfg),
		clients:  make(map[*client.Client]struct{}),
	}
}

func (s *Server) Start(ctx context.Context) error {
	logger.Info(ctx, "Server initialized")
	dir, _ := s.cfg.Get(config.DirKey)
	dbFilename, _ := s.cfg.Get(config.DBFilenameKey)
	file, openErr := os.Open(dir + "/" + dbFilename)
	if openErr != nil && !os.IsNotExist(openErr) {
		return openErr
	}
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.IsNotExist(openErr) && !stat.IsDir() {
		defer file.Close()
		rdb := rdb.NewRDBParser(file)
		if err := rdb.ParseRDB(ctx); err != nil {
			return err
		}
		s.store.RestoreRDB(rdb.GetData(), rdb.GetExpiry())
	}

	if err := s.Listen(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Server) Listen(ctx context.Context) error {
	listenAddr, err := s.cfg.Get(config.ListenAddrKey)
	if err != nil {
		listenAddr = defaultListenAddr
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s.ln = ln
	logger.Info(ctx, "Ready to accept connections tcp on %s", listenAddr)
	return s.loop(ctx)
}

func (s *Server) loop(ctx context.Context) error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				return fmt.Errorf("failed to accept connection: %w", err)
			}
		}
		cl := client.NewClient(conn)
		s.mu.Lock()
		s.clients[cl] = struct{}{}
		s.mu.Unlock()
		ctx := context.WithValue(ctx, logger.RemoteAddrKey, conn.RemoteAddr())
		go s.handleClient(ctx, cl)
	}
}

func (s *Server) handleClient(ctx context.Context, cl *client.Client) {
	defer func(c *client.Client) {
		s.mu.Lock()
		delete(s.clients, c)
		s.mu.Unlock()
		cl.Close(ctx)
	}(cl)

	go cl.HandleConnection(ctx)

	for {
		select {
		case <-cl.DisconnectChan():
			return
		case msg := <-cl.MessageChan():
			err := s.handleMessage(ctx, cl, msg)
			if err != nil {
				logger.Error(ctx, "failed to handle message, err: %v", err)
			}
			err = cl.Send()
			if err != nil {
				logger.Error(ctx, "failed to send message, err: %v", err)
				return
			}
		}
	}
}

func (s *Server) handleMessage(ctx context.Context, cl *client.Client, r *resp.Resp) error {
	writer := cl.Writer
	cmdName := r.Data.([]*resp.Resp)[0].String()
	args := r.Data.([]*resp.Resp)[1:]
	logger.Info(ctx, "received command, cmd: %s, args: %v", cmdName, args)
	cmd, err := s.cFactory.GetCommand(cmdName)
	if err != nil {
		writeErr := cl.Writer.WriteError(err)
		if writeErr != nil {
			return fmt.Errorf("failed to write error: %v, err: %w", writeErr, err)
		}
		return err
	}
	err = cmd.Execute(cl, writer, args)
	if err != nil {
		cl.Writer.Reset()
		writeErr := cl.Writer.WriteError(err)
		if writeErr != nil {
			return fmt.Errorf("failed to write error: %v, err: %w", writeErr, err)
		}
		return err
	}

	return nil
}

func (s *Server) Close(_ context.Context) error {
	close(s.done)
	if s.ln == nil {
		return nil
	}
	if err := s.ln.Close(); err != nil {
		return fmt.Errorf("failed to close listener: %w", err)
	}
	return nil
}
