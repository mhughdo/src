package resp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/codecrafters-io/redis-starter-go/pkg/telemetry/logger"
)

// RESP Data Types
const (
	SimpleString   = '+'
	SimpleError    = '-'
	Integer        = ':'
	BulkString     = '$'
	Array          = '*'
	Null           = '_'
	Boolean        = '#'
	Double         = ','
	BigNumber      = '('
	BulkError      = '!'
	VerbatimString = '='
	Map            = '%'
	Set            = '~'
	Pushes         = '>'
)

var (
	ErrUnknownType     = errors.New("unknown type")
	ErrInvalidType     = errors.New("invalid type")
	ErrParsingErr      = errors.New("parsing error")
	ErrIncompleteInput = errors.New("incomplete input")
	ErrEmptyLine       = errors.New("empty line")
)

type Resp struct {
	Length int
	Type   byte
	rd     *bufio.Reader
	Data   any
}

func NewResp(rd io.Reader) *Resp {
	return &Resp{
		rd: bufio.NewReader(rd),
	}
}

func (r *Resp) String() string {
	return string(r.Bytes())
}

func (r *Resp) Bytes() []byte {
	ctx := context.Background()
	switch val := r.Data.(type) {
	case []byte:
		return val
	case nil:
		return []byte{}
	case []*Resp:
		if r.Length == -1 {
			return []byte{}
		}
		s, err := stringifyArray(val)
		if err != nil {
			logger.Error(ctx, "error serializing array, err: %v", err)
			return []byte{}
		}
		return s
	case map[string]*Resp:
		s, err := stringifyMap(val)
		if err != nil {
			logger.Error(ctx, "error serializing map, err: %v", err)
			return []byte{}
		}
		return s
	default:
		return []byte{}
	}
}

func (r *Resp) Int64() (int64, error) {
	switch val := r.Data.(type) {
	case []byte:
		i, err := strconv.ParseInt(string(val), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse int: %w", err)
		}
		return i, nil
	default:
		return 0, ErrInvalidType
	}
}

func (r *Resp) Uint64() (uint64, error) {
	switch val := r.Data.(type) {
	case []byte:
		i, err := strconv.ParseUint(string(val), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse uint: %w", err)
		}
		return i, nil
	default:
		return 0, ErrInvalidType
	}
}

func (r *Resp) Float64() (float64, error) {
	switch val := r.Data.(type) {
	case []byte:
		f, err := strconv.ParseFloat(string(val), 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse float: %w", err)
		}
		return f, nil
	default:
		return 0, ErrInvalidType
	}
}

func (r *Resp) Bool() (bool, error) {
	switch val := r.Data.(type) {
	case []byte:
		return string(val) == "t", nil
	default:
		return false, ErrInvalidType
	}
}

func (r *Resp) IsNull() bool {
	return r.Type == Null
}

func (r *Resp) GetCommandName() (string, error) {
	if r.Type != Array {
		return "", fmt.Errorf("invalid type: %c", r.Type)
	}

	if r.Length <= 0 {
		return "", fmt.Errorf("invalid length: %d", r.Length)
	}

	return r.Data.([]*Resp)[0].String(), nil
}

func (r *Resp) ToStringSlice() ([]string, error) {
	if r.Type != Array {
		return nil, fmt.Errorf("invalid type: %c", r.Type)
	}
	s := make([]string, r.Length)
	for i, v := range r.Data.([]*Resp) {
		s[i] = v.String()
	}
	return s, nil
}

// SimpleString: +OK\r\n
// SimpleError: -Error message\r\n
// Integer: :1000\r\n or :-1000\r\n
// Null: _\r\n
// Boolean: #t\r\n
// Double: ,3.14\r\n or ,10\r\n
// BigNumber: (3492890328409238509324850943850943825024385\r\n
// BulkString: $5\r\nhello\r\n
// BulkError: !21\r\nSYNTAX invalid syntax\r\n
// VerbatimString: =15\r\ntxt:Some string\r\n
// Array: *2\r\n$5\r\nhello\r\n$5\r\nworld\r\n
// Map: %2\r\n+first\r\n:1\r\n+second\r\n:2\r\n
// Set: ~3\r\n+first\r\n+second\r\n+third\r\n
// Pushes: >2\r\n+first\r\n+second\r\n

func (r *Resp) readLine() ([]byte, error) {
	b, err := r.rd.ReadSlice('\n')
	if err != nil {
		if err != bufio.ErrBufferFull {
			return nil, err
		}

		full := make([]byte, len(b))
		copy(full, b)

		b, err = r.rd.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		full = append(full, b...)
		b = full
	}
	if len(b) <= 2 || b[len(b)-1] != '\n' || b[len(b)-2] != '\r' {
		return nil, fmt.Errorf("invalid input: %q", b)
	}
	return b[:len(b)-2], nil
}

func (r *Resp) ParseResp() (*Resp, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, fmt.Errorf("error reading line: %w", err)
	}
	if len(line) == 0 {
		return nil, ErrEmptyLine
	}
	respType := line[0]
	resp := Resp{Type: respType}
	n := 0
	switch respType {
	case SimpleString, SimpleError, Integer, Null, Boolean, Double, BigNumber:
		resp.Data = line[1:]
	case BulkString, BulkError, VerbatimString:
		resp.Data, n, err = r.parseBulkString(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing bulk input: %w", err)
		}
		if n != -1 && len(resp.Data.([]byte)) != n {
			return nil, fmt.Errorf("%w: expected length %d, got length %d", ErrIncompleteInput, n, len(resp.Data.([]byte)))
		}
	case Array, Pushes, Set:
		resp.Data, n, err = r.parseArray(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing array input: %w", err)
		}
		if n != -1 && n != len(resp.Data.([]*Resp)) {
			return nil, fmt.Errorf("%w: expected length %d, got length %d", ErrIncompleteInput, n, len(resp.Data.([]*Resp)))
		}
	case Map:
		resp.Data, n, err = r.parseMap(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing map input: %w", err)
		}
		if n != -1 && n != len(resp.Data.(map[string]*Resp)) {
			return nil, fmt.Errorf("%w: expected length %d, got length %d", ErrIncompleteInput, n, len(resp.Data.(map[string]*Resp)))
		}
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, respType)
	}

	resp.Length = n
	return &resp, nil
}

func (r *Resp) parseBulkString(line []byte) ([]byte, int, error) {
	n, err := parseLen(line)
	if err != nil {
		return nil, 0, err
	}
	if n == -1 {
		return []byte{}, n, nil
	}
	b := make([]byte, n+2)
	_, err = io.ReadFull(r.rd, b)
	if err != nil {
		return []byte{}, n, err
	}
	return b[:n], n, nil
}

// func (r *RESP) parseVerbatimString(line []byte) ([]byte, error) {
// 	b, err := r.parseBulkString(line)
// 	if err != nil {
// 		return []byte{}, err
// 	}
// 	if len(b) < 4 || b[3] != ':' {
// 		return []byte{}, fmt.Errorf("can't parse verbatim string reply: %q", line)
// 	}
// 	return b[4:], nil
// }

func (r *Resp) parseMap(line []byte) (map[string]*Resp, int, error) {
	n, err := parseLen(line)
	if err != nil {
		return nil, 0, err
	}
	if n == -1 {
		return nil, n, nil
	}
	m := make(map[string]*Resp, n)
	for i := 0; i < n; i++ {
		key, err := r.ParseResp()
		if err != nil {
			return nil, n, err
		}
		val, err := r.ParseResp()
		if err != nil {
			return nil, n, err
		}
		m[key.String()] = val
	}
	return m, n, nil
}

func (r *Resp) parseArray(line []byte) ([]*Resp, int, error) {
	n, err := parseLen(line)
	if err != nil {
		return nil, 0, err
	}
	if n == -1 {
		return nil, n, nil
	}
	arr := make([]*Resp, n)
	for i := 0; i < n; i++ {
		resp, err := r.ParseResp()
		if err != nil {
			return nil, n, err
		}
		arr[i] = resp
	}
	return arr, n, nil
}

// func serializeResp(data any) ([]byte, error) {
// 	r, err := json.Marshal(data)
// 	if err != nil {
// 		return []byte{}, fmt.Errorf("error marshalling resp: %w", err)
// 	}

// 	return r, nil
// }

func stringifyArray(s []*Resp) ([]byte, error) {
	var b strings.Builder
	b.WriteString("[")
	for i, resp := range s {
		b.WriteString(resp.String())
		if i < len(s)-1 {
			b.WriteString(",")
		}
	}
	b.WriteString("]")
	return []byte(b.String()), nil
}

func stringifyMap(m map[string]*Resp) ([]byte, error) {
	var b strings.Builder
	b.WriteString("{")
	i := 0
	for k, v := range m {
		b.WriteString(k)
		b.WriteString(":")
		b.WriteString(v.String())
		if i < len(m)-1 {
			b.WriteString(",")
		}
		i++
	}
	b.WriteString("}")
	return []byte(b.String()), nil
}

func parseLen(line []byte) (int, error) {
	n, err := strconv.Atoi(string(line[1:]))
	if err != nil {
		return 0, err
	}
	if n < -1 {
		return 0, fmt.Errorf("error parsing length: invalid length %d", n)
	}
	return n, nil
}

func (r *Resp) ToResponse() []byte {
	switch r.Type {
	case SimpleString, SimpleError, Integer, Null, Boolean, Double, BigNumber:
		return []byte(fmt.Sprintf("%c%s\r\n", r.Type, r))
	case BulkString, VerbatimString:
		return []byte(fmt.Sprintf("%c%d\r\n%s\r\n", r.Type, r.Length, r))
	case BulkError:
		return []byte(fmt.Sprintf("%c%d\r\nERR %s\r\n", r.Type, r.Length, r))
	case Array, Pushes, Set:
		b := []byte(fmt.Sprintf("%c%d\r\n", r.Type, r.Length))
		for _, resp := range r.Data.([]*Resp) {
			b = append(b, resp.ToResponse()...)
		}
	case Map:
		b := []byte(fmt.Sprintf("%c%d\r\n", r.Type, r.Length))
		for k, v := range r.Data.(map[string]*Resp) {
			b = append(b, []byte(k)...)
			b = append(b, ':')
			b = append(b, v.ToResponse()...)
		}
	}

	return []byte{}
}
