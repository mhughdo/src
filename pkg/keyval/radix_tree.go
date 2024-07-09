package keyval

import (
	"bytes"
	"sort"
	"sync"
)

// StreamEntry represents a single entry in a Redis stream.
type StreamEntry struct {
	ID     string
	Fields map[string]string
}

// Node types
const (
	Node4Type = iota
	Node16Type
	Node48Type
	Node256Type
	MaxPrefixLen = 10
)

// RadixNode represents a node in a Radix tree.
type RadixNode struct {
	nodeType int
	prefix   []byte
	children interface{}
	isLeaf   bool
	entries  []StreamEntry // Entries stored in this node if it's a leaf.
	mu       sync.RWMutex
}

type Node4 struct {
	keys     [4]byte
	children [4]*RadixNode
	size     int
}

type Node16 struct {
	keys     [16]byte
	children [16]*RadixNode
	size     int
}

type Node48 struct {
	keys     [256]byte
	children [48]*RadixNode
	size     int
}

type Node256 struct {
	children [256]*RadixNode
}

// RadixTree represents the whole Radix tree.
type RadixTree struct {
	root *RadixNode
	mu   sync.RWMutex
}

// NewRadixNode initializes a new Radix node.
func NewRadixNode(nodeType int, prefix []byte) *RadixNode {
	var children interface{}
	switch nodeType {
	case Node4Type:
		children = &Node4{}
	case Node16Type:
		children = &Node16{}
	case Node48Type:
		children = &Node48{}
	case Node256Type:
		children = &Node256{}
	}
	return &RadixNode{
		nodeType: nodeType,
		prefix:   prefix,
		children: children,
	}
}

// NewRadixTree initializes a new Radix tree.
func NewRadixTree() *RadixTree {
	return &RadixTree{
		root: NewRadixNode(Node4Type, nil),
	}
}

// AddEntry adds a stream entry to the Radix tree.
func (t *RadixTree) AddEntry(id string, entry StreamEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for {
		commonPrefixLen := commonPrefixLength(node.prefix, []byte(id))
		if commonPrefixLen == len(node.prefix) {
			id = id[commonPrefixLen:]
			if len(id) == 0 {
				node.isLeaf = true
				node.entries = append(node.entries, entry)
				return
			}

			c := id[0]
			child := node.findChild(c)
			if child == nil {
				newNode := NewRadixNode(Node4Type, []byte(id))
				newNode.entries = append(newNode.entries, entry)
				newNode.isLeaf = true
				node.addChild(c, newNode)
				return
			}
			node = child
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
	var traverse func(*RadixNode, []byte)

	traverse = func(node *RadixNode, prefix []byte) {
		if node == nil || (count > 0 && uint64(len(result)) >= count) {
			return
		}

		currentID := append(prefix, node.prefix...)
		var startCondition, endCondition bool
		if inclusiveStart {
			startCondition = bytes.Compare(currentID, []byte(startID)) >= 0
		} else {
			startCondition = bytes.Compare(currentID, []byte(startID)) > 0
		}

		if inclusiveEnd {
			endCondition = bytes.Compare(currentID, []byte(endID)) <= 0
		} else {
			endCondition = bytes.Compare(currentID, []byte(endID)) < 0
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

		switch node.nodeType {
		case Node4Type:
			for i := 0; i < node.children.(*Node4).size; i++ {
				traverse(node.children.(*Node4).children[i], currentID)
			}
		case Node16Type:
			for i := 0; i < node.children.(*Node16).size; i++ {
				traverse(node.children.(*Node16).children[i], currentID)
			}
		case Node48Type:
			for i := 0; i < 256; i++ {
				idx := node.children.(*Node48).keys[i]
				if idx > 0 {
					traverse(node.children.(*Node48).children[idx-1], currentID)
				}
			}
		case Node256Type:
			for i := 0; i < 256; i++ {
				if node.children.(*Node256).children[i] != nil {
					traverse(node.children.(*Node256).children[i], currentID)
				}
			}
		}
	}

	traverse(t.root, nil)
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
	child := NewRadixNode(Node4Type, n.prefix[position:])
	child.children = n.children
	child.isLeaf = n.isLeaf
	child.entries = n.entries

	n.prefix = n.prefix[:position]
	n.children = &Node4{
		keys:     [4]byte{child.prefix[0]},
		children: [4]*RadixNode{child},
		size:     1,
	}
	n.isLeaf = false
	n.entries = nil
}

// commonPrefixLength returns the length of the common prefix between two byte slices.
func commonPrefixLength(a, b []byte) int {
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

// findChild finds the child node associated with the given key.
func (n *RadixNode) findChild(key byte) *RadixNode {
	switch n.nodeType {
	case Node4Type:
		for i := 0; i < n.children.(*Node4).size; i++ {
			if n.children.(*Node4).keys[i] == key {
				return n.children.(*Node4).children[i]
			}
		}
	case Node16Type:
		for i := 0; i < n.children.(*Node16).size; i++ {
			if n.children.(*Node16).keys[i] == key {
				return n.children.(*Node16).children[i]
			}
		}
	case Node48Type:
		idx := n.children.(*Node48).keys[key]
		if idx > 0 {
			return n.children.(*Node48).children[idx-1]
		}
	case Node256Type:
		return n.children.(*Node256).children[key]
	}
	return nil
}

// addChild adds a child node to the current node.
func (n *RadixNode) addChild(key byte, node *RadixNode) {
	switch n.nodeType {
	case Node4Type:
		children := n.children.(*Node4)
		if children.size < 4 {
			children.keys[children.size] = key
			children.children[children.size] = node
			children.size++
		} else {
			n.grow()
			n.addChild(key, node)
		}
	case Node16Type:
		children := n.children.(*Node16)
		if children.size < 16 {
			children.keys[children.size] = key
			children.children[children.size] = node
			children.size++
		} else {
			n.grow()
			n.addChild(key, node)
		}
	case Node48Type:
		children := n.children.(*Node48)
		if children.size < 48 {
			idx := children.size
			children.keys[key] = byte(idx + 1)
			children.children[idx] = node
			children.size++
		} else {
			n.grow()
			n.addChild(key, node)
		}
	case Node256Type:
		children := n.children.(*Node256)
		children.children[key] = node
	}
}

// grow grows the node to a larger type.
func (n *RadixNode) grow() {
	switch n.nodeType {
	case Node4Type:
		newNode := &Node16{}
		oldNode := n.children.(*Node4)
		copy(newNode.keys[:], oldNode.keys[:])
		copy(newNode.children[:], oldNode.children[:])
		newNode.size = oldNode.size
		n.children = newNode
		n.nodeType = Node16Type
	case Node16Type:
		newNode := &Node48{}
		oldNode := n.children.(*Node16)
		for i := 0; i < oldNode.size; i++ {
			newNode.keys[oldNode.keys[i]] = byte(i + 1)
			newNode.children[i] = oldNode.children[i]
		}
		newNode.size = oldNode.size
		n.children = newNode
		n.nodeType = Node48Type
	case Node48Type:
		newNode := &Node256{}
		oldNode := n.children.(*Node48)
		for i := 0; i < 256; i++ {
			if oldNode.keys[i] > 0 {
				newNode.children[i] = oldNode.children[oldNode.keys[i]-1]
			}
		}
		n.children = newNode
		n.nodeType = Node256Type
	}
}
