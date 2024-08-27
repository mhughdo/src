package command

import (
	"errors"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type Keys struct {
	kv keyval.KV
}

func (k *Keys) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'keys' command"))
	}
	pattern := args[0].String()
	keys := k.kv.Keys()
	if pattern == "*" {
		return wr.WriteStringSlice(k.kv.Keys())
	}
	re, err := resp.TranslatePattern(pattern)
	if err != nil {
		return wr.WriteError(err)
	}
	var matchedKeys []string
	for _, key := range keys {
		if re.MatchString(key) {
			matchedKeys = append(matchedKeys, key)
		}
	}

	return wr.WriteStringSlice(matchedKeys)
}

func (k *Keys) IsBlocking(_ []*resp.Resp) bool {
	return false
}
