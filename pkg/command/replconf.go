package command

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/resp"
)

type ReplConf struct{}

func (rc *ReplConf) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 1 {
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
	// if len(args) < 2 {
	// 	return wr.WriteError(errors.New("wrong number of arguments for 'replconf capa' command"))
	// }
	// We don't need to handle/save the capa arguments
	case "ack":
		if len(args) != 2 {
			return wr.WriteError(errors.New("wrong number of arguments for 'replconf ack' command"))
		}
		offsetStr := args[1].String()
		offset, err := strconv.ParseUint(offsetStr, 10, 64)
		if err != nil {
			return wr.WriteError(fmt.Errorf("invalid offset in REPLCONF ACK: %v", err))
		}
		c.UpdateOffset(offset)
		return nil
	default:
		return wr.WriteError(fmt.Errorf("unknown replconf subcommand: %s", subCommand))
	}

	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (rc *ReplConf) IsBlocking(_ []*resp.Resp) bool {
	return false
}
