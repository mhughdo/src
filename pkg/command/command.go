package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/mhughdo/src/internal/app/server/config"
	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/keyval"
	"github.com/mhughdo/src/pkg/resp"
)

var (
	ErrCommandNotFound = errors.New("unknown command")
)

var TransactionCommands = map[string]struct{}{
	"multi":   {},
	"exec":    {},
	"discard": {},
}

type ServerInfoProvider interface {
	GetReplicaInfo() []map[string]string
	GetReplicationID() string
	GetReplicaAcknowledgedCount(offset uint64) int
	PropagateCommand(ctx context.Context, cl *client.Client, r *resp.Resp)
	GetCommand(cmd string) (Command, error)
}

type Command interface {
	Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error
	IsBlocking(args []*resp.Resp) bool
}

type CommandFactory struct {
	commands map[string]Command
	// serverInfo ServerInfoProvider
}

type EchoCommand struct {
}

func (ec *EchoCommand) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'echo' command"))
	}

	return wr.WriteBytes(args[0].Bytes())
}

func (ec *EchoCommand) IsBlocking(_ []*resp.Resp) bool {
	return false
}

type PingCommand struct {
}

func (pc *PingCommand) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 0 {
		return wr.WriteError(errors.New("wrong number of arguments for 'ping' command"))
	}
	return wr.WriteSimpleValue(resp.SimpleString, []byte("PONG"))
}

func (pc *PingCommand) IsBlocking(_ []*resp.Resp) bool {
	return false
}

func NewCommandFactory(kv keyval.KV, cfg *config.Config, serverInfo ServerInfoProvider) *CommandFactory {
	return &CommandFactory{
		commands: map[string]Command{
			"echo":     &EchoCommand{},
			"ping":     &PingCommand{},
			"set":      &Set{kv: kv},
			"get":      &Get{kv: kv},
			"hello":    &Hello{},
			"info":     &Info{cfg: cfg, serverInfo: serverInfo},
			"client":   &ClientCmd{},
			"config":   &ConfigCmd{cfg: cfg},
			"keys":     &Keys{kv: kv},
			"type":     &TypeCmd{kv: kv},
			"xadd":     &XAdd{kv: kv},
			"xrange":   &XRange{kv: kv},
			"xread":    &XRead{kv: kv},
			"replconf": &ReplConf{},
			"psync": &Psync{
				kv:         kv,
				serverInfo: serverInfo,
			},
			"save": &Save{kv: kv},
			"wait": &Wait{
				serverInfo: serverInfo,
			},
			"incr":    &Incr{kv: kv},
			"multi":   &Multi{},
			"discard": &Discard{},
			"exec": &Exec{
				serverInfo: serverInfo,
			},
		},
	}
}

func (cf *CommandFactory) GetCommand(cmd string) (Command, error) {
	command, found := cf.commands[cmd]
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrCommandNotFound, cmd)
	}
	return command, nil
}
