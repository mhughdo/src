package keyval

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestRadixTree_AddEntry(t *testing.T) {
	tests := []struct {
		name     string
		entries  []StreamEntry
		wantTree *RadixTree
	}{
		{
			name: "Add single entry",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantTree: func() *RadixTree {
				tree := NewRadixTree()
				tree.AddEntry("1234567890-0", StreamEntry{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}})
				return tree
			}(),
		},
		{
			name: "Add multiple entries with common prefix",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantTree: func() *RadixTree {
				tree := NewRadixTree()
				tree.AddEntry("1234567890-0", StreamEntry{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}})
				tree.AddEntry("1234567890-1", StreamEntry{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}})
				tree.AddEntry("1234567891-0", StreamEntry{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}})
				return tree
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()
			for _, entry := range tt.entries {
				tree.AddEntry(entry.ID, entry)
			}

			if !reflect.DeepEqual(tree, tt.wantTree) {
				t.Errorf("AddEntry() tree = %v, want %v", tree, tt.wantTree)
			}
		})
	}
}

func TestRadixTree_Range(t *testing.T) {
	tests := []struct {
		name           string
		entries        []StreamEntry
		startID        string
		endID          string
		wantEntries    []StreamEntry
		count          uint64
		inclusiveStart bool
		inclusiveEnd   bool
		wantErr        bool
	}{
		{
			name:           "Range in empty tree",
			startID:        "-",
			endID:          "+",
			wantEntries:    nil,
			count:          0,
			inclusiveStart: true,
			inclusiveEnd:   true,
			wantErr:        false,
		},
		{
			name: "Range with one entry",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567890-0",
			endID:          "1234567890-0",
			count:          0,
			inclusiveStart: true,
			inclusiveEnd:   true,
			wantEntries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Range with multiple entries",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "-",
			endID:          "+",
			count:          0,
			inclusiveStart: true,
			inclusiveEnd:   true,
			wantEntries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Range with multiple entries with specific range",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567890-1",
			endID:          "1234567891-1",
			count:          0,
			inclusiveStart: true,
			inclusiveEnd:   true,
			wantEntries: []StreamEntry{
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Range with limit",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567890-1",
			endID:          "+",
			count:          2,
			inclusiveStart: true,
			inclusiveEnd:   true,
			wantEntries: []StreamEntry{
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Exclusive Range",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567890-0",
			endID:          "+",
			count:          0,
			inclusiveStart: false,
			inclusiveEnd:   true,
			wantEntries: []StreamEntry{
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Exclusive Range on both sides",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567890-0",
			endID:          "1234567892-0",
			count:          0,
			inclusiveStart: false,
			inclusiveEnd:   false,
			wantEntries: []StreamEntry{
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Exclusive Range on both sides with limit",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567891-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567890-0",
			endID:          "1234567892-0",
			count:          1,
			inclusiveStart: false,
			inclusiveEnd:   false,
			wantEntries: []StreamEntry{
				{ID: "1234567891-0", Fields: map[string]string{"field1": "value1"}},
			},
			wantErr: false,
		},
		{
			name: "Range with no matching entries",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567890-1", Fields: map[string]string{"field1": "value1"}},
				{ID: "1234567892-0", Fields: map[string]string{"field1": "value1"}},
			},
			startID:        "1234567891-0",
			endID:          "1234567891-1",
			count:          0,
			inclusiveStart: true,
			inclusiveEnd:   true,
			wantEntries:    nil,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()
			for _, entry := range tt.entries {
				tree.AddEntry(entry.ID, entry)
			}

			gotEntries, err := tree.Range(tt.startID, tt.endID, tt.count, tt.inclusiveStart, tt.inclusiveEnd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Range() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !compareStreamEntries(gotEntries, tt.wantEntries) {
				t.Errorf("Range() gotEntries = %v, want %v", gotEntries, tt.wantEntries)
			}
		})
	}
}

func TestCommonPrefixLength(t *testing.T) {
	tests := []struct {
		a, b []byte
		want int
	}{
		{[]byte("1234567890-0"), []byte("1234567890-1234"), 11},
		{[]byte("1234567890-0"), []byte("1234567891-0"), 9},
		{[]byte("abc"), []byte("abcde"), 3},
		{[]byte("abc"), []byte("xyz"), 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.a, tt.b), func(t *testing.T) {
			if got := commonPrefixLength(tt.a, tt.b); got != tt.want {
				t.Errorf("commonPrefixLength(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func compareStreamEntries(a, b []StreamEntry) bool {
	if a == nil && b == nil {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || !reflect.DeepEqual(a[i].Fields, b[i].Fields) {
			return false
		}
	}
	return true
}
func BenchmarkRadixTree_AddEntry(b *testing.B) {
	tree := NewRadixTree()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("%d", i)
		entry := StreamEntry{ID: id, Fields: map[string]string{"field": "value"}}
		tree.AddEntry(id, entry)
	}
}

func BenchmarkRadixTree_Range(b *testing.B) {
	tree := NewRadixTree()
	for i := 0; i < 100000; i++ {
		id := fmt.Sprintf("%d", i)
		entry := StreamEntry{ID: id, Fields: map[string]string{"field": "value"}}
		tree.AddEntry(id, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := rand.Intn(100000)
		end := start + rand.Intn(1000)
		startID := fmt.Sprintf("%d", start)
		endID := fmt.Sprintf("%d", end)
		_, err := tree.Range(startID, endID, 0, true, true)
		if err != nil {
			b.Errorf("Range() error = %v", err)
		}
	}
}
