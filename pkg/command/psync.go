package command

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/resp"
)

type Psync struct {
	serverInfo ServerInfoProvider
}

var emptyRDB, _ = hex.DecodeString("524544495330303131fa0972656469732d76657205372e322e30fa0a72656469732d62697473c040fa056374696d65c26d08bc65fa08757365642d6d656dc2b0c41000fa08616f662d62617365c000fff06e3bfec0ff5aa2")

func (p *Psync) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 2 {
		return wr.WriteError(errors.New("wrong number of arguments for 'psync' command"))
	}

	replicationID := args[0].String()
	offset := args[1].String()

	if replicationID != "?" || offset != "-1" {
		return wr.WriteError(errors.New("invalid arguments for 'psync' command"))
	}

	response := fmt.Sprintf("FULLRESYNC %s 0", p.serverInfo.GetReplicationID())
	err := wr.WriteSimpleValue(resp.SimpleString, []byte(response))
	if err != nil {
		return err
	}
	err = c.Send()
	if err != nil {
		return err
	}
	emptyRDBResponse := fmt.Sprintf("$%d\r\n%s", len(emptyRDB), emptyRDB)
	_, err = c.Conn().Write([]byte(emptyRDBResponse))
	if err != nil {
		return err
	}

	return c.Send()
}

func (p *Psync) IsBlocking(_ []*resp.Resp) bool {
	return false
}
