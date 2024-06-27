package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type ConfigCmd struct {
	cfg *config.Config
}

func (c *ConfigCmd) Execute(cl *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'config' command"))
	}
	subCmd := strings.ToUpper(args[0].String())
	args = args[1:]

	switch subCmd {
	case "GET":
		return c.handleGet(cl, wr, args)
	case "SET":
		return c.handleSet(cl, wr, args)
	default:
		return wr.WriteError(fmt.Errorf("unknown subcommand '%s' for 'config' command", subCmd))
	}
}

func (c *ConfigCmd) handleSet(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 2 || len(args)%2 != 0 {
		return wr.WriteError(errors.New("wrong number of arguments for config|set command"))
	}

	for i := 0; i < len(args); i += 2 {
		key := args[i].String()
		value := args[i+1].String()
		err := c.cfg.Set(key, value)
		if err != nil {
			return wr.WriteError(err)
		}
	}
	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (c *ConfigCmd) handleGet(cl *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 1 {
		return wr.WriteError(errors.New("wrong number of arguments for config|get command"))
	}
	keys := make([]string, 0, len(args))
	for _, arg := range args {
		keys = append(keys, arg.String())
	}
	valueMap := c.cfg.GetBatch(keys)

	if cl.GetPreferredRespVersion() == int(resp.RESP3) {
		resp := make(map[string]any, len(valueMap))
		for key, value := range valueMap {
			resp[key] = value
		}
		return wr.WriteMap(resp)
	} else {
		var resp []string
		for key, value := range valueMap {
			resp = append(resp, key)
			resp = append(resp, value)
		}
		return wr.WriteStringSlice(resp)
	}
}
