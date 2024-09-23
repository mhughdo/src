package command

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mhughdo/src/internal/client"
	"github.com/mhughdo/src/pkg/keyval"
	"github.com/mhughdo/src/pkg/resp"
)

type XRead struct {
	kv          keyval.KV
	streamOrder []string
}

type XReadOptions struct {
	Count   uint64
	Block   *time.Duration
	Streams map[string]string
}

// Edge cases
// 1. Block > 0, id "+", stream is not empty => Return immediately
// 2. Block > 0, id "$", stream is not empty => Block until new data is available
// 3. Block > 0, id "+", stream is empty => Block until new data is available
func (x *XRead) Execute(cl *client.Client, wr *resp.Writer, args []*resp.Resp) error {
	opts, err := x.parseArgs(args)
	if err != nil {
		return wr.WriteError(err)
	}

	result, err := x.readStreams(opts)
	if err != nil {
		return wr.WriteError(err)
	}

	if result == nil {
		return wr.WriteNull(resp.Array)
	}

	return x.writeResult(cl, wr, result)
}

func (x *XRead) parseArgs(args []*resp.Resp) (*XReadOptions, error) {
	opts := &XReadOptions{
		Streams: make(map[string]string),
	}

	i := 0
	for i < len(args) {
		switch strings.ToUpper(args[i].String()) {
		case "COUNT":
			i++
			if i >= len(args) {
				return nil, ErrSyntaxError
			}
			count, err := args[i].Uint64()
			if err != nil {
				return nil, errors.New("ERR value is not an integer or out of range")
			}
			opts.Count = count
		case "BLOCK":
			i++
			if i >= len(args) {
				return nil, ErrSyntaxError
			}
			block, err := args[i].Int64()
			if err != nil {
				return nil, errors.New("ERR value is not an integer or out of range")
			}
			duration := time.Duration(block) * time.Millisecond
			opts.Block = &duration
		case "STREAMS":
			i++
			streamCount := (len(args) - i) / 2
			if streamCount <= 0 || (len(args)-i)%2 != 0 {
				return nil, errors.New("ERR Unbalanced 'xread' list of streams: for each stream key an ID or '$' must be specified")
			}
			x.streamOrder = make([]string, streamCount)
			for j := 0; j < streamCount; j++ {
				key := args[i+j].String()
				id := args[i+streamCount+j].String()
				opts.Streams[key] = id
				x.streamOrder[j] = key
			}
			return opts, nil
		default:
			return nil, ErrSyntaxError
		}
		i++
	}

	return nil, ErrSyntaxError
}

func (x *XRead) readStreams(opts *XReadOptions) (map[string][]keyval.StreamEntry, error) {
	result := make(map[string][]keyval.StreamEntry)
	hasData := false

	for streamName, lastID := range opts.Streams {
		stream, err := x.kv.GetStream(streamName, false)
		if err != nil {
			return nil, err
		}
		if stream == nil {
			continue
		}

		var entries []keyval.StreamEntry
		if lastID == "$" {
			// Ignore this case as $ means only new entries
		} else if lastID == "+" {
			entries = append(entries, stream.Range(stream.LastID(), "+", 1)...)
		} else {
			entries = append(entries, stream.Range(lastID, "+", opts.Count)...)
		}
		if len(entries) > 0 {
			result[streamName] = entries
			hasData = true
		}
	}

	if hasData {
		return result, nil
	}

	if opts.Block != nil {
		return x.blockingRead(opts)
	}

	return nil, nil
}

func (x *XRead) blockingRead(opts *XReadOptions) (map[string][]keyval.StreamEntry, error) {
	result := make(map[string][]keyval.StreamEntry)
	subscriptions := make(map[string]chan keyval.StreamEntry)

	defer func() {
		for streamName, ch := range subscriptions {
			stream, _ := x.kv.GetStream(streamName, false)
			if stream != nil {
				stream.Unsubscribe(ch)
			}
		}
	}()

	for streamName := range opts.Streams {
		stream, err := x.kv.GetStream(streamName, true)
		if err != nil {
			return nil, err
		}

		ch := stream.Subscribe()
		subscriptions[streamName] = ch
	}

	var timer *time.Timer
	if opts.Block != nil && *opts.Block > 0 {
		timer = time.NewTimer(*opts.Block)
		defer timer.Stop()
	}
	cases := make([]reflect.SelectCase, 0, len(subscriptions)+1)
	for _, streamName := range x.streamOrder {
		ch := subscriptions[streamName]
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)})
	}

	if timer != nil {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(timer.C)})
	}

	chosen, value, _ := reflect.Select(cases)
	if chosen == len(cases)-1 && timer != nil {
		// Timeout
		return nil, nil
	}

	if chosen < len(subscriptions) {
		streamName := x.streamOrder[chosen]
		entry := value.Interface().(keyval.StreamEntry)
		result[streamName] = []keyval.StreamEntry{entry}
		return result, nil
	}

	return nil, nil
}

func (x *XRead) writeResult(cl *client.Client, wr *resp.Writer, result map[string][]keyval.StreamEntry) error {
	if cl.GetPreferredRespVersion() == int(resp.RESP3) {
		return x.writeResultRESP3(wr, result)
	}
	return x.writeResultRESP2(wr, result)
}

func (x *XRead) writeResultRESP2(wr *resp.Writer, result map[string][]keyval.StreamEntry) error {
	var response []any
	for _, streamName := range x.streamOrder {
		if entries, ok := result[streamName]; ok {
			streamResponse := []any{streamName, x.formatEntries(entries)}
			response = append(response, streamResponse)
		}
	}
	return wr.WriteSlice(response)
}

func (x *XRead) writeResultRESP3(wr *resp.Writer, result map[string][]keyval.StreamEntry) error {
	response := make(map[string]any)
	for _, streamName := range x.streamOrder {
		if entries, ok := result[streamName]; ok {
			response[streamName] = x.formatEntries(entries)
		}
	}
	return wr.WriteMapOrdered(response, x.streamOrder)
}

func (x *XRead) formatEntries(entries []keyval.StreamEntry) []any {
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

func (x *XRead) IsBlocking(args []*resp.Resp) bool {
	for i, arg := range args {
		if strings.ToUpper(arg.String()) == "BLOCK" {
			if i+1 < len(args) {
				if _, err := strconv.ParseInt(args[i+1].String(), 10, 64); err == nil {
					return true
				}
			}
			break
		}
	}
	return false
}
