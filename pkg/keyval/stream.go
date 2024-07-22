package keyval

import (
	"errors"
	"strings"
)

type Stream struct {
	tree *RadixTree
}

func NewStream() *Stream {
	return &Stream{
		tree: NewRadixTree(),
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
		if !s.tree.ValidateID(entry.ID) {
			return "", errors.New("ERR The ID specified in XADD is equal or smaller than the target stream top item")
		}
	}

	s.tree.AddEntry(entry.ID, entry.Fields)
	return entry.ID, nil
}

func (s *Stream) Range(startID, endID string, count uint64) ([]StreamEntry, error) {
	return s.tree.Range(startID, endID, count)
}

func (s *Stream) TrimBySize(maxSize int) int {
	return s.tree.TrimBySize(maxSize)
}

func (s *Stream) TrimByMinID(minID string) int {
	return s.tree.TrimByMinID(minID)
}
