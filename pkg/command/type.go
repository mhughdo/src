package command

import (
	"errors"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type TypeCmd struct {
	kv keyval.KV
}

func (t *TypeCmd) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'type' command"))
	}

	key := args[0].String()
	val := t.kv.Get(key)
	if val == nil {
		return wr.WriteSimpleValue(resp.SimpleString, []byte("none"))
	}

	return wr.WriteSimpleValue(resp.SimpleString, []byte("string"))
}
