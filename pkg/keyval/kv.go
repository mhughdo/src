package keyval

import "sync"

type KV interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
}

type kv struct {
	mu    sync.RWMutex
	store map[string][]byte
}

func NewStore() KV {
	return &kv{
		store: make(map[string][]byte),
	}
}

func (kv *kv) Get(key string) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	if val, found := kv.store[key]; found {
		return val, nil
	}
	return nil, nil
}

func (kv *kv) Set(key string, value []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[key] = value
	return nil
}
