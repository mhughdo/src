package command

import (
	"errors"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/keyval"
	"github.com/mhughdo/src/pkg/resp"
)

type TypeCmd struct {
	kv keyval.KV
}

func (t *TypeCmd) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'type' command"))
	}

	key := args[0].String()

	return wr.WriteSimpleValue(resp.SimpleString, []byte(t.kv.Type(key)))
}

func (t *TypeCmd) IsBlocking(_ []*resp.Resp) bool {
	return false
}
