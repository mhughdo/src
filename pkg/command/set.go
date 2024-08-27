package command

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codecrafters-io/redis-starter-go/internal/client"
	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
	"github.com/codecrafters-io/redis-starter-go/pkg/resp"
)

type Set struct {
	kv keyval.KV
}

type SetOptions struct {
	XX      bool
	NX      bool
	GET     bool
	EX      time.Duration
	PX      time.Duration
	EXAT    time.Time
	PXAT    time.Time
	KEEPTTL bool
}

// https://redis.io/docs/latest/commands/set/
// SET key value [NX | XX] [GET] [EX seconds | PX milliseconds | EXAT unix-time-seconds | PXAT unix-time-milliseconds | KEEPTTL]
// EX seconds -- Set the specified expire time, in seconds (a positive integer).
// PX milliseconds -- Set the specified expire time, in milliseconds (a positive integer).
// EXAT timestamp-seconds -- Set the specified Unix time at which the key will expire, in seconds (a positive integer).
// PXAT timestamp-milliseconds -- Set the specified Unix time at which the key will expire, in milliseconds (a positive integer).
// NX -- Only set the key if it does not already exist.
// XX -- Only set the key if it already exists.
// KEEPTTL -- Retain the time to live associated with the key.
// GET -- Return the old string stored at key, or nil if key did not exist. An error is returned and SET aborted if the value stored at key is not a string.

func (s *Set) Execute(c *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	if len(args) < 2 {
		return wr.WriteError(errors.New("wrong number of arguments for 'set' command"))
	}
	key := args[0].String()
	value := args[1].Bytes()
	opts, err := s.parseOptions(args[2:])
	if err != nil {
		return err
	}

	if opts.NX && opts.XX {
		return wr.WriteError(errors.New("both NX and XX options are not allowed"))
	}

	if opts.KEEPTTL && (opts.EX > 0 || opts.PX > 0 || !opts.EXAT.IsZero() || !opts.PXAT.IsZero()) {
		return wr.WriteError(errors.New("KEEPTTL option is not allowed with EX, PX, EXAT, or PXAT options"))
	}

	var oldVal []byte
	if opts.GET || opts.NX || opts.XX {
		oldVal = s.kv.Get(key)
	}
	keyExists := oldVal != nil

	if opts.NX {
		if opts.GET && keyExists {
			return wr.WriteBytes(oldVal)
		}
		if keyExists {
			return wr.WriteNull(resp.BulkString)
		}
	}

	if opts.XX {
		if !keyExists {
			return wr.WriteNull(resp.BulkString)
		}
	}

	err = s.kv.Set(key, value)
	if err != nil {
		return err
	}
	if !opts.KEEPTTL {
		s.kv.DeleteTTL(key)
	}

	if opts.EX > 0 {
		s.kv.Expire(key, opts.EX)
	} else if opts.PX > 0 {
		s.kv.PExpire(key, opts.PX)
	} else if !opts.EXAT.IsZero() {
		s.kv.ExpireAt(key, opts.EXAT)
	} else if !opts.PXAT.IsZero() {
		s.kv.PExpireAt(key, opts.PXAT)
	}

	if opts.GET {
		if keyExists {
			return wr.WriteBytes(oldVal)
		} else {
			return wr.WriteNull(resp.BulkString)
		}
	}

	return wr.WriteSimpleValue(resp.SimpleString, []byte("OK"))
}

func (s *Set) parseOptions(args []*resp.Resp) (*SetOptions, error) {
	opts := &SetOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i].String()
		switch strings.ToUpper(arg) {
		case "NX":
			opts.NX = true
		case "XX":
			opts.XX = true
		case "GET":
			opts.GET = true
		case "KEEPTTL":
			opts.KEEPTTL = true
		case "EX":
			if i+1 >= len(args) {
				return nil, errors.New("missing value for EX option")
			}
			i++
			seconds, err := args[i].Int64()
			if err != nil {
				return nil, fmt.Errorf("invalid value for EX option: %w", err)
			}
			opts.EX = time.Duration(seconds) * time.Second
		case "PX":
			if i+1 >= len(args) {
				return nil, errors.New("missing value for PX option")
			}
			i++
			milliseconds, err := args[i].Int64()
			if err != nil {
				return nil, fmt.Errorf("invalid value for PX option: %w", err)
			}
			opts.PX = time.Duration(milliseconds) * time.Millisecond
		case "EXAT":
			if i+1 >= len(args) {
				return nil, errors.New("missing value for EXAT option")
			}
			i++
			seconds, err := args[i].Int64()
			if err != nil {
				return nil, fmt.Errorf("invalid value for EXAT option: %w", err)
			}
			opts.EXAT = time.Unix(seconds, 0)
		case "PXAT":
			if i+1 >= len(args) {
				return nil, errors.New("missing value for PXAT option")
			}
			i++
			milliseconds, err := args[i].Int64()
			if err != nil {
				return nil, fmt.Errorf("invalid value for PXAT option: %w", err)
			}
			opts.PXAT = time.UnixMilli(milliseconds)
		default:
			return nil, fmt.Errorf("unknown option: %s", arg)
		}
	}

	return opts, nil
}

func (s *Set) IsBlocking(_ []*resp.Resp) bool {
	return false
}
