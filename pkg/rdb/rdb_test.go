package rdb

import (
	"context"
	"os"
	"reflect"
	"testing"
)

func TestRDBParser(t *testing.T) {
	tests := []struct {
		name       string
		filepath   string
		want       map[string]string
		wantExpiry map[string]uint64
		wantErr    error
	}{
		{
			name:     "Normal",
			filepath: "../../testdata/dump.rdb",
			wantErr:  nil,
			want: map[string]string{
				"double":        "3.1456789",
				"str":           "thisisastring",
				"emptystr":      "",
				"int64":         "-9223372036854775808",
				"int":           "1000",
				"boolean":       "true",
				"bignumber":     "734589327589437589324758943278954389",
				"strwithexpire": "not_expired",
				"strwithPX":     "not_expired_with_PX",
				"uint64":        "18446744073709551615",
			},
			wantExpiry: map[string]uint64{
				"strwithexpire": 11719245191989,
				"strwithPX":     101719245280555,
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
			gotExpiry := rdb.GetExpiry()
			if !reflect.DeepEqual(gotData, tt.want) {
				t.Errorf("RDBParser.Parse() got = %v, want %v", gotData, tt.want)
			}
			if !reflect.DeepEqual(gotExpiry, tt.wantExpiry) {
				t.Errorf("RDBParser.Parse() gotExpiry = %v, wantExpiry %v", gotExpiry, tt.wantExpiry)
			}
		})
	}
}
