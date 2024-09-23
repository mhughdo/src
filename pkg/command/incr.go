package command

import (
	"errors"
	"math"
	"strconv"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/keyval"
	"github.com/mhughdo/src/pkg/resp"
)

var (
	ErrNotInteger = errors.New("ERR value is not an integer or out of range")
)

type Incr struct {
	kv keyval.KV
}

func (i *Incr) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) != 1 {
		return wr.WriteError(errors.New("wrong number of arguments for 'incr' command"))
	}

	key := args[0].String()

	currentValueBytes := i.kv.Get(key)
	var currentValue int64
	var err error

	if currentValueBytes == nil {
		currentValue = 0
	} else {
		currentValue, err = strconv.ParseInt(string(currentValueBytes), 10, 64)
		if err != nil {
			return wr.WriteError(ErrNotInteger)
		}
	}

	if currentValue == math.MaxInt64 {
		return wr.WriteError(ErrNotInteger)
	}

	newValue := currentValue + 1
	err = i.kv.Set(key, []byte(strconv.FormatInt(newValue, 10)))
	if err != nil {
		return wr.WriteError(err)
	}

	return wr.WriteInteger(newValue)
}

func (i *Incr) IsBlocking(_ []*resp.Resp) bool {
	return false
}
