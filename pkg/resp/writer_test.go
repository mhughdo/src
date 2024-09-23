package resp

import (
	"bytes"
	"encoding"
	"errors"
	"net"
	"testing"
	"time"
)

type SampleStruct struct{}

var _ encoding.BinaryMarshaler = (*SampleStruct)(nil)

func (t *SampleStruct) MarshalBinary() ([]byte, error) {
	return []byte("hello"), nil
}
func TestWriteArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  []any
		want    []byte
		wantErr error
	}{
		{
			name:    "EmptyArray",
			values:  []any{},
			want:    []byte("*0\r\n"),
			wantErr: nil,
		},
		{
			name: "Array",
			values: []any{
				"string",
				12,
				34.56,
				[]byte{'b', 'y', 't', 'e', 's'},
				true,
				nil,
				errors.New("Err invalid value"),
			},
			want:    []byte("*7\r\n$6\r\nstring\r\n:12\r\n,34.56\r\n$5\r\nbytes\r\n#t\r\n_\r\n!17\r\nErr invalid value\r\n"),
			wantErr: nil,
		},
		{
			name: "Array with negative int",
			values: []any{
				-12,
				-34.56,
			},
			want:    []byte("*2\r\n:-12\r\n,-34.56\r\n"),
			wantErr: nil,
		},
		{
			name: "Array with time",
			values: []any{
				time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			want:    []byte("*1\r\n$20\r\n2020-01-01T00:00:00Z\r\n"),
			wantErr: nil,
		},
		{
			name: "Array with time.Duration",
			values: []any{
				time.Hour,
			},
			want: []byte("*1\r\n:3600000000000\r\n"),
		},
		{
			name: "Array with net.IP",
			values: []any{
				net.ParseIP("192.168.1.1"),
			},
			want:    []byte("*1\r\n$11\r\n192.168.1.1\r\n"),
			wantErr: nil,
		},
		{
			name: "Array with marshalable struct",
			values: []any{
				&SampleStruct{},
			},
			want:    []byte("*1\r\n$5\r\nhello\r\n"),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := NewWriter(buf, RESP3)
			err := w.WriteSlice(tt.values)
			if err != tt.wantErr {
				t.Errorf("WriteArray() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(buf.Bytes(), tt.want) {
				t.Errorf("WriteArray() = %v, want %v", buf.String(), string(tt.want))
			}
		})
	}
}

func TestWriteResp2Value(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   any
		want    []byte
		wantErr error
	}{
		{
			name:    "BulkString",
			value:   "Hello World",
			want:    []byte("$11\r\nHello World\r\n"),
			wantErr: nil,
		},
		{
			name:  "EmptyBulkString",
			value: "",
			want:  []byte("$0\r\n\r\n"),
		},
		{
			name:    "SimpleErr",
			value:   errors.New("ERR unknown command"),
			want:    []byte("-ERR unknown command\r\n"),
			wantErr: nil,
		},
		{
			name:    "Integer",
			value:   1000,
			want:    []byte(":1000\r\n"),
			wantErr: nil,
		},
		{
			name:    "Float",
			value:   3.14,
			want:    []byte("$4\r\n3.14\r\n"),
			wantErr: nil,
		},
		{
			name:    "Nil",
			value:   nil,
			want:    []byte("$0\r\n\r\n"),
			wantErr: nil,
		},
		{
			name:    "Bool",
			value:   true,
			want:    []byte("$1\r\n1\r\n"),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := NewWriter(buf, RESP2)
			err := w.WriteValue(tt.value)
			if err != tt.wantErr {
				t.Errorf("WriteValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !bytes.Equal(buf.Bytes(), tt.want) {
				t.Errorf("WriteValue() = %s, want %s", buf.Bytes(), tt.want)
			}
		})
	}
}

func TestWriteResp3Value(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   any
		want    []byte
		wantErr error
	}{
		{
			name:    "BulkString",
			value:   "Hello World",
			want:    []byte("$11\r\nHello World\r\n"),
			wantErr: nil,
		},
		{
			name:  "EmptyBulkString",
			value: "",
			want:  []byte("$0\r\n\r\n"),
		},
		{
			name:    "BulkError",
			value:   []byte("ERR unknown command"),
			want:    []byte("$19\r\nERR unknown command\r\n"),
			wantErr: nil,
		},
		{
			name:    "Integer",
			value:   1000,
			want:    []byte(":1000\r\n"),
			wantErr: nil,
		},
		{
			name:  "Float",
			value: 3.14,
			want:  []byte(",3.14\r\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := NewWriter(buf, RESP3)
			err := w.WriteValue(tt.value)
			if err != tt.wantErr {
				t.Errorf("WriteValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !bytes.Equal(buf.Bytes(), tt.want) {
				t.Errorf("WriteValue() = %v, want %v", buf.Bytes(), tt.want)
			}
		})
	}
}

func TestWriteSimpleValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   []byte
		valType byte
		want    []byte
		wantErr error
	}{
		{
			name:    "SimpleString",
			value:   []byte("OK"),
			valType: '+',
			want:    []byte("+OK\r\n"),
			wantErr: nil,
		},
		{
			name:    "SimpleError",
			value:   []byte("ERR unknown command"),
			valType: '-',
			want:    []byte("-ERR unknown command\r\n"),
			wantErr: nil,
		},
		{
			name:    "Integer",
			value:   []byte("1000"),
			valType: ':',
			want:    []byte(":1000\r\n"),
			wantErr: nil,
		},
		{
			name:    "Null",
			value:   []byte(""),
			valType: '_',
			want:    []byte("_\r\n"),
			wantErr: nil,
		},
		{
			name:    "Boolean",
			value:   []byte("t"),
			valType: '#',
			want:    []byte("#t\r\n"),
			wantErr: nil,
		},
		{
			name:    "Float",
			value:   []byte("3.14"),
			valType: ',',
			want:    []byte(",3.14\r\n"),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			w := NewWriter(buf, RESP3)
			err := w.WriteSimpleValue(tt.valType, tt.value)
			if err != tt.wantErr {
				t.Errorf("WriteSimpleValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !bytes.Equal(buf.Bytes(), tt.want) {
				t.Errorf("WriteSimpleValue() = %v, want %v", buf.String(), string(tt.want))
			}
		})
	}
}
