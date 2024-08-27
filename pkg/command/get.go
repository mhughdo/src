package command

import (
	"errors"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type Get struct {
	kv keyval.KV
}

func (g *Get) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return errors.New("wrong number of arguments for 'get' command")
	}

	val := g.kv.Get(args[0].String())
	if val == nil {
		return wr.WriteNull(resp.BulkString)
	}

	return wr.WriteBytes(val)
}

func (g *Get) IsBlocking(_ []*resp.Resp) bool {
	return false
}
