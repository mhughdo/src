package command

import (
	"errors"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/resp"
)

const (
	ServerName    = "redis"
	ServerVersion = "6.2.6"
	ID            = 1
	Mode          = "standalone"
	Role          = "master"
)

type Hello struct {
}

// HELLO [protover [AUTH username password] [SETNAME clientname]]
func (h *Hello) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	protover := resp.DefaultVersion
	if len(args) > 0 {
		pv, err := args[0].Int64()
		if err != nil {
			return wr.WriteError(errors.New("invalid protocol version"))
		}
		if pv != int64(resp.RESP2) && pv != int64(resp.RESP3) {
			return wr.WriteError(errors.New("unsupported protocol version"))
		}

		protover = resp.RESPVersion(pv)
	}

	for i := 1; i < len(args); i++ {
		optName := args[i].String()
		switch optName {
		case "AUTH":
			if i+2 >= len(args) {
				return wr.WriteError(errors.New("AUTH requires 2 arguments"))
			}
			c.SetAuthenticated(true)
			i += 2
		case "SETNAME":
			if i+1 >= len(args) {
				return wr.WriteError(errors.New("SETNAME requires 1 argument"))
			}
			c.SetName(args[i+1].String())
		}
	}

	return wr.WriteMap(map[string]any{
		"server":  ServerName,
		"version": ServerVersion,
		"proto":   protover,
		"id":      ID,
		"mode":    Mode,
		"role":    Role,
		"modules": []string{},
	})
}

func (h *Hello) IsBlocking(_ []*resp.Resp) bool {
	return false
}
