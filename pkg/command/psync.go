package command

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/keyval"
	"github.com/mhughdo/src/pkg/rdb"
	"github.com/mhughdo/src/pkg/resp"
)

type Psync struct {
	serverInfo ServerInfoProvider
	kv         keyval.KV
}

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
	rdbSaver := rdb.NewRDBSaver(p.kv.Export())
	buf := &bytes.Buffer{}
	err = rdbSaver.SaveRDB(buf)
	if err != nil {
		return err
	}
	rdbData := buf.String()
	rdbRes := fmt.Sprintf("$%d\r\n%s", len(rdbData), rdbData)
	_, err = c.Conn().Write([]byte(rdbRes))
	if err != nil {
		return err
	}

	return c.Send()
}

func (p *Psync) IsBlocking(_ []*resp.Resp) bool {
	return false
}
