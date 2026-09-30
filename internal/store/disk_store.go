package store

import (
	"sync"
	"sync/atomic"
	"time"
)

const numShards = 32

type shard struct {
	mu   sync.RWMutex
	data map[string]string
}

type CompactionDiff struct {
	op    byte
	key   string
	value string
}

type DiskStore struct {
	shards [numShards]*shard
	aof    *AOF
	path   string

	isCompacting     atomic.Bool
	compactionMu     sync.Mutex
	compactionBuffer []CompactionDiff
	stopCompactor    chan struct{}
}

func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= prime32
	}
	return hash
}

func (d *DiskStore) getShard(key string) *shard {
	return d.shards[fnv32(key)%numShards]
}

func NewDiskStore(path string) (*DiskStore, error) {
	aof, err := NewAOF(path)
	if err != nil {
		return nil, err
	}

	store := &DiskStore{
		aof:           aof,
		path:          path,
		stopCompactor: make(chan struct{}),
	}

	for i := 0; i < numShards; i++ {
		store.shards[i] = &shard{
			data: make(map[string]string),
		}
	}

	err = store.aof.Read(func(op byte, key, value string) {
		s := store.getShard(key)
		switch op {
		case OpSet:
			s.data[key] = value
		case OpDelete:
			delete(s.data, key)
		}
	})
	if err != nil {
		store.aof.Close()
		return nil, err
	}

	go store.startBackgroundCompactor(5 * time.Minute)
	return store, nil
}

func (d *DiskStore) trackCompaction(op byte, key, value string) {
	if !d.isCompacting.Load() {
		return
	}

	d.compactionMu.Lock()
	defer d.compactionMu.Unlock()

	if d.isCompacting.Load() {
		d.compactionBuffer = append(d.compactionBuffer, CompactionDiff{op: op, key: key, value: value})
	}
}

func (d *DiskStore) Set(key, value string) error {
	if err := d.aof.Write(OpSet, key, value); err != nil {
		return err
	}

	s := d.getShard(key)
	s.mu.Lock()
	s.data[key] = value
	s.mu.Unlock()

	d.trackCompaction(OpSet, key, value)
	return nil
}

func (d *DiskStore) Get(key string) (string, bool) {
	s := d.getShard(key)
	s.mu.RLock()
	value, ok := s.data[key]
	s.mu.RUnlock()
	return value, ok
}

func (d *DiskStore) Delete(key string) (bool, error) {
	s := d.getShard(key)

	s.mu.Lock()
	_, ok := s.data[key]
	if !ok {
		s.mu.Unlock()
		return false, nil
	}
	delete(s.data, key)
	s.mu.Unlock()

	if err := d.aof.Write(OpDelete, key, ""); err != nil {
		return false, err
	}

	d.trackCompaction(OpDelete, key, "")
	return true, nil
}

func (d *DiskStore) Close() error {
	close(d.stopCompactor)
	return d.aof.Close()
}
