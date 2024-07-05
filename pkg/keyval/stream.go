package keyval

import (
	"sync"
)

// Stream represents a Redis stream.
type Stream struct {
	tree *RadixTree
	mu   sync.RWMutex
}

// NewStream initializes a new Stream.
func NewStream() *Stream {
	return &Stream{
		tree: NewRadixTree(),
	}
}

// AddEntry adds an entry to the stream.
func (s *Stream) AddEntry(entry StreamEntry) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tree.AddEntry(entry.ID, entry)
	return entry.ID
}

// Range retrieves entries in the stream between startID and endID.
func (s *Stream) Range(startID, endID string, count uint64, inclusiveStart bool, inclusiveEnd bool) ([]StreamEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tree.Range(startID, endID, count, inclusiveStart, inclusiveEnd)
}
