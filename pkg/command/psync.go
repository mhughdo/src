package command

import (
	"errors"
	"fmt"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
	"github.com/codecrafters-io/redis-starter-go/pkg/utils"
)

type Psync struct{}

func (p *Psync) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
    if len(args) != 2 {
        return wr.WriteError(errors.New("wrong number of arguments for 'psync' command"))
    }

    replicationID := args[0].String()
    offset := args[1].String()

    if replicationID != "?" || offset != "-1" {
        return wr.WriteError(errors.New("invalid arguments for 'psync' command"))
    }

    newReplicationID := utils.GenerateRandomAlphanumeric(40)
    response := fmt.Sprintf("FULLRESYNC %s 0", newReplicationID)

    return wr.WriteSimpleValue(resp.SimpleString, []byte(response))
}

func (p *Psync) IsBlocking(_ []*resp.Resp) bool {
    return false
}
