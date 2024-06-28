package keyval

import (
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

type Value struct {
	Type   ValueType
	Data   any
	Expiry uint64
}

type KV interface {
	RestoreRDB(data map[string]Value)
	Get(key string) []byte
	Set(key string, value []byte) error
	Expire(key string, duration time.Duration)
	PExpire(key string, duration time.Duration)
	ExpireAt(key string, expiry time.Time)
	PExpireAt(key string, expiry time.Time)
	Exists(key string) bool
	DeleteTTL(key string)
	Keys() []string
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
