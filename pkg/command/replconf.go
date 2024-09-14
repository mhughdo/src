package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type ReplConf struct{}

func (rc *ReplConf) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 2 {
		return wr.WriteError(errors.New("wrong number of arguments for 'replconf' command"))
	}

	subCommand := strings.ToLower(args[0].String())
	switch subCommand {
	case "listening-port":
		if len(args) != 2 {
			return wr.WriteError(errors.New("wrong number of arguments for 'replconf listening-port' command"))
		}
		port := args[1].String()
		c.ListeningPort = port
	case "capa":
		if len(args) != 2 {
			return wr.WriteError(errors.New("wrong number of arguments for 'replconf capa' command"))
		}
		// capability := args[1].String()
		// Ignore capability for now
	default:
		return wr.WriteError(fmt.Errorf("unknown replconf subcommand: %s", subCommand))
	}

	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (rc *ReplConf) IsBlocking(_ []*resp.Resp) bool {
	return false
}
