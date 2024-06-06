package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

var (
	ErrCommandNotFound = errors.New("unknown command")
)

type Command interface {
	Execute(wr *resp.Writer, args []*resp.Resp) error
}

type CommandFactory struct {
	commands map[string]Command
}

type EchoCommand struct {
}

func (ec *EchoCommand) Execute(wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return errors.New("wrong number of arguments for 'echo' command")
	}
	err := wr.WriteValue(args[0].Bytes())
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	err = wr.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush response: %w", err)
	}

	return nil
}

type PingCommand struct {
}

func (pc *PingCommand) Execute(wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 0 {
		return errors.New("wrong number of arguments for 'ping' command")
	}
	err := wr.WriteSimpleValue(resp.SimpleString, []byte("PONG"))
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	err = wr.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush response: %w", err)
	}

	return nil
}

func NewCommandFactory() *CommandFactory {
	kv := keyval.NewStore()

	return &CommandFactory{
		commands: map[string]Command{
			"echo": &EchoCommand{},
			"ping": &PingCommand{},
			"set":  &Set{kv: kv},
			"get":  &Get{kv: kv},
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
