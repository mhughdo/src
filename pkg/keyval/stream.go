package keyval

import (
	"errors"
	"strings"
	"sync"
)

type Stream struct {
	tree        *RadixTree
	subscribers map[chan StreamEntry]struct{}
	mu          sync.RWMutex
}

func NewStream() *Stream {
	return &Stream{
		tree:        NewRadixTree(),
		subscribers: make(map[chan StreamEntry]struct{}),
	}
}

func (s *Stream) AddEntry(entry StreamEntry) (string, error) {
	var err error
	if entry.ID == "*" {
		entry.ID = s.tree.GenerateID()
	} else if strings.HasSuffix(entry.ID, "-*") {
		entry.ID, err = s.tree.GenerateIncompleteID(entry.ID)
		if err != nil {
			return "", err
		}
	} else {
		valid, id := s.tree.ValidateID(entry.ID)
		isLargest := s.tree.IsLargestID(id)
		if !valid || !isLargest {
			if id == "0-0" {
				return "", errors.New("ERR The ID specified in XADD must be greater than 0-0")
			}
			return "", errors.New("ERR The ID specified in XADD is equal or smaller than the target stream top item")
		}
		entry.ID = id
	}

	s.tree.AddEntry(entry.ID, entry.Fields)
	s.notifySubscribers(StreamEntry{ID: entry.ID, Fields: entry.Fields})
	return entry.ID, nil
}

func (s *Stream) Subscribe() chan StreamEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan StreamEntry, 100)
	s.subscribers[ch] = struct{}{}
	return ch
}

func (s *Stream) Unsubscribe(ch chan StreamEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, ch)
	close(ch)
}

func (s *Stream) notifySubscribers(entry StreamEntry) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subscribers {
		select {
		case ch <- entry:
		default:
			// If the channel is full, we skip this notification
		}
	}
}

func (s *Stream) LastID() string {
	return s.tree.LastID
}

func (s *Stream) Range(startID, endID string, count uint64) []StreamEntry {
	var startValid, endValid bool
	startValid, startID = s.tree.ValidateID(startID)
	endValid, endID = s.tree.ValidateID(endID)
	if !startValid || !endValid {
		return nil
	}

	return s.tree.Range(startID, endID, count)
}

func (s *Stream) TrimBySize(maxSize int) int {
	return s.tree.TrimBySize(maxSize)
}

func (s *Stream) TrimByMinID(minID string) int {
	return s.tree.TrimByMinID(minID)
}
