package keyval

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var entrySlicePool = sync.Pool{
	New: func() interface{} {
		return make([]StreamEntry, 0, 100)
	},
}

type StreamEntry struct {
	ID     string
	Fields [][]string
}

type NodeChildren interface {
	findChild(key byte) *RadixNode
	addChild(key byte, node *RadixNode) (NodeChildren, bool)
	getChild(index int) *RadixNode
	grow() NodeChildren
	size() int
}

const (
	Node4Type = iota
	Node16Type
	Node48Type
	Node256Type
	MaxPrefixLen = 10
)

type RadixNode struct {
	nodeType int
	prefix   []byte
	children NodeChildren
	isLeaf   bool
	entries  []StreamEntry
	mu       sync.RWMutex
}

type Node4 struct {
	keys     [4]byte
	children [4]*RadixNode
	count    int
}

type Node16 struct {
	keys     [16]byte
	children [16]*RadixNode
	count    int
}

type Node48 struct {
	keys     [256]byte
	children [48]*RadixNode
	count    int
}

type Node256 struct {
	children [256]*RadixNode
}

type RadixTree struct {
	root          *RadixNode
	mu            sync.RWMutex
	size          int
	lastTimestamp int64
	sequence      int64
	lastID        string
}

func NewRadixNode(prefix []byte) *RadixNode {
	return &RadixNode{
		prefix:   prefix,
		children: &Node4{},
		isLeaf:   false,
		nodeType: Node4Type,
	}
}

func NewRadixTree() *RadixTree {
	return &RadixTree{
		root:          NewRadixNode(nil),
		size:          0,
		lastTimestamp: 0,
		sequence:      0,
		lastID:        "",
	}
}

func (t *RadixTree) AddEntry(originalID string, fields [][]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry := StreamEntry{
		ID:     originalID,
		Fields: fields,
	}

	node := t.root
	id := originalID

	for {
		commonPrefixLen := commonPrefixLength(node.prefix, []byte(id))
		if commonPrefixLen == len(node.prefix) {
			id = id[commonPrefixLen:]
			if len(id) == 0 {
				node.mu.Lock()
				node.isLeaf = true
				if cap(node.entries) == 0 {
					node.entries = make([]StreamEntry, 0, 4)
				}
				node.entries = append(node.entries, entry)
				node.mu.Unlock()
				t.size++
				t.updateLastID(originalID)
				return
			}

			c := id[0]
			child := node.findChild(c)
			if child == nil {
				newNode := NewRadixNode([]byte(id))
				newNode.entries = append(newNode.entries, entry)
				newNode.isLeaf = true
				node.addChild(c, newNode)
				t.size++
				t.updateLastID(originalID)
				return
			}
			node = child
		} else {
			node.split(commonPrefixLen)
		}
	}
}

func (t *RadixTree) updateLastID(id string) {
	if id > t.lastID {
		t.lastID = id
		parts := strings.Split(id, "-")
		if len(parts) == 2 {
			timestamp, err := strconv.ParseInt(parts[0], 10, 64)
			if err == nil {
				t.lastTimestamp = timestamp
				t.sequence, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}
	}
}

func (t *RadixTree) Range(startID, endID string, count uint64) ([]StreamEntry, error) {
	inclusiveStart := true
	inclusiveEnd := true
	if strings.HasPrefix(startID, "(") {
		startID = startID[1:]
		inclusiveStart = false
	}
	if strings.HasPrefix(endID, "(") {
		endID = endID[1:]
		inclusiveEnd = false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if startID == "-" {
		startID = ""
	}
	if endID == "+" {
		endID = "\xff"
	}

	result := entrySlicePool.Get().([]StreamEntry)
	result = result[:0]

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
				node.mu.RLock()
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
							node.mu.RUnlock()
							return
						}
					}
				}
				node.mu.RUnlock()
			}
		}

		for i := 0; i < node.children.size(); i++ {
			child := node.children.getChild(i)
			if child != nil {
				traverse(child, currentID)
			}
		}
	}

	traverse(t.root, nil)
	if count > 0 && uint64(len(result)) > count {
		result = result[:count]
	}

	entrySlicePool.Put(result[:0])
	return result, nil
}

func (n *RadixNode) split(position int) {
	child := NewRadixNode(n.prefix[position:])
	child.children = n.children
	child.isLeaf = n.isLeaf
	child.entries = n.entries

	n.prefix = n.prefix[:position]
	n.children = &Node4{}
	n.addChild(child.prefix[0], child)
	n.isLeaf = false
	n.entries = nil
}

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

func (n *RadixNode) findChild(key byte) *RadixNode {
	return n.children.findChild(key)
}

func (n *RadixNode) addChild(key byte, child *RadixNode) {
	newChildren, _ := n.children.addChild(key, child)
	n.children = newChildren
}

func (n *RadixNode) grow() {
	newChildren, grew := n.children.addChild(0, nil)
	if grew {
		n.children = newChildren
		switch newChildren.(type) {
		case *Node16:
			n.nodeType = Node16Type
		case *Node48:
			n.nodeType = Node48Type
		case *Node256:
			n.nodeType = Node256Type
		}
	}
}

func (t *RadixTree) GenerateID() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	currentTime := time.Now().UnixMilli()
	if currentTime <= t.lastTimestamp {
		t.sequence++
	} else {
		t.lastTimestamp = currentTime
		t.sequence = 0
	}

	return fmt.Sprintf("%d-%d", t.lastTimestamp, t.sequence)
}

func (t *RadixTree) GenerateIncompleteID(incompleteID string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	parts := strings.Split(incompleteID, "-")
	if len(parts) != 2 || parts[1] != "*" {
		return "", errors.New("ERR Invalid stream ID")
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", errors.New("ERR Invalid stream ID")
	}

	if timestamp < t.lastTimestamp {
		timestamp = t.lastTimestamp
	}

	if timestamp == t.lastTimestamp {
		t.sequence++
	} else {
		t.lastTimestamp = timestamp
		t.sequence = 0
	}

	return fmt.Sprintf("%d-%d", t.lastTimestamp, t.sequence), nil
}

func (t *RadixTree) ValidateID(id string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.lastID == "" {
		return true
	}

	return id > t.lastID
}

func (t *RadixTree) TrimBySize(maxSize int) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.size <= maxSize {
		return 0
	}

	trimmed := 0
	var traverse func(*RadixNode) bool
	traverse = func(node *RadixNode) bool {
		if node == nil {
			return false
		}

		if node.isLeaf {
			node.mu.Lock()
			for len(node.entries) > 0 && t.size > maxSize {
				node.entries = node.entries[1:]
				t.size--
				trimmed++
			}
			node.mu.Unlock()
			return t.size <= maxSize
		}

		for i := 0; i < node.children.size(); i++ {
			child := node.children.getChild(i)
			if child != nil {
				if traverse(child) {
					return true
				}
			}
		}
		return false
	}

	traverse(t.root)
	return trimmed
}

func (t *RadixTree) TrimByMinID(minID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	trimmed := 0
	var traverse func(*RadixNode) bool
	traverse = func(node *RadixNode) bool {
		if node == nil {
			return false
		}

		if node.isLeaf {
			node.mu.Lock()
			initialLen := len(node.entries)
			for i, entry := range node.entries {
				if entry.ID >= minID {
					node.entries = node.entries[i:]
					trimmed += i
					t.size -= i
					node.mu.Unlock()
					return true
				}
			}
			trimmed += initialLen
			t.size -= initialLen
			node.entries = nil
			node.mu.Unlock()
			return false
		}

		for i := 0; i < node.children.size(); i++ {
			child := node.children.getChild(i)
			if child != nil {
				if traverse(child) {
					return true
				}
			}
		}
		return false
	}

	traverse(t.root)
	return trimmed
}

func (n *Node4) findChild(key byte) *RadixNode {
	for i := 0; i < n.count; i++ {
		if n.keys[i] == key {
			return n.children[i]
		}
	}
	return nil
}

func (n *Node4) addChild(key byte, node *RadixNode) (NodeChildren, bool) {
	if n.count < 4 {
		n.keys[n.count] = key
		n.children[n.count] = node
		n.count++
		return n, false
	}
	newNode := n.grow()
	newNode, _ = newNode.addChild(key, node)
	return newNode, true
}

func (n *Node4) size() int {
	return n.count
}

func (n *Node16) findChild(key byte) *RadixNode {
	for i := 0; i < n.count; i++ {
		if n.keys[i] == key {
			return n.children[i]
		}
	}
	return nil
}

func (n *Node16) addChild(key byte, node *RadixNode) (NodeChildren, bool) {
	if n.count < 16 {
		n.keys[n.count] = key
		n.children[n.count] = node
		n.count++
		return n, false
	}
	newNode := n.grow()
	newNode, _ = newNode.addChild(key, node)
	return newNode, true
}

func (n *Node16) size() int {
	return n.count
}

func (n *Node48) findChild(key byte) *RadixNode {
	idx := n.keys[key]
	if idx > 0 {
		return n.children[idx-1]
	}
	return nil
}

func (n *Node48) addChild(key byte, node *RadixNode) (NodeChildren, bool) {
	if n.count < 48 {
		idx := n.count
		n.keys[key] = byte(idx + 1)
		n.children[idx] = node
		n.count++
		return n, false
	}
	newNode := n.grow()
	newNode, _ = newNode.addChild(key, node)
	return newNode, true
}

func (n *Node48) size() int {
	return n.count
}

func (n *Node256) findChild(key byte) *RadixNode {
	return n.children[key]
}

func (n *Node256) addChild(key byte, node *RadixNode) (NodeChildren, bool) {
	n.children[key] = node
	return n, false
}

func (n *Node256) size() int {
	count := 0
	for i := 0; i < 256; i++ {
		if n.children[i] != nil {
			count++
		}
	}
	return count
}

func (n *Node4) grow() NodeChildren {
	new := &Node16{count: n.count}
	copy(new.keys[:], n.keys[:])
	copy(new.children[:], n.children[:])
	return new
}

func (n *Node16) grow() NodeChildren {
	new := &Node48{count: n.count}
	for i := 0; i < n.count; i++ {
		new.keys[n.keys[i]] = byte(i + 1)
		new.children[i] = n.children[i]
	}
	return new
}

func (n *Node48) grow() NodeChildren {
	new := &Node256{}
	for i := 0; i < 256; i++ {
		if n.keys[i] != 0 {
			new.children[i] = n.children[n.keys[i]-1]
		}
	}
	return new
}

func (n *Node256) grow() NodeChildren {
	return n
}

func (n *Node4) getChild(index int) *RadixNode {
	if index < 0 || index >= n.count {
		return nil
	}
	return n.children[index]
}

func (n *Node16) getChild(index int) *RadixNode {
	if index < 0 || index >= n.count {
		return nil
	}
	return n.children[index]
}

func (n *Node48) getChild(index int) *RadixNode {
	if index < 0 || index >= n.count {
		return nil
	}
	for i := 0; i < 256; i++ {
		if n.keys[i] == byte(index+1) {
			return n.children[index]
		}
	}
	return nil
}

func (n *Node256) getChild(index int) *RadixNode {
	count := 0
	for i := 0; i < 256; i++ {
		if n.children[i] != nil {
			if count == index {
				return n.children[i]
			}
			count++
		}
	}
	return nil
}
