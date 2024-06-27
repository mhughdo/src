package command

import (
	"errors"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type Keys struct {
	kv keyval.KV
}

func (k *Keys) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'keys' command"))
	}
	if args[0].String() != "*" {
		return wr.WriteError(errors.New("only '*' is supported"))
	}

	return wr.WriteStringSlice(k.kv.Keys())
}
