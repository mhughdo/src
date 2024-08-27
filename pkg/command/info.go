package command

import (
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

const (
	SERVER = "server"
)

var (
	sections = map[string]map[string]string{
		SERVER: {
			"src_version":    "1.0.0",
			"redis_version":  "7.4-rc1",
			"redis_git_sha1": "random-sha1",
			"redis_build_id": "20240616",
			"redis_mode":     "standalone",
		},
	}
)

type Info struct {
}

func (h *Info) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	argsLen := len(args)
	if argsLen > 1 {
		return wr.WriteError(fmt.Errorf("wrong number of arguments for 'info' command"))
	}
	str := strings.Builder{}
	if argsLen == 1 {
		str.WriteString(buildSectionString(args[0].String(), sections[args[0].String()]))
		return wr.WriteString(str.String())
	}
	for sectionName, section := range sections {
		str.WriteString(buildSectionString(sectionName, section))
		str.WriteString("\r\n")
	}

	return wr.WriteValue(str.String())
}

func buildSectionString(sectionName string, section map[string]string) string {
	// # Server\r\nupstash_version:1.10.5\r\n,etc.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\r\n", sectionName))
	for key, value := range section {
		sb.WriteString(fmt.Sprintf("%s:%s\r\n", key, value))
	}
	return sb.String()
}

func (h *Info) IsBlocking(_ []*resp.Resp) bool {
	return false
}
