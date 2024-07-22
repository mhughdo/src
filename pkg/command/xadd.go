package command

import (
	"errors"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type XAdd struct {
	kv keyval.KV
}

type XAddOptions struct {
	Nomkstream bool
	MaxLen     int
	MinID      string
	// Limit      uint64
}

var ErrWrongNumberOfArguments = errors.New("ERR wrong number of arguments for 'xadd' command")

func (x *XAdd) Execute(_ *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) <= 3 {
		return wr.WriteError(ErrWrongNumberOfArguments)
	}
	streamID := args[0].String()
	var id string
	var fields [][]string

	opts := &XAddOptions{}
	for i := 1; i < len(args); i++ {
		arg := args[i].String()
		upArg := strings.ToUpper(arg)
		switch upArg {
		case "NOMKSTREAM":
			opts.Nomkstream = true
		case "MAXLEN", "MINID":
			i++
			if i < len(args) {
				if args[i].String() == "=" {
					i++
				} else if args[i].String() == "~" {
					return wr.WriteError(errors.New("ERR syntax error, ~ is not yet supported"))
				}
			}
			if i >= len(args) {
				return wr.WriteError(ErrWrongNumberOfArguments)
			}
			if upArg == "MAXLEN" {
				threshold, err := args[i].Int64()
				if err != nil {
					return wr.WriteError(ErrWrongNumberOfArguments)
				}
				opts.MaxLen = int(threshold)
			} else {
				opts.MinID = args[i].String()
			}
			if strings.ToUpper(args[i+1].String()) == "LIMIT" {
				return wr.WriteError(errors.New("ERR syntax error, LIMIT is not yet supported"))
			}
		default:
			if id == "" {
				id = arg
				continue
			}
			if i+1 >= len(args) {
				return wr.WriteError(ErrWrongNumberOfArguments)
			}
			fields = append(fields, []string{arg, args[i+1].String()})
			i++
		}
	}
	if id == "" || len(fields) == 0 {
		return wr.WriteError(ErrWrongNumberOfArguments)
	}
	createIfNotExists := !opts.Nomkstream
	s, err := x.kv.GetStream(streamID, createIfNotExists)
	if err != nil {
		return wr.WriteError(err)
	}
	if s == nil {
		return wr.WriteNull(resp.BulkString)
	}
	entryID, err := s.AddEntry(keyval.StreamEntry{
		ID:     id,
		Fields: fields,
	})
	if err != nil {
		return wr.WriteError(err)
	}
	if opts.MaxLen > 0 {
		s.TrimBySize(opts.MaxLen)
	} else if opts.MinID != "" {
		s.TrimByMinID(opts.MinID)
	}
	return wr.WriteString(entryID)
}
