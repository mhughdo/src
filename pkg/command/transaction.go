package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mhughdo/src/internal/app/server/config"
	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/resp"
)

type Multi struct{}

func (mc *Multi) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 0 {
		return wr.WriteError(errors.New("wrong number of arguments for 'multi' command"))
	}
	if c.IsInTransaction() {
		return wr.WriteError(errors.New("ERR MULTI calls can not be nested"))
	}
	c.StartTransaction()
	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (mc *Multi) IsBlocking(_ []*resp.Resp) bool {
	return false
}

type Exec struct {
	serverInfo ServerInfoProvider
}

func (ec *Exec) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 0 {
		return wr.WriteError(errors.New("wrong number of arguments for 'exec' command"))
	}
	if !c.IsInTransaction() {
		return wr.WriteError(errors.New("ERR EXEC without MULTI"))
	}

	queue := c.GetTransactionQueue()
	responses := make([]string, 0, len(queue))

	for _, cmdResp := range queue {
		cmdName := strings.ToLower(cmdResp.Data.([]*resp.Resp)[0].String())
		args := cmdResp.Data.([]*resp.Resp)[1:]

		cmd, err := ec.serverInfo.GetCommand(cmdName)
		if err != nil {
			responses = append(responses, fmt.Sprintf("-%s", err.Error()))
			continue
		}

		var buf bytes.Buffer
		tmpWriter := resp.NewWriter(&buf, resp.RESPVersion(c.GetPreferredRespVersion()))

		err = cmd.Execute(c, tmpWriter, args)
		if err != nil {
			responses = append(responses, fmt.Sprintf("-%s", err.Error()))
		} else {
			responses = append(responses, buf.String())
		}

		if _, ok := config.WriteableCommands[cmdName]; ok {
			ec.serverInfo.PropagateCommand(context.Background(), c, cmdResp)
		}
	}

	respRawRes := fmt.Sprintf("*%d\r\n", len(responses))
	for _, resp := range responses {
		respRawRes += resp
	}
	_, err := wr.Write([]byte(respRawRes))
	if err != nil {
		return err
	}

	c.ClearTransaction()
	return nil
}

func (ec *Exec) IsBlocking(_ []*resp.Resp) bool {
	return false
}

// Discard implements the DISCARD command.
type Discard struct{}

func (dc *Discard) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 0 {
		return wr.WriteError(errors.New("wrong number of arguments for 'discard' command"))
	}
	if !c.IsInTransaction() {
		return wr.WriteError(errors.New("ERR DISCARD without MULTI"))
	}
	c.ClearTransaction()
	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (dc *Discard) IsBlocking(_ []*resp.Resp) bool {
	return false
}
