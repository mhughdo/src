package command

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/rdb"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

// SaveCommand implements the SAVE command.
type Save struct {
	kv keyval.KV
}

func (c *Save) Execute(cl *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	file, err := os.Create("dump2.rdb")
	if err != nil {
		return wr.WriteError(fmt.Errorf("failed to create dump.rdb: %w", err))
	}
	defer file.Close()

	saver := rdb.NewRDBSaver(c.kv.Export())
	if err := saver.SaveRDB(file); err != nil {
		return wr.WriteError(fmt.Errorf("failed to save RDB: %w", err))
	}

	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (c *Save) IsBlocking(_ []*resp.Resp) bool {
	return true
}
