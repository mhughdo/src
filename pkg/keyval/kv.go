package keyval

import (
	"sync"
	"time"
)

type KV interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Expire(key string, duration time.Duration)
	PExpire(key string, duration time.Duration)
	ExpireAt(key string, expiry time.Time)
	PExpireAt(key string, expiry time.Time)
	Exists(key string) bool
	DeleteTTL(key string)
}

type kv struct {
	mu     sync.RWMutex
	store  map[string][]byte
	expiry map[string]time.Time
}

func NewStore() KV {
	return &kv{
		store:  make(map[string][]byte),
		expiry: make(map[string]time.Time),
	}
}

func (kv *kv) Get(key string) ([]byte, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	if kv.isExpired(key) {
		delete(kv.store, key)
		delete(kv.expiry, key)
		return nil, nil
	}
	value, found := kv.store[key]
	if !found {
		return nil, nil
	}
	return value, nil
}

func (kv *kv) Set(key string, value []byte) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[key] = value
	delete(kv.expiry, key)
	return nil
}

func (kv *kv) Expire(key string, duration time.Duration) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.expiry[key] = time.Now().Add(duration)
}

func (kv *kv) PExpire(key string, duration time.Duration) {
	kv.Expire(key, duration)
}

func (kv *kv) ExpireAt(key string, expiry time.Time) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.expiry[key] = expiry
}

func (kv *kv) PExpireAt(key string, expiry time.Time) {
	kv.ExpireAt(key, expiry)
}

func (kv *kv) Exists(key string) bool {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	_, found := kv.store[key]
	return found
}

func (kv *kv) isExpired(key string) bool {
	if exp, ok := kv.expiry[key]; ok {
		if exp.Before(time.Now()) {
			return true
		}
	}
	return false
}

func (kv *kv) DeleteTTL(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.expiry, key)
}
