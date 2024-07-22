package rdb

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
)

func TestRDBParser(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		want     map[string]keyval.Value
		wantErr  error
	}{
		{
			name:     "Normal",
			filepath: "../../testdata/dump.rdb",
			wantErr:  nil,
			want: map[string]keyval.Value{
				"double": {
					Type: keyval.ValueTypeString,
					Data: []byte("3.1456789"),
				},
				"str": {
					Type: keyval.ValueTypeString,
					Data: []byte("thisisastring"),
				},
				"emptystr": {
					Type: keyval.ValueTypeString,
					Data: []byte(""),
				},
				"int64": {
					Type: keyval.ValueTypeString,
					Data: []byte("-9223372036854775808"),
				},
				"int": {
					Type: keyval.ValueTypeString,
					Data: []byte("1000"),
				},
				"boolean": {
					Type: keyval.ValueTypeString,
					Data: []byte("true"),
				},
				"bignumber": {
					Type: keyval.ValueTypeString,
					Data: []byte("734589327589437589324758943278954389"),
				},
				"strwithexpire": {
					Type:   keyval.ValueTypeString,
					Data:   []byte("not_expired"),
					Expiry: 11719245191989,
				},
				"strwithPX": {
					Type:   keyval.ValueTypeString,
					Data:   []byte("not_expired_with_PX"),
					Expiry: 101719245280555,
				},
				"uint64": {
					Type: keyval.ValueTypeString,
					Data: []byte("18446744073709551615"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			file, err := os.Open(tt.filepath)
			if err != nil {
				t.Errorf("failed to open file, err: %v", err)
			}
			defer file.Close()

			rdb := NewRDBParser(file)
			err = rdb.ParseRDB(ctx)
			if err != tt.wantErr {
				t.Errorf("RDBParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotData := rdb.GetData()
			if !reflect.DeepEqual(gotData, tt.want) {
				t.Errorf("RDBParser.Parse() got = %v, want %v", gotData, tt.want)
			}
		})
	}
}
