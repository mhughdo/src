package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
	"github.com/codecrafters-io/redis-starter-go/pkg/telemetry/logger"
)

type Message struct {
	Resp   *resp.Resp
	Client *Client
}

type Info struct {
	name       string
	libName    string
	libVersion string
}

type Client struct {
	conn                 net.Conn
	authenticated        bool
	info                 Info
	preferredRespVersion int
	bw                   *bufio.Writer
	lastInteraction      time.Time
	disconnectChan       chan *Client
	messageChan          chan<- Message
	Writer               *resp.Writer
	closed               bool
}

func NewClient(conn net.Conn, messageChan chan<- Message) *Client {
	bw := bufio.NewWriter(conn)
	return &Client{
		conn:            conn,
		lastInteraction: time.Now(),
		disconnectChan:  make(chan *Client),
		messageChan:     messageChan,
		bw:              bw,
		Writer:          resp.NewWriter(bw, resp.DefaultVersion),
	}
}

func (c *Client) SetRespVersion(version resp.RESPVersion) {
	c.Writer.SetVersion(version)
}

func (c *Client) SetLibName(name string) {
	c.info.libName = name
}

func (c *Client) GetLibName() string {
	return c.info.libName
}

func (c *Client) SetLibVersion(version string) {
	c.info.libVersion = version
}

func (c *Client) GetLibVersion() string {
	return c.info.libVersion
}

func (c *Client) SetName(name string) {
	c.info.name = name
}

func (c *Client) GetName() string {
	return c.info.name
}

func (c *Client) IsAuthenticated() bool {
	return c.authenticated
}

func (c *Client) SetAuthenticated(authenticated bool) {
	c.authenticated = authenticated
}

func (c *Client) SetPreferredRespVersion(version int) {
	c.preferredRespVersion = version
}

func (c *Client) GetPreferredRespVersion() int {
	return c.preferredRespVersion
}

func (c *Client) Send() error {
	return c.Writer.Flush()
}

func (c *Client) Conn() net.Conn {
	return c.conn
}

func (c *Client) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func (c *Client) IsClosed() bool {
	return c.closed
}

func (c *Client) Close(ctx context.Context) {
	if c.closed {
		return
	}
	err := c.conn.Close()
	if err != nil {
		logger.Error(ctx, "failed to close connection, err: %v", err)
	}
	c.closed = true
	logger.Info(ctx, "connection closed, addr: %s", c.conn.RemoteAddr())
}

func (c *Client) HandleConnection(ctx context.Context) {
	defer func() {
		c.disconnectChan <- c
	}()
	reader := bufio.NewReader(c.conn)
	buffer := &bytes.Buffer{}

	for {
		part, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				logger.Info(ctx, "connection closed, addr: %s", c.conn.RemoteAddr())
				return
			}

			logger.Error(ctx, "failed to receive input, err: %v", err)
			return
		}
		c.lastInteraction = time.Now()
		buffer.Write(part)
		respReader := resp.NewResp(bytes.NewReader(buffer.Bytes()))
		r, err := respReader.ParseResp()
		if err != nil {
			if errors.Is(err, resp.ErrEmptyLine) {
				continue
			}
			if errors.Is(err, resp.ErrIncompleteInput) || errors.Is(err, resp.ErrUnknownType) || errors.Is(err, io.EOF) {
				continue
			}
			logger.Error(ctx, "failed to parse input, err: %v", err)
			buffer.Reset()
			continue
		}
		if r.Type != resp.Array || len(r.Data.([]*resp.Resp)) < 1 {
			logger.Error(ctx, "invalid command format, expected array with at least one element")
			buffer.Reset()
			continue
		}
		c.messageChan <- Message{Resp: r, Client: c}
		buffer.Reset()
	}
}
