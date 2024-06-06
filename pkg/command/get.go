package command

import (
	"errors"
	"fmt"

	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type Get struct {
	kv keyval.KV
}

func (g *Get) Execute(wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return errors.New("wrong number of arguments for 'get' command")
	}

	val, err := g.kv.Get(args[0].String())
	if err != nil {
		return err
	}
	if val == nil {
		err = wr.WriteSimpleValue(resp.Null, nil)
		if err != nil {
			return err
		}
	} else {
		err = wr.WriteValue(val)
		if err != nil {
			return err
		}
	}
	err = wr.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush response: %w", err)
	}

	return nil
}
