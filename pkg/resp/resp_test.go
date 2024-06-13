package resp

import (
	"bytes"
	"testing"
)

func TestParseRESP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   []byte
		want    *Resp
		wantErr error
	}{
		{
			name:  "SimpleString",
			input: []byte("+OK\r\n"),
			want: &Resp{Type: SimpleString,
				Data: []byte("OK")},
			wantErr: nil,
		},
		{
			name:  "SimpleError",
			input: []byte("-ERR unknown command\r\n"),
			want: &Resp{Type: SimpleError,
				Data: []byte("ERR unknown command")},
			wantErr: nil,
		},
		{
			name:  "Integer",
			input: []byte(":1000\r\n"),
			want: &Resp{Type: Integer,

				Data: []byte("1000")},
			wantErr: nil,
		},
		{
			name:  "Negative Integer",
			input: []byte(":-1000\r\n"),
			want: &Resp{Type: Integer,

				Data: []byte("-1000")},
			wantErr: nil,
		},
		{
			name:  "Null",
			input: []byte("_\r\n"),
			want:  &Resp{Type: Null},
		},
		{
			name:  "Boolean",
			input: []byte("#t\r\n"),
			want: &Resp{Type: Boolean,
				Data: []byte("t")},
		},
		{
			name:  "Float",
			input: []byte(",3.14\r\n"),
			want: &Resp{Type: Double,
				Data: []byte("3.14")},
			wantErr: nil,
		},
		{
			name:  "Infinity Float",
			input: []byte(",inf\r\n"),
			want: &Resp{Type: Double,
				Data: []byte("inf")},
			wantErr: nil,
		},
		{
			name:  "NaN Float",
			input: []byte(",nan\r\n"),
			want: &Resp{Type: Double,
				Data: []byte("nan")},
			wantErr: nil,
		},
		{
			name:  "BigNumber",
			input: []byte("(3492890328409238509324850943850943825024385\r\n"),
			want: &Resp{Type: BigNumber,

				Data: []byte("3492890328409238509324850943850943825024385")},
			wantErr: nil,
		},
		{
			name:  "BulkString",
			input: []byte("$5\r\nhello\r\n"),
			want: &Resp{Type: BulkString,

				Data: []byte("hello")},
			wantErr: nil,
		},
		{
			name:  "Null BulkString",
			input: []byte("$-1\r\n"),
			want: &Resp{Type: BulkString,

				Data: []byte("")},
			wantErr: nil,
		},
		{
			name:  "BulkError",
			input: []byte("!21\r\nSYNTAX invalid syntax\r\n"),
			want: &Resp{Type: BulkError,

				Data: []byte("SYNTAX invalid syntax")},
			wantErr: nil,
		},
		{
			name:  "VerbatimString",
			input: []byte("=15\r\ntxt:Some string\r\n"),
			want: &Resp{Type: VerbatimString,

				Data: []byte("txt:Some string")},
			wantErr: nil,
		},
		{
			name:  "Nested Array",
			input: []byte("*2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Hello\r\n-World\r\n"),
			want: &Resp{Type: Array,
				Data: []*Resp{
					{Type: Array,
						Length: 3,
						Data: []*Resp{
							{Type: Integer, Data: []byte("1")},
							{Type: Integer, Data: []byte("2")},
							{Type: Integer, Data: []byte("3")},
						},
					},
					{
						Type:   Array,
						Length: 2,
						Data: []*Resp{
							{Type: SimpleString, Data: []byte("Hello")},
							{Type: SimpleError, Data: []byte("World")},
						},
					},
				},
			},
		},
		{
			name:  "Array of Mixed Types",
			input: []byte("*5\r\n:1\r\n:2\r\n:3\r\n:4\r\n$5\r\nhello\r\n"),
			want: &Resp{Type: Array,
				Length: 5,
				Data: []*Resp{
					{Type: Integer, Data: []byte("1")},
					{Type: Integer, Data: []byte("2")},
					{Type: Integer, Data: []byte("3")},
					{Type: Integer, Data: []byte("4")},
					{Type: BulkString,
						Length: 5,
						Data:   []byte("hello")},
				},
			},
		},
		{
			name:  "Array of SimpleString",
			input: []byte("*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n"),
			want: &Resp{Type: Array,
				Length: 2,
				Data: []*Resp{
					{Type: BulkString, Length: 5, Data: []byte("hello")},

					{Type: BulkString, Length: 5, Data: []byte("world")},
				},
			},
			wantErr: nil,
		},
		{
			name:  "Array of Integer",
			input: []byte("*3\r\n:1\r\n:2\r\n:3\r\n"),
			want: &Resp{Type: Array,
				Data: []*Resp{
					{Type: Integer, Data: []byte("1")},
					{Type: Integer, Data: []byte("2")},
					{Type: Integer, Data: []byte("3")},
				},
			},
			wantErr: nil,
		},
		{
			name:    "Empty Array",
			input:   []byte("*0\r\n"),
			want:    &Resp{Type: Array, Data: []*Resp{}},
			wantErr: nil,
		},
		{
			name:    "Null Array",
			input:   []byte("*-1\r\n"),
			want:    &Resp{Type: Array, Data: nil},
			wantErr: nil,
		},
		// {
		// 	name:  "Map",
		// 	input: []byte("%2\r\n+first\r\n:1\r\n+second\r\n:2\r\n"),
		// 	want: &Resp{Type: Map,
		// 		Length: 2,
		// 		Data: map[string]*Resp{
		// 			"first":  {Type: Integer, Data: []byte("1")},
		// 			"second": {Type: Integer, Data: []byte("2")},
		// 		},
		// 	},
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewResp(bytes.NewReader(tt.input))
			got, err := resp.ParseResp()
			if err != tt.wantErr {
				t.Errorf("ParseRESP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.String() != tt.want.String() {
				t.Errorf("ParseRESP() got = %v, want %v", got, tt.want)
			}
		})
	}
}
