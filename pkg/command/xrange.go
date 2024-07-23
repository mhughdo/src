package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type XRange struct {
	kv keyval.KV
}

var (
	ErrWrongArgCount = errors.New("wrong number of arguments for 'xrange' command")
)

func (x *XRange) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if err := x.validateArgs(args); err != nil {
		return wr.WriteError(err)
	}

	streamID := args[0].String()
	start, end, count, err := x.parseArgs(args)
	if err != nil {
		return wr.WriteError(err)
	}

	stream, err := x.kv.GetStream(streamID, false)
	if err != nil {
		return wr.WriteError(err)
	}
	if stream == nil {
		return wr.WriteStringSlice([]string{})
	}

	entries := stream.Range(start, end, count)
	result := x.formatEntries(entries)

	return wr.WriteSlice(result)
}

func (x *XRange) validateArgs(args []*resp.Resp) error {
	if len(args) < 3 {
		return ErrWrongArgCount
	}
	if len(args) > 5 || len(args) == 4 {
		return ErrSyntaxError
	}
	return nil
}

func (x *XRange) parseArgs(args []*resp.Resp) (start, end string, count uint64, err error) {
	start, err = parseStreamID(args[1].String(), true)
	if err != nil {
		return
	}
	end, err = parseStreamID(args[2].String(), false)
	if err != nil {
		return
	}

	if len(args) == 5 {
		if strings.ToUpper(args[3].String()) != "COUNT" {
			err = ErrSyntaxError
			return
		}
		count, err = args[4].Uint64()
		if err != nil {
			return
		}
		if count < 0 {
			count = 0
		}
	}
	return
}

func (x *XRange) formatEntries(entries []keyval.StreamEntry) []any {
	var result []any
	for _, entry := range entries {
		entryResult := []any{entry.ID}
		var entryFields []any
		for _, field := range entry.Fields {
			entryFields = append(entryFields, field[0], field[1])
		}
		entryResult = append(entryResult, entryFields)
		result = append(result, entryResult)
	}
	return result
}

func parseStreamID(streamID string, start bool) (string, error) {
	if streamID == "-" || streamID == "+" {
		return streamID, nil
	}

	parts := strings.Split(streamID, "-")
	if len(parts) > 2 {
		return "", ErrSyntaxError
	}

	if len(parts) == 2 {
		if _, err := strconv.ParseUint(parts[1], 10, 64); err != nil {
			return "", ErrInvalidStreamID
		}
		return streamID, nil
	}

	if start {
		return fmt.Sprintf("%s-0", parts[0]), nil
	}
	return fmt.Sprintf("%s-%d", parts[0], uint64(math.MaxUint64)), nil
}
