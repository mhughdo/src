package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

var (
	ErrCommandNotFound = errors.New("unknown command")
)

type Command interface {
	Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error
}

type CommandFactory struct {
	commands map[string]Command
}

type EchoCommand struct {
}

func (ec *EchoCommand) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'echo' command"))
	}

	return wr.WriteBytes(args[0].Bytes())
}

type PingCommand struct {
}

func (pc *PingCommand) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 0 {
		return wr.WriteError(errors.New("wrong number of arguments for 'ping' command"))
	}
	return wr.WriteSimpleValue(resp.SimpleString, []byte("PONG"))
}

func NewCommandFactory(kv keyval.KV, cfg *config.Config) *CommandFactory {
	return &CommandFactory{
		commands: map[string]Command{
			"echo":   &EchoCommand{},
			"ping":   &PingCommand{},
			"set":    &Set{kv: kv},
			"get":    &Get{kv: kv},
			"hello":  &Hello{},
			"info":   &Info{},
			"client": &ClientCmd{},
			"config": &ConfigCmd{cfg: cfg},
			"keys":   &Keys{kv: kv},
		},
	}
}

func (cf *CommandFactory) GetCommand(cmd string) (Command, error) {
	command, found := cf.commands[strings.ToLower(cmd)]
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrCommandNotFound, cmd)
	}
	return command, nil
}
