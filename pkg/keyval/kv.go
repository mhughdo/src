package keyval

import (
	"errors"
	"sync"
	"time"
)

type ValueType int

const (
	ValueTypeString ValueType = iota
	ValueTypeList
	ValueTypeHash
	ValueTypeSet
	ValueTypeSortedSet
	ValueTypeStream
	ValueTypeJSON
)

func (t ValueType) String() string {
	switch t {
	case ValueTypeString:
		return "string"
	case ValueTypeList:
		return "list"
	case ValueTypeHash:
		return "hash"
	case ValueTypeSet:
		return "set"
	case ValueTypeSortedSet:
		return "sortedset"
	case ValueTypeStream:
		return "stream"
	case ValueTypeJSON:
		return "json"
	}
	return "unknown"
}

type Value struct {
	Type   ValueType
	Data   any
	Expiry uint64
}

type KV interface {
	RestoreRDB(data map[string]Value)
	GetExpiry(key string) uint64
	IsExpired(key string) bool
	Get(key string) []byte
	Set(key string, value []byte) error
	Expire(key string, duration time.Duration)
	PExpire(key string, duration time.Duration)
	ExpireAt(key string, expiry time.Time)
	PExpireAt(key string, expiry time.Time)
	Exists(key string) bool
	DeleteTTL(key string)
	Keys() []string
	Type(key string) string
	GetStream(key string, createIfNotExists bool) (*Stream, error)
	Export() map[string]Value
}

type kv struct {
	mu    sync.RWMutex
	store map[string]Value
}

func NewStore() KV {
	return &kv{
		store: make(map[string]Value),
	}
}

func (kv *kv) Get(key string) []byte {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	if kv.isExpired(key) {
		delete(kv.store, key)
		return nil
	}
	v, found := kv.store[key]
	if !found || v.Type != ValueTypeString {
		return nil
	}
	return v.Data.([]byte)
}

func (kv *kv) Type(key string) string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	elem, found := kv.store[key]
	if !found {
		return "none"
	}
	return elem.Type.String()
}

func (kv *kv) Set(key string, value []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[key] = Value{
		Type: ValueTypeString,
		Data: value,
	}
	return nil
}

func (kv *kv) Expire(key string, duration time.Duration) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if v, ok := kv.store[key]; ok {
		v.Expiry = uint64(time.Now().Add(duration).UnixMilli())
		kv.store[key] = v
	}
}

func (kv *kv) PExpire(key string, duration time.Duration) {
	kv.Expire(key, duration)
}

func (kv *kv) ExpireAt(key string, expiry time.Time) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if v, ok := kv.store[key]; ok {
		v.Expiry = uint64(expiry.UnixMilli())
		kv.store[key] = v
	}
}

func (kv *kv) PExpireAt(key string, expiry time.Time) {
	kv.ExpireAt(key, expiry)
}

func (kv *kv) Keys() []string {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	keys := make([]string, 0, len(kv.store))
	for key := range kv.store {
		keys = append(keys, key)
	}
	return keys
}

func (kv *kv) RestoreRDB(data map[string]Value) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store = data
}

func (kv *kv) Exists(key string) bool {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	_, found := kv.store[key]
	return found
}

func (kv *kv) isExpired(key string) bool {
	if v, ok := kv.store[key]; ok && v.Expiry > 0 && v.Expiry < uint64(time.Now().UnixMilli()) {
		return true
	}
	return false
}

func (kv *kv) DeleteTTL(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if v, ok := kv.store[key]; ok {
		v.Expiry = 0
		kv.store[key] = v
	}
}

func (kv *kv) GetStream(key string, createIfNotExist bool) (*Stream, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	s, found := kv.store[key]
	if !found {
		if !createIfNotExist {
			return nil, nil
		}
		kv.store[key] = Value{
			Type: ValueTypeStream,
			Data: NewStream(),
		}
		return kv.store[key].Data.(*Stream), nil
	}
	if s.Type != ValueTypeStream {
		return nil, errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")
	}
	return s.Data.(*Stream), nil
}

func (kv *kv) GetExpiry(key string) uint64 {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	if v, ok := kv.store[key]; ok {
		return v.Expiry
	}
	return 0
}

func (kv *kv) IsExpired(key string) bool {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	if v, ok := kv.store[key]; ok {
		return v.Expiry > 0 && v.Expiry < uint64(time.Now().UnixMilli())
	}
	return false
}

func (kv *kv) Export() map[string]Value {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	exportedData := make(map[string]Value, len(kv.store))
	for k, v := range kv.store {
		exportedData[k] = v
	}
	return exportedData
}
