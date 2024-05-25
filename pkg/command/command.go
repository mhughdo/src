package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

var (
	ErrCommandNotFound = errors.New("unknown command")
)

type Command interface {
	Execute(args []*resp.Resp) (*resp.Resp, error)
}

type CommandFactory struct {
	commands map[string]Command
}

type EchoCommand struct{}

func (ec *EchoCommand) Execute(args []*resp.Resp) (*resp.Resp, error) {
	if len(args) != 1 {
		return nil, errors.New("wrong number of arguments for 'echo' command")
	}
	return &resp.Resp{Type: resp.BulkString, Data: args[0].Data, Length: args[0].Length}, nil
}

type PingCommand struct{}

func (pc *PingCommand) Execute(args []*resp.Resp) (*resp.Resp, error) {
	return &resp.Resp{Type: resp.SimpleString, Data: []byte("PONG")}, nil
}

func NewCommandFactory() *CommandFactory {
	return &CommandFactory{
		commands: map[string]Command{
			"echo": &EchoCommand{},
			"ping": &PingCommand{},
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
