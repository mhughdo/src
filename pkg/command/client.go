package command

import (
	"errors"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

const (
	libVersion = "LIB-VER"
	libName    = "LIB-NAME"
)

type ClientCmd struct{}

func (c *ClientCmd) Execute(cl *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'client' command"))
	}

	subCmd := strings.ToUpper(args[0].String())
	switch subCmd {
	case "SETNAME":
		if len(args) != 2 {
			return wr.WriteError(errors.New("wrong number of arguments for 'CLIENT SETNAME' command"))
		}
		cl.SetName(args[1].String())
	case "SETINFO":
		if len(args) != 3 {
			return wr.WriteError(errors.New("wrong number of arguments for 'CLIENT SETINFO' command"))
		}
		infoName := strings.ToUpper(args[1].String())
		infoValue := strings.ToUpper(args[2].String())
		if infoName != libVersion && infoName != libName {
			return wr.WriteError(errors.New("wrong argument for 'CLIENT SETINFO' command"))
		}
		if infoName == libVersion {
			cl.SetLibVersion(infoValue)
		} else if infoName == libName {
			cl.SetLibName(infoValue)
		}
	}

	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}
