package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
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
	ln             net.Listener
	mu             sync.Mutex
	cfg            *config.Config
	done           chan struct{}
	store          keyval.KV
	clients        map[*client.Client]struct{}
	cFactory       *command.CommandFactory
	messageChan    chan client.Message
	disconnectChan chan *client.Client
}

func NewServer(cfg *config.Config) *Server {
	store := keyval.NewStore()
	return &Server{
		mu:             sync.Mutex{},
		cfg:            cfg,
		done:           make(chan struct{}),
		store:          store,
		cFactory:       command.NewCommandFactory(store, cfg),
		clients:        make(map[*client.Client]struct{}),
		messageChan:    make(chan client.Message),
		disconnectChan: make(chan *client.Client),
	}
}

func (s *Server) Start(ctx context.Context) error {
	logger.Info(ctx, "Server initialized")
	dir, _ := s.cfg.Get(config.DirKey)
	dbFilename, _ := s.cfg.Get(config.DBFilenameKey)
	file, openErr := os.Open(dir + "/" + dbFilename)
	if openErr != nil && !os.IsNotExist(openErr) {
		return fmt.Errorf("failed to open file, err: %v", openErr)
	}
	var isValidRDBFile bool
	var stat os.FileInfo
	var err error
	if !os.IsNotExist(openErr) {
		stat, err = file.Stat()
		if err != nil {
			return fmt.Errorf("failed to get file stat, err: %v", err)
		}
		isValidRDBFile = !stat.IsDir()
	}

	if isValidRDBFile {
		defer file.Close()
		rdb := rdb.NewRDBParser(file)
		if err := rdb.ParseRDB(ctx); err != nil {
			return fmt.Errorf("failed to parse rdb, err: %v", err)
		}
		s.store.RestoreRDB(rdb.GetData())
	}
	if err := s.Listen(ctx); err != nil {
		return fmt.Errorf("failed to listen, err: %v", err)
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
	go s.loop(ctx)
	return s.acceptConnection(ctx)
}

func (s *Server) acceptConnection(ctx context.Context) error {
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
		cl := client.NewClient(conn, s.messageChan)
		s.mu.Lock()
		s.clients[cl] = struct{}{}
		s.mu.Unlock()
		go cl.HandleConnection(ctx)
	}
}

func (s *Server) loop(ctx context.Context) {
	for {
		select {
		case <-s.done:
			return
		case cl := <-s.disconnectChan:
			s.closeClient(ctx, cl)
		case msg := <-s.messageChan:
			ctx := context.WithValue(ctx, logger.RemoteAddrKey, msg.Client.RemoteAddr())
			err := s.handleMessage(ctx, msg.Client, msg.Resp)
			if err != nil {
				logger.Error(ctx, "failed to handle message, err: %v", err)
				if strings.Contains(err.Error(), "failed to write error:") {
					s.closeClient(ctx, msg.Client)
					continue
				}
			}
			err = msg.Client.Send()
			if err != nil {
				logger.Error(ctx, "failed to send message, err: %v", err)
				s.closeClient(ctx, msg.Client)
			}
		}
	}
}

func (s *Server) closeClient(ctx context.Context, cl *client.Client) {
	s.mu.Lock()
	delete(s.clients, cl)
	s.mu.Unlock()
	cl.Close(ctx)
}

func (s *Server) handleMessage(ctx context.Context, cl *client.Client, r *resp.Resp) error {
	writer := cl.Writer
	cmdName := r.Data.([]*resp.Resp)[0].String()
	args := r.Data.([]*resp.Resp)[1:]
	logger.Info(ctx, "received command, cmd: %s, args: %v", cmdName, args)
	cmd, err := s.cFactory.GetCommand(cmdName)
	if err != nil {
		return s.writeError(cl, err)
	}
	isBlocking := cmd.IsBlocking(args)
	if isBlocking {
		go s.handleBlockingCommand(ctx, cl, cmd, writer, args)
		return nil
	}
	err = cmd.Execute(cl, writer, args)
	if err != nil {
		cl.Writer.Reset()
		return s.writeError(cl, err)
	}

	return nil
}

func (s *Server) handleBlockingCommand(ctx context.Context, cl *client.Client, cmd command.Command, writer *resp.Writer, args []*resp.Resp) {
	err := cmd.Execute(cl, writer, args)
	if err != nil {
		cl.Writer.Reset()
		err = s.writeError(cl, err)
	}
	if err != nil {
		logger.Error(ctx, "failed to handle message, err: %v", err)
		if strings.Contains(err.Error(), "failed to write error:") {
			s.closeClient(ctx, cl)
			return
		}
	}

	err = cl.Send()
	if err != nil {
		logger.Error(ctx, "failed to send message, err: %v", err)
		s.closeClient(ctx, cl)
	}
}

func (s *Server) writeError(cl *client.Client, err error) error {
	writeErr := cl.Writer.WriteError(err)
	if writeErr != nil {
		return errors.Join(err, fmt.Errorf("failed to write error: %v", writeErr))
	}
	return err
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
