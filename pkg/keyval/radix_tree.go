package keyval

import (
	"sort"
	"sync"
)

// StreamEntry represents a single entry in a Redis stream.
type StreamEntry struct {
	ID     string
	Fields map[string]string
}

// RadixNode represents a node in a Radix tree.
type RadixNode struct {
	prefix    string
	children  map[rune]*RadixNode
	isLeaf    bool
	entries   []StreamEntry  // Entries stored in this node if it's a leaf.
	endpoints map[string]int // Tracks exact matches with the count of entries.
	mu        sync.RWMutex
}

// RadixTree represents the whole Radix tree.
type RadixTree struct {
	root *RadixNode
	mu   sync.RWMutex
}

// NewRadixNode initializes a new Radix node.
func NewRadixNode(prefix string) *RadixNode {
	return &RadixNode{
		prefix:    prefix,
		children:  make(map[rune]*RadixNode),
		entries:   []StreamEntry{},
		endpoints: make(map[string]int),
	}
}

// NewRadixTree initializes a new Radix tree.
func NewRadixTree() *RadixTree {
	return &RadixTree{
		root: NewRadixNode(""),
	}
}

// AddEntry adds a stream entry to the Radix tree.
func (t *RadixTree) AddEntry(id string, entry StreamEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for {
		commonPrefixLen := commonPrefixLength(node.prefix, id)
		if commonPrefixLen == len(node.prefix) {
			id = id[commonPrefixLen:]
			if id == "" {
				node.isLeaf = true
				node.entries = append(node.entries, entry)
				node.endpoints[entry.ID]++
				return
			}

			c := rune(id[0])
			if _, ok := node.children[c]; !ok {
				node.children[c] = NewRadixNode(id)
				node.children[c].entries = append(node.children[c].entries, entry)
				node.children[c].isLeaf = true
				node.children[c].endpoints[entry.ID]++
				return
			}
			node = node.children[c]
		} else {
			node.split(commonPrefixLen)
		}
	}
}

// Range retrieves entries in the Radix tree between startID and endID.
func (t *RadixTree) Range(startID, endID string, count uint64, inclusiveStart bool, inclusiveEnd bool) ([]StreamEntry, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if startID == "-" {
		startID = ""
	}
	if endID == "+" {
		endID = "\xff" // Arbitrary high value
	}

	var result []StreamEntry
	var traverse func(*RadixNode, string)

	traverse = func(node *RadixNode, prefix string) {
		if node == nil || (count > 0 && uint64(len(result)) >= count) {
			return
		}

		currentID := prefix + node.prefix
		var startCondition, endCondition bool
		if inclusiveStart {
			startCondition = currentID >= startID
		} else {
			startCondition = currentID > startID
		}

		if inclusiveEnd {
			endCondition = currentID <= endID
		} else {
			endCondition = currentID < endID
		}

		if startCondition && endCondition {
			if node.isLeaf {
				for _, entry := range node.entries {
					if inclusiveStart {
						startCondition = entry.ID >= startID
					} else {
						startCondition = entry.ID > startID
					}
					if inclusiveEnd {
						endCondition = entry.ID <= endID
					} else {
						endCondition = entry.ID < endID
					}
					if startCondition && endCondition {
						result = append(result, entry)
						if count > 0 && uint64(len(result)) >= count {
							return
						}
					}
				}
			}
		}
		keys := make([]rune, 0, len(node.children))
		for key := range node.children {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			return keys[i] < keys[j]
		})
		for _, key := range keys {
			traverse(node.children[key], currentID)
		}
	}

	traverse(t.root, "")
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	if count > 0 && uint64(len(result)) > count {
		result = result[:count]
	}

	return result, nil
}

// split splits the node at the given position.
func (n *RadixNode) split(position int) {
	child := NewRadixNode(n.prefix[position:])
	child.children = n.children
	child.isLeaf = n.isLeaf
	child.entries = n.entries
	child.endpoints = n.endpoints

	n.prefix = n.prefix[:position]
	n.children = map[rune]*RadixNode{rune(child.prefix[0]): child}
	n.isLeaf = false
	n.entries = []StreamEntry{}
	n.endpoints = make(map[string]int)
}

// commonPrefixLength returns the length of the common prefix between two strings.
func commonPrefixLength(a, b string) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return minLen
}
