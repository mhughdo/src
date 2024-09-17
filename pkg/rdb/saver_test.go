package rdb

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/codecrafters-io/redis-starter-go/pkg/keyval"
)

func TestRDBSaver(t *testing.T) {
	tests := []struct {
		name     string
		setupFn  func(store keyval.KV)
		expected map[string]keyval.Value
	}{
		{
			name: "Basic string values",
			setupFn: func(store keyval.KV) {
				store.Set("str1", []byte("hello"))
				store.Set("str2", []byte("world"))
			},
			expected: map[string]keyval.Value{
				"str1": {Type: keyval.ValueTypeString, Data: []byte("hello")},
				"str2": {Type: keyval.ValueTypeString, Data: []byte("world")},
			},
		},
		{
			name: "String values with expiry",
			setupFn: func(store keyval.KV) {
				store.Set("exp1", []byte("expiring soon"))
				store.PExpire("exp1", time.Hour)
				store.Set("exp2", []byte("not expiring"))
			},
			expected: map[string]keyval.Value{
				"exp1": {Type: keyval.ValueTypeString, Data: []byte("expiring soon"), Expiry: uint64(time.Now().Add(time.Hour).UnixMilli())},
				"exp2": {Type: keyval.ValueTypeString, Data: []byte("not expiring")},
			},
		},
		{
			name: "Empty string and numeric values",
			setupFn: func(store keyval.KV) {
				store.Set("empty", []byte(""))
				store.Set("num", []byte("12345"))
			},
			expected: map[string]keyval.Value{
				"empty": {Type: keyval.ValueTypeString, Data: []byte("")},
				"num":   {Type: keyval.ValueTypeString, Data: []byte("12345")},
			},
		},
		{
			name: "Unicode string values",
			setupFn: func(store keyval.KV) {
				store.Set("unicode", []byte("こんにちは世界"))
			},
			expected: map[string]keyval.Value{
				"unicode": {Type: keyval.ValueTypeString, Data: []byte("こんにちは世界")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			store := keyval.NewStore()
			tt.setupFn(store)

			data := store.Export()

			var buf bytes.Buffer
			saver := NewRDBSaver(data)
			err := saver.SaveRDB(&buf)
			if err != nil {
				t.Fatalf("Failed to save RDB: %v", err)
			}

			parser := NewRDBParser(bytes.NewReader(buf.Bytes()))
			err = parser.ParseRDB(ctx)
			if err != nil {
				t.Fatalf("Failed to parse RDB: %v", err)
			}
			parsedData := parser.GetData()

			// Compare parsed data with expected data
			if !reflect.DeepEqual(parsedData, tt.expected) {
				t.Errorf("Parsed data does not match expected data.\nGot: %+v\nWant: %+v", parsedData, tt.expected)
			}

			// Additional check for expiry times
			for key, expectedValue := range tt.expected {
				if expectedValue.Expiry > 0 {
					parsedExpiry := parsedData[key].Expiry
					if parsedExpiry == 0 || parsedExpiry < expectedValue.Expiry || parsedExpiry > expectedValue.Expiry+1000 {
						t.Errorf("Expiry for key %s does not match.\nGot: %d\nWant: %d (±1000ms)", key, parsedExpiry, expectedValue.Expiry)
					}
				}
			}
		})
	}
}
