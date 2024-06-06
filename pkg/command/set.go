package command

import (
	"errors"
	"fmt"

	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type Set struct {
	kv keyval.KV
}

func (s *Set) Execute(wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 2 {
		return errors.New("wrong number of arguments for 'set' command")
	}

	err := s.kv.Set(args[0].String(), args[1].Bytes())
	if err != nil {
		return err
	}
	err = wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	err = wr.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush response: %w", err)
	}

	return nil
}
