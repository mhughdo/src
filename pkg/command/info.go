package command

import (
	"fmt"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/app/server/config"
	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

const (
	SERVER      = "server"
	REPLICATION = "replication"
)

type DynamicFieldHandler func(*config.Config) string

type SectionInfo struct {
	StaticFields  map[string]string
	DynamicFields map[string]DynamicFieldHandler
}

var sections = map[string]SectionInfo{
	SERVER: {
		StaticFields: map[string]string{
			"src_version":    "1.0.0",
			"redis_version":  "7.4-rc1",
			"redis_git_sha1": "random-sha1",
			"redis_build_id": "20240616",
			"redis_mode":     "standalone",
		},
	},
	REPLICATION: {
		DynamicFields: map[string]DynamicFieldHandler{
			"role": determineRole,
		},
	},
}

type Info struct {
	cfg *config.Config
}

func (h *Info) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	argsLen := len(args)
	if argsLen > 1 {
		return wr.WriteError(fmt.Errorf("wrong number of arguments for 'info' command"))
	}

	str := strings.Builder{}

	if argsLen == 1 {
		sectionName := args[0].String()
		if sectionInfo, exists := sections[sectionName]; exists {
			str.WriteString(buildSectionString(sectionName, sectionInfo, h.cfg))
		}
	} else {
		for sectionName, sectionInfo := range sections {
			str.WriteString(buildSectionString(sectionName, sectionInfo, h.cfg))
			str.WriteString("\r\n")
		}
	}

	return wr.WriteValue(str.String())
}

func buildSectionString(sectionName string, sectionInfo SectionInfo, cfg *config.Config) string {
	// # Server\r\nupstash_version:1.10.5\r\n,etc.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\r\n", sectionName))

	for key, value := range sectionInfo.StaticFields {
		sb.WriteString(fmt.Sprintf("%s:%s\r\n", key, value))
	}

	for key, handler := range sectionInfo.DynamicFields {
		value := handler(cfg)
		sb.WriteString(fmt.Sprintf("%s:%s\r\n", key, value))
	}

	return sb.String()
}

func (h *Info) IsBlocking(_ []*resp.Resp) bool {
	return false
}

func determineRole(cfg *config.Config) string {
	_, err := cfg.Get(config.ReplicaOfKey)
	if err == nil {
		return "slave"
	}
	return "master"
}
