package raft

import "sync"

// This is a sample KV store for testing purposes
// This is an adaptation of the following code https://github.com/eliben/raft/blob/master/part3/storage.go
type KvStore struct {
	mu sync.Mutex
	m  map[string][]byte
}

func NewStorage() *KvStore {
	m := make(map[string][]byte)
	return &KvStore{
		m: m,
	}
}

func (kv *KvStore) Get(key string) ([]byte, bool) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	v, found := kv.m[key]
	return v, found
}

func (kv *KvStore) Set(key string, value []byte) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.m[key] = value
}

func (kv *KvStore) IsSet() bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	return len(kv.m) > 0
}
