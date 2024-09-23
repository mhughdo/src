package command

import (
	"fmt"
	"strings"

	"github.com/mhughdo/src/internal/app/server/config"
	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/resp"
)

const (
	SERVER      = "server"
	REPLICATION = "replication"
)

type DynamicFieldHandler func(*config.Config, ServerInfoProvider) string

type SectionInfo struct {
	StaticFields  map[string]string
	DynamicFields map[string]DynamicFieldHandler
	CustomBuilder func(*config.Config, ServerInfoProvider) string
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
			"role":               determineRole,
			"master_replid":      getMasterReplID,
			"master_repl_offset": getMasterReplOffset,
			"connected_slaves": func(cfg *config.Config, serverInfo ServerInfoProvider) string {
				return fmt.Sprintf("%d", len(serverInfo.GetReplicaInfo()))
			},
		},
		CustomBuilder: buildReplicaInfo,
	},
}

type Info struct {
	cfg        *config.Config
	serverInfo ServerInfoProvider
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
			str.WriteString(h.buildSectionString(sectionName, sectionInfo, h.cfg))
		}
	} else {
		for sectionName, sectionInfo := range sections {
			str.WriteString(h.buildSectionString(sectionName, sectionInfo, h.cfg))
			str.WriteString("\r\n")
		}
	}

	return wr.WriteValue(str.String())
}

func (h *Info) buildSectionString(sectionName string, sectionInfo SectionInfo, cfg *config.Config) string {
	// # Server\r\nupstash_version:1.10.5\r\n,etc.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\r\n", sectionName))

	for key, value := range sectionInfo.StaticFields {
		sb.WriteString(fmt.Sprintf("%s:%s\r\n", key, value))
	}

	for key, handler := range sectionInfo.DynamicFields {
		value := handler(cfg, h.serverInfo)
		sb.WriteString(fmt.Sprintf("%s:%s\r\n", key, value))
	}

	if sectionInfo.CustomBuilder != nil {
		sb.WriteString(sectionInfo.CustomBuilder(cfg, h.serverInfo))
	}

	return sb.String()
}

func (h *Info) IsBlocking(_ []*resp.Resp) bool {
	return false
}

func determineRole(cfg *config.Config, _ ServerInfoProvider) string {
	_, err := cfg.Get(config.ReplicaOfKey)
	if err == nil {
		return "slave"
	}
	return "master"
}

func getMasterReplOffset(_ *config.Config, _ ServerInfoProvider) string {
	return "0"
}

func buildReplicaInfo(cfg *config.Config, serverInfo ServerInfoProvider) string {
	var sb strings.Builder
	replicaInfo := serverInfo.GetReplicaInfo()

	for i, replica := range replicaInfo {
		sb.WriteString(fmt.Sprintf("slave%d:id=%s,ip=%s,port=%s,state=online,offset=0,lag=0\r\n",
			i, replica["id"], replica["addr"], replica["listening_port"]))
	}
	return sb.String()
}

func getMasterReplID(_ *config.Config, serverInfo ServerInfoProvider) string {
	return serverInfo.GetReplicationID()
}
