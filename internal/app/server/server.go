package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/command"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/rdb"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
	"github.com/codecrafters-io/redis-starter-go/pkg/telemetry/logger"
	"github.com/codecrafters-io/redis-starter-go/pkg/utils"
)

const (
	defaultListenAddr      = ":6379"
	replicaPingInterval    = 45 * time.Second
	replconfGetAckInterval = 30 * time.Second
)

type Server struct {
	ln                net.Listener
	mu                sync.Mutex
	cfg               *config.Config
	done              chan struct{}
	store             keyval.KV
	offset            uint64
	queueMu           sync.Mutex
	clients           map[*client.Client]struct{}
	cFactory          *command.CommandFactory
	isMaster          bool
	replicas          map[*client.Client]struct{}
	masterAddr        string
	messageChan       chan client.Message
	commandChan       chan commandRequest
	masterClient      *client.Client
	replicationID     string
	disconnectChan    chan *client.Client
	responseWaitQueue []*responseRequest
}

type commandRequest struct {
	cmd    []byte
	respCh chan *resp.Resp
}

type responseRequest struct {
	respCh chan *resp.Resp
}

func NewServer(cfg *config.Config) *Server {
	store := keyval.NewStore()
	replicaOf, _ := cfg.Get(config.ReplicaOfKey)
	isMaster := replicaOf == ""
	var masterAddr string
	if !isMaster {
		parts := strings.Split(replicaOf, " ")
		masterAddr = fmt.Sprintf("%s:%s", parts[0], parts[1])
	}
	var replicationID string
	if isMaster {
		replicationID = utils.GenerateRandomAlphanumeric(40)
	}
	s := &Server{
		mu:                sync.Mutex{},
		cfg:               cfg,
		done:              make(chan struct{}),
		store:             store,
		clients:           make(map[*client.Client]struct{}),
		isMaster:          isMaster,
		replicas:          make(map[*client.Client]struct{}),
		masterAddr:        masterAddr,
		messageChan:       make(chan client.Message),
		commandChan:       make(chan commandRequest),
		masterClient:      nil,
		replicationID:     replicationID,
		disconnectChan:    make(chan *client.Client),
		responseWaitQueue: []*responseRequest{},
	}
	s.cFactory = command.NewCommandFactory(store, cfg, s)
	return s
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

	if !s.isMaster {
		conn, err := net.Dial("tcp", s.masterAddr)
		if err != nil {
			return fmt.Errorf("failed to connect to master, err: %v", err)
		}
		s.masterClient = client.NewClient(conn, nil)
		go s.commandSender(ctx)
		go s.handleMasterConnection(ctx)
		if err := s.startReplication(ctx); err != nil {
			return fmt.Errorf("failed to start replication: %v", err)
		}
	} else {
		go s.pingReplicas(ctx)
		go s.sendReplConfGetAck(ctx)
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

func (s *Server) handleMasterConnection(ctx context.Context) {
	defer func() {
		if s.masterClient != nil {
			s.masterClient.Close(ctx)
		}
	}()
	reader := bufio.NewReader(s.masterClient.Conn())
	buffer := &bytes.Buffer{}

	for {
		select {
		case <-s.done:
			return
		default:
			part, err := reader.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
					logger.Info(ctx, "Master connection closed")
					return
				}
				logger.Error(ctx, "Error reading from master: %v", err)
				return
			}

			buffer.Write(part)
			respReader := resp.NewResp(bytes.NewReader(buffer.Bytes()))
			r, err := respReader.ParseResp()
			if err != nil {
				if strings.Contains(err.Error(), "unexpected EOF") {
					continue
				}
				if errors.Is(err, resp.ErrEmptyLine) {
					continue
				}
				if errors.Is(err, resp.ErrIncompleteInput) || errors.Is(err, resp.ErrUnknownType) || errors.Is(err, io.EOF) {
					continue
				}
				logger.Error(ctx, "Failed to parse response from master: %v", err)
				buffer.Reset()
				continue
			}
			s.dispatchResponse(r)
			s.handleMasterResponse(ctx, r, reader)
			if r.Type == resp.Array {
				s.offset += uint64(len(buffer.Bytes()))
			}
			buffer.Reset()
		}
	}
}

func (s *Server) receiveRDB(ctx context.Context, reader *bufio.Reader) {
	for {
		select {
		case <-s.done:
			return
		default:
			header, err := reader.ReadBytes('\n')
			if err != nil {
				logger.Error(ctx, "Failed to read RDB header: %v", err)
				return
			}

			if !bytes.HasPrefix(header, []byte("$")) {
				logger.Error(ctx, "Invalid RDB header: %s", header)
				return
			}

			lengthStr := string(header[1 : len(header)-2])
			length, err := strconv.Atoi(lengthStr)
			if err != nil {
				logger.Error(ctx, "Invalid RDB length: %v", err)
				return
			}

			rdbData := make([]byte, length)
			_, err = io.ReadFull(reader, rdbData)
			if err != nil {
				logger.Error(ctx, "Failed to read RDB data: %v", err)
				return
			}
			rdbParser := rdb.NewRDBParser(bytes.NewReader(rdbData))
			if err := rdbParser.ParseRDB(ctx); err != nil {
				logger.Error(ctx, "Failed to parse RDB data: %v", err)
				return
			}
			storeData := rdbParser.GetData()
			if len(storeData) == 0 {
				logger.Info(ctx, "Received empty RDB file from master")
				return
			}
			s.store.RestoreRDB(rdbParser.GetData())
			logger.Info(ctx, "Successfully received and restored RDB file from master")
			return
		}
	}
}

func (s *Server) handleMasterResponse(ctx context.Context, r *resp.Resp, reader *bufio.Reader) {
	switch r.Type {
	case resp.SimpleString:
		if strings.Contains(r.String(), "FULLRESYNC") {
			s.receiveRDB(ctx, reader)
		}
	case resp.Array:
		cmdName := strings.ToLower(r.Data.([]*resp.Resp)[0].String())
		args := r.Data.([]*resp.Resp)[1:]
		if cmdName == "replconf" && len(args) >= 2 && strings.ToLower(args[0].String()) == "getack" && args[1].String() == "*" {
			s.respondToGetAck(ctx)
		} else {
			cmd, err := s.cFactory.GetCommand(cmdName)
			if err != nil {
				logger.Error(ctx, "Unknown command from master: %s", cmdName)
				return
			}
			tmpWriter := resp.NewWriter(&bytes.Buffer{}, resp.RESP3)
			err = cmd.Execute(s.masterClient, tmpWriter, args)
			if err != nil {
				logger.Error(ctx, "Failed to execute command from master: %v", err)
			}
		}
	case resp.BulkString:
	case resp.SimpleError, resp.BulkError:
		errMsg := r.String()
		logger.Error(ctx, "Error from master: %s", errMsg)
	default:
		logger.Warn(ctx, "Unhandled RESP type from master: %c", r.Type)
	}
}

func (s *Server) respondToGetAck(ctx context.Context) {
	ackMessage := resp.CreateCommand("REPLCONF", "ACK", strconv.FormatUint(s.offset, 10))
	_, err := s.masterClient.Writer.Write(ackMessage)
	if err != nil {
		logger.Error(ctx, "Failed to write REPLCONF ACK to master: %v", err)
	}

	err = s.masterClient.Writer.Flush()
	if err != nil {
		logger.Error(ctx, "Failed to flush REPLCONF ACK to master: %v", err)
	}
}

func (s *Server) commandSender(ctx context.Context) {
	for {
		select {
		case <-s.done:
			return
		case req := <-s.commandChan:
			_, err := s.masterClient.Writer.Write(req.cmd)
			if err != nil {
				logger.Error(ctx, "Failed to write command to master: %v", err)
				close(req.respCh)
				continue
			}
			err = s.masterClient.Writer.Flush()
			if err != nil {
				logger.Error(ctx, "Failed to flush command to master: %v", err)
				close(req.respCh)
				continue
			}

			s.queueMu.Lock()
			s.responseWaitQueue = append(s.responseWaitQueue, &responseRequest{respCh: req.respCh})
			s.queueMu.Unlock()
		}
	}
}

func (s *Server) dispatchResponse(r *resp.Resp) {
	s.queueMu.Lock()
	if len(s.responseWaitQueue) == 0 {
		s.queueMu.Unlock()
		return
	}
	req := s.responseWaitQueue[0]
	s.responseWaitQueue = s.responseWaitQueue[1:]
	s.queueMu.Unlock()

	req.respCh <- r
	close(req.respCh)
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
		cl.ID = utils.GenerateRandomAlphanumeric(40)
		s.clients[cl] = struct{}{}
		go cl.HandleConnection(ctx)
	}
}

func (s *Server) startReplication(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			s.masterClient = nil
		}
	}()

	if err := s.sendPingToMaster(ctx); err != nil {
		return fmt.Errorf("failed to send PING to master: %v", err)
	}

	if err := s.sendReplconfToMaster(ctx); err != nil {
		return fmt.Errorf("failed to send REPLCONF to master: %v", err)
	}

	if err := s.sendPsyncToMaster(ctx); err != nil {
		return fmt.Errorf("failed to send PSYNC to master: %v", err)
	}

	return nil
}

func (s *Server) sendPingToMaster(ctx context.Context) error {
	pingCmd := resp.CreatePingCommand()
	if err := s.sendCommandAndCheckOK(ctx, pingCmd, "PING"); err != nil {
		return err
	}
	logger.Info(ctx, "Successfully sent PING to master")
	return nil
}

func (s *Server) sendReplconfToMaster(ctx context.Context) error {
	port, _ := s.cfg.Get(config.ListenAddrKey)
	port = strings.TrimPrefix(port, ":")
	replconfPort := resp.CreateReplconfCommand("listening-port", port)

	if err := s.sendCommandAndCheckOK(ctx, replconfPort, "REPLCONF listening-port"); err != nil {
		return err
	}

	replconfCapa := resp.CreateReplconfCommand("capa", "psync2")
	if err := s.sendCommandAndCheckOK(ctx, replconfCapa, "REPLCONF capa psync2"); err != nil {
		return err
	}

	logger.Info(ctx, "Successfully sent REPLCONF commands to master")
	return nil
}

func (s *Server) sendCommandAndCheckOK(ctx context.Context, cmd []byte, cmdName string) error {
	respCh := make(chan *resp.Resp, 1)
	req := commandRequest{
		cmd:    cmd,
		respCh: respCh,
	}

	select {
	case s.commandChan <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case r := <-respCh:
		if r == nil {
			return fmt.Errorf("failed to send %s command", cmdName)
		}
		if r.Type != resp.SimpleString || (r.String() != "OK" && r.String() != "PONG") {
			return fmt.Errorf("unexpected response for %s: %v", cmdName, r)
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for %s response", cmdName)
	}
}

func (s *Server) sendPsyncToMaster(ctx context.Context) error {
	psyncCmd := resp.CreatePsyncCommand("?", "-1")
	respCh := make(chan *resp.Resp, 1)
	req := commandRequest{
		cmd:    psyncCmd,
		respCh: respCh,
	}

	select {
	case s.commandChan <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case r := <-respCh:
		if r == nil {
			return fmt.Errorf("failed to send PSYNC command")
		}
		if r.Type != resp.SimpleString || !strings.HasPrefix(r.String(), "FULLRESYNC") {
			return fmt.Errorf("unexpected response for PSYNC: %v", r)
		}
		logger.Info(ctx, "Successfully sent PSYNC to master and received FULLRESYNC")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for PSYNC response")
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
	cmdName := strings.ToLower(r.Data.([]*resp.Resp)[0].String())
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
	if _, ok := config.WriteableCommands[cmdName]; ok {
		s.propagateCommand(ctx, r)
	}
	if cmdName == "replconf" && len(args) > 0 && args[0].String() == "listening-port" {
		s.addReplica(cl)
	}

	return nil
}

func (s *Server) propagateCommand(ctx context.Context, r *resp.Resp) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send the command to each replica
	for replica := range s.replicas {
		_, err := replica.Conn().Write(r.RAW())
		if err != nil {
			logger.Error(ctx, "Failed to send command to replica %s: %v", replica.ID, err)
			// Remove disconnected replica
			delete(s.replicas, replica)
			replica.Close(ctx)
		}
	}
}

func (s *Server) pingReplicas(ctx context.Context) {
	ticker := time.NewTicker(replicaPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			for replica := range s.replicas {
				go func(replica *client.Client) {
					err := s.sendPing(replica)
					if err != nil {
						logger.Error(ctx, "Failed to ping replica %s: %v", replica.ID, err)
						s.removeReplica(ctx, replica)
					}
				}(replica)
			}
			s.mu.Unlock()
		}
	}
}

func (s *Server) sendReplConfGetAck(ctx context.Context) {
	ticker := time.NewTicker(replconfGetAckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			replicas := make([]*client.Client, 0, len(s.replicas))
			for replica := range s.replicas {
				replicas = append(replicas, replica)
			}
			s.mu.Unlock()

			for _, replica := range replicas {
				go func(replica *client.Client) {
					getAckCmd := resp.CreateCommand("REPLCONF", "GETACK", "*")
					_, err := replica.Writer.Write(getAckCmd)
					if err != nil {
						logger.Error(ctx, "Failed to write REPLCONF GETACK to replica %s: %v", replica.ID, err)
						s.removeReplica(ctx, replica)
						return
					}
					err = replica.Writer.Flush()
					if err != nil {
						if errors.Is(err, net.ErrClosed) {
							logger.Info(ctx, "Failed to flush REPLCONF GETACK to replica %s, replica disconnected", replica.ID)
						} else {
							logger.Error(ctx, "Failed to flush REPLCONF GETACK to replica %s: %v", replica.ID, err)
						}
						s.removeReplica(ctx, replica)
						return
					}
				}(replica)
			}
		}
	}
}

func (s *Server) sendPing(replica *client.Client) error {
	pingCmd := resp.CreatePingCommand()

	_, err := replica.Conn().Write(pingCmd)
	if err != nil {
		return fmt.Errorf("error sending PING to replica %s: %w", replica.ID, err)
	}

	err = replica.Writer.Flush()
	if err != nil {
		return fmt.Errorf("error flushing PING to replica %s: %w", replica.ID, err)
	}

	return nil
}

func (s *Server) removeReplica(ctx context.Context, replica *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.replicas, replica)
	replica.Close(ctx)
	logger.Info(ctx, "Removed replica %s due to failed PING", replica.ID)
}

func (s *Server) addReplica(c *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.replicas[c] = struct{}{}
}

func (s *Server) GetReplicaInfo() []map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := make([]map[string]string, 0, len(s.replicas))
	for replica := range s.replicas {
		replicaInfo := map[string]string{
			"id":             replica.ID,
			"addr":           replica.Conn().RemoteAddr().String(),
			"listening_port": replica.ListeningPort,
		}
		info = append(info, replicaInfo)
	}
	return info
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

func (s *Server) GetReplicationID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.replicationID
}
