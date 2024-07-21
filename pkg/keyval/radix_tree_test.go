package keyval

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
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
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantTree: func() *RadixTree {
				tree := NewRadixTree()
				tree.AddEntry("1234567890-0", [][]string{{"field1", "value1"}})
				return tree
			}(),
		},
		{
			name: "Add multiple entries with common prefix",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantTree: func() *RadixTree {
				tree := NewRadixTree()
				tree.AddEntry("1234567890-0", [][]string{{"field1", "value1"}})
				tree.AddEntry("1234567890-1", [][]string{{"field1", "value1"}})
				tree.AddEntry("1234567891-0", [][]string{{"field1", "value1"}})
				return tree
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()
			for _, entry := range tt.entries {
				tree.AddEntry(entry.ID, entry.Fields)
			}

			if !reflect.DeepEqual(tree, tt.wantTree) {
				t.Errorf("AddEntry() tree = %v, want %v", tree, tt.wantTree)
			}
		})
	}

}

func TestRadixTree_Range(t *testing.T) {
	tests := []struct {
		name        string
		entries     []StreamEntry
		startID     string
		endID       string
		wantEntries []StreamEntry
		count       uint64
		wantErr     bool
	}{
		{
			name:        "Range in empty tree",
			startID:     "-",
			endID:       "+",
			wantEntries: nil,
			count:       0,
			wantErr:     false,
		},
		{
			name: "Range with one entry",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "1234567890-0",
			endID:   "1234567890-0",
			count:   0,
			wantEntries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Range with multiple entries",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "-",
			endID:   "+",
			count:   0,
			wantEntries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Range with multiple small entries",
			entries: []StreamEntry{
				{ID: "0-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "2-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "3-1000", Fields: [][]string{{"field1", "value1"}}},
				{ID: "3-10", Fields: [][]string{{"field1", "value1"}}},
				{ID: "30-100", Fields: [][]string{{"field1", "value1"}}},
				{ID: "300-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "3000-20", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "3-0",
			endID:   "+",
			count:   0,
			wantEntries: []StreamEntry{
				{ID: "3-10", Fields: [][]string{{"field1", "value1"}}},
				{ID: "3-1000", Fields: [][]string{{"field1", "value1"}}},
				{ID: "30-100", Fields: [][]string{{"field1", "value1"}}},
				{ID: "300-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "3000-20", Fields: [][]string{{"field1", "value1"}}},
			},
		},
		{
			name: "Range with multiple entries with specific range",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "1234567890-1",
			endID:   "1234567891-1",
			count:   0,
			wantEntries: []StreamEntry{
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Range with limit",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "1234567890-1",
			endID:   "+",
			count:   2,
			wantEntries: []StreamEntry{
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Exclusive Range",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "(1234567890-0",
			endID:   "+",
			count:   0,
			wantEntries: []StreamEntry{
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Exclusive Range on both sides",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "(1234567890-0",
			endID:   "(1234567892-0",
			count:   0,
			wantEntries: []StreamEntry{
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Exclusive Range on both sides with limit",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567891-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID: "(1234567890-0",
			endID:   "(1234567892-0",
			count:   1,
			wantEntries: []StreamEntry{
				{ID: "1234567891-0", Fields: [][]string{{"field1", "value1"}}},
			},
			wantErr: false,
		},
		{
			name: "Range with no matching entries",
			entries: []StreamEntry{
				{ID: "1234567890-0", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567890-1", Fields: [][]string{{"field1", "value1"}}},
				{ID: "1234567892-0", Fields: [][]string{{"field1", "value1"}}},
			},
			startID:     "1234567891-0",
			endID:       "1234567891-1",
			count:       0,
			wantEntries: nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()
			for _, entry := range tt.entries {
				tree.AddEntry(entry.ID, entry.Fields)
			}

			gotEntries, err := tree.Range(tt.startID, tt.endID, tt.count)
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
		fields := [][]string{{"field", "value"}}
		tree.AddEntry(id, fields)
	}
}

func BenchmarkRadixTree_Range(b *testing.B) {
	tree := NewRadixTree()
	for i := 0; i < 100000; i++ {
		id := fmt.Sprintf("%d", i)
		fields := [][]string{{"field", fmt.Sprintf("value-%d", i)}}
		tree.AddEntry(id, fields)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := rand.Intn(100000)
		end := start + rand.Intn(10000)
		startID := fmt.Sprintf("%d", start)
		endID := fmt.Sprintf("%d", end)
		_, err := tree.Range(startID, endID, 0)
		if err != nil {
			b.Errorf("Range() error = %v", err)
		}
	}
}

func TestGenerateID(t *testing.T) {
	tree := NewRadixTree()

	tests := []struct {
		name     string
		sleepMs  int
		wantDiff bool
	}{
		{"Immediate sequential calls", 0, false},
		{"Calls with 1ms delay", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id1 := tree.GenerateID()
			if tt.sleepMs > 0 {
				time.Sleep(time.Duration(tt.sleepMs) * time.Millisecond)
			}
			id2 := tree.GenerateID()

			if tt.wantDiff {
				if id1 == id2 {
					t.Errorf("Expected different IDs, got the same: %s", id1)
				}
			} else {
				parts1 := strings.Split(id1, "-")
				parts2 := strings.Split(id2, "-")
				if parts1[0] != parts2[0] {
					t.Errorf("Expected same timestamp, got %s and %s", parts1[0], parts2[0])
				}
				if parts2[1] != incrementSequence(parts1[1]) {
					t.Errorf("Expected incremented sequence, got %s after %s", parts2[1], parts1[1])
				}
			}
		})
	}
}

func setupRadixTree() *RadixTree {
	tree := NewRadixTree()
	entries := []StreamEntry{
		{ID: "1000-0", Fields: [][]string{{"key1", "value1"}}},
		{ID: "1001-0", Fields: [][]string{{"key2", "value2"}}},
		{ID: "1001-1", Fields: [][]string{{"key3", "value3"}}},
	}

	for _, e := range entries {
		tree.AddEntry(e.ID, e.Fields)
	}
	return tree
}

func TestGenerateIncompleteID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"Valid incomplete ID", "1002-*", "1002-0", false},
		{"Valid incomplete ID same as last timestamp", "1001-*", "1001-2", false},
		{"Invalid format", "1234567890*", "", true},
		{"Non-numeric timestamp", "abcdefg-*", "", true},
		{"Empty string", "", "", true},
		{"Only asterisk", "*", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := setupRadixTree()
			got, err := tree.GenerateIncompleteID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateIncompleteID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.HasPrefix(got, tt.want) {
				t.Errorf("GenerateIncompleteID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"Valid greater ID", "1001-2", true},
		{"Valid greater timestamp", "1002-0", true},
		{"Equal to last ID", "1001-1", false},
		{"Lesser ID", "1000-0", false},
		{"Lesser ID 2", "1-0", false},
		{"Invalid format", "1000", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := setupRadixTree()
			if got := tree.ValidateID(tt.id); got != tt.want {
				t.Errorf("ValidateID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func incrementSequence(seq string) string {
	i, _ := strconv.ParseInt(seq, 10, 64)
	return strconv.FormatInt(i+1, 10)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func formatInt64(i int64) string {
	return strconv.FormatInt(i, 10)
}

func generateLargeDataset(size int) []StreamEntry {
	entries := make([]StreamEntry, size)
	baseTimestamp := time.Now().UnixMilli()

	for i := 0; i < size; i++ {
		timestamp := baseTimestamp + int64(i)
		sequence := rand.Intn(1000)
		id := fmt.Sprintf("%d-%d", timestamp, sequence)
		entries[i] = StreamEntry{
			ID: id,
			Fields: [][]string{
				{"key", fmt.Sprintf("value-%d", i)},
			},
		}
	}

	return entries
}

func TestTrimBySize(t *testing.T) {
	tests := []struct {
		name         string
		datasetSize  int
		maxSize      int
		expectedSize int
	}{
		{"Small trim", 1000, 900, 900},
		{"Large trim", 10000, 5000, 5000},
		{"No trim needed", 1000, 1500, 1000},
		{"Trim to zero", 1000, 0, 0},
		{"Trim to one", 1000, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()
			dataset := generateLargeDataset(tt.datasetSize)

			for _, entry := range dataset {
				tree.AddEntry(entry.ID, entry.Fields)
			}

			trimmed := tree.TrimBySize(tt.maxSize)

			if tree.size != tt.expectedSize {
				t.Errorf("After TrimBySize(%d), tree size = %d, want %d", tt.maxSize, tree.size, tt.expectedSize)
			}

			if trimmed != tt.datasetSize-tt.expectedSize {
				t.Errorf("TrimBySize(%d) returned %d, want %d", tt.maxSize, trimmed, tt.datasetSize-tt.expectedSize)
			}

			entries, _ := tree.Range("-", "+", uint64(tree.size))
			if len(entries) != tt.expectedSize {
				t.Errorf("Range returned %d entries, want %d", len(entries), tt.expectedSize)
			}

			for i, entry := range entries {
				expectedID := dataset[tt.datasetSize-tt.expectedSize+i].ID
				if entry.ID != expectedID {
					t.Errorf("Entry %d has ID %s, want %s", i, entry.ID, expectedID)
				}
			}
		})
	}
}

func TestTrimByMinID(t *testing.T) {
	tests := []struct {
		name        string
		datasetSize int
		trimIndex   int
	}{
		{"Trim half", 1000, 500},
		{"Trim quarter", 10000, 7500},
		{"Trim all but one", 1000, 999},
		{"Trim none", 1000, 0},
		{"Trim all", 1000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()
			dataset := generateLargeDataset(tt.datasetSize)

			for _, entry := range dataset {
				tree.AddEntry(entry.ID, entry.Fields)
			}

			var minID string
			var expectedTrimmed int

			if tt.trimIndex >= tt.datasetSize {
				// If trimIndex is equal to or greater than datasetSize,
				// use a minID that is guaranteed to be greater than all entries
				minID = dataset[tt.datasetSize-1].ID + "1"
				expectedTrimmed = tt.datasetSize
			} else {
				minID = dataset[tt.trimIndex].ID
				expectedTrimmed = tt.trimIndex
			}

			trimmed := tree.TrimByMinID(minID)

			expectedSize := tt.datasetSize - expectedTrimmed
			if tree.size != expectedSize {
				t.Errorf("After TrimByMinID(%s), tree size = %d, want %d", minID, tree.size, expectedSize)
			}

			if trimmed != expectedTrimmed {
				t.Errorf("TrimByMinID(%s) returned %d, want %d", minID, trimmed, expectedTrimmed)
			}

			entries, _ := tree.Range("-", "+", uint64(tree.size))
			if len(entries) != expectedSize {
				t.Errorf("Range returned %d entries, want %d", len(entries), expectedSize)
			}

			for i, entry := range entries {
				if tt.trimIndex < tt.datasetSize {
					expectedID := dataset[tt.trimIndex+i].ID
					if entry.ID != expectedID {
						t.Errorf("Entry %d has ID %s, want %s", i, entry.ID, expectedID)
					}
				}
			}

			// Verify that no entries with ID less than minID remain
			for _, entry := range entries {
				if entry.ID < minID {
					t.Errorf("Found entry with ID %s, which is less than minID %s", entry.ID, minID)
				}
			}
		})
	}
}

func TestTrimByMinIDExplicit(t *testing.T) {
	tests := []struct {
		name              string
		entries           []StreamEntry
		minID             string
		expectedTrimmed   int
		expectedRemaining []string
	}{
		{
			name: "Trim none",
			entries: []StreamEntry{
				{ID: "1000-0", Fields: [][]string{{"key", "value1"}}},
				{ID: "1001-0", Fields: [][]string{{"key", "value2"}}},
				{ID: "1002-0", Fields: [][]string{{"key", "value3"}}},
			},
			minID:             "0-1",
			expectedTrimmed:   0,
			expectedRemaining: []string{"1000-0", "1001-0", "1002-0"},
		},
		{
			name: "Trim one",
			entries: []StreamEntry{
				{ID: "1000-0", Fields: [][]string{{"key", "value1"}}},
				{ID: "1001-0", Fields: [][]string{{"key", "value2"}}},
				{ID: "1002-0", Fields: [][]string{{"key", "value3"}}},
			},
			minID:             "1001",
			expectedTrimmed:   1,
			expectedRemaining: []string{"1001-0", "1002-0"},
		},
		{
			name: "Trim all but one",
			entries: []StreamEntry{
				{ID: "1000-0", Fields: [][]string{{"key", "value1"}}},
				{ID: "1001-0", Fields: [][]string{{"key", "value2"}}},
				{ID: "1002-0", Fields: [][]string{{"key", "value3"}}},
			},
			minID:             "1002-0",
			expectedTrimmed:   2,
			expectedRemaining: []string{"1002-0"},
		},
		{
			name: "Trim all",
			entries: []StreamEntry{
				{ID: "1000-0", Fields: [][]string{{"key", "value1"}}},
				{ID: "1001-0", Fields: [][]string{{"key", "value2"}}},
				{ID: "1002-0", Fields: [][]string{{"key", "value3"}}},
			},
			minID:             "1003-0",
			expectedTrimmed:   3,
			expectedRemaining: []string{},
		},
		{
			name: "Trim with non-existent ID",
			entries: []StreamEntry{
				{ID: "1000-0", Fields: [][]string{{"key", "value1"}}},
				{ID: "1001-0", Fields: [][]string{{"key", "value2"}}},
				{ID: "1002-0", Fields: [][]string{{"key", "value3"}}},
			},
			minID:             "1001-5",
			expectedTrimmed:   2,
			expectedRemaining: []string{"1002-0"},
		},
		{
			name: "Trim with same sequence number",
			entries: []StreamEntry{
				{ID: "1000-0", Fields: [][]string{{"key", "value1"}}},
				{ID: "1000-1", Fields: [][]string{{"key", "value2"}}},
				{ID: "1000-2", Fields: [][]string{{"key", "value3"}}},
			},
			minID:             "1000-1",
			expectedTrimmed:   1,
			expectedRemaining: []string{"1000-1", "1000-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := NewRadixTree()

			for _, entry := range tt.entries {
				tree.AddEntry(entry.ID, entry.Fields)
			}

			trimmed := tree.TrimByMinID(tt.minID)

			if trimmed != tt.expectedTrimmed {
				t.Errorf("TrimByMinID(%s) returned %d, want %d", tt.minID, trimmed, tt.expectedTrimmed)
			}

			entries, err := tree.Range("-", "+", uint64(len(tt.entries)))
			if err != nil {
				t.Fatalf("Failed to get range: %v", err)
			}

			if len(entries) != len(tt.expectedRemaining) {
				t.Errorf("Range returned %d entries, want %d", len(entries), len(tt.expectedRemaining))
			}

			for i, entry := range entries {
				if entry.ID != tt.expectedRemaining[i] {
					t.Errorf("Entry %d has ID %s, want %s", i, entry.ID, tt.expectedRemaining[i])
				}
			}

			// Verify that no entries with ID less than minID remain
			for _, entry := range entries {
				if entry.ID < tt.minID {
					t.Errorf("Found entry with ID %s, which is less than minID %s", entry.ID, tt.minID)
				}
			}
		})
	}
}
