package store

import (
	"os"
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

	//swapMu orders writers against the compactor's final log swap
	//set/delete hold it for reading; compact holds it for writing
	swapMu sync.RWMutex

	compactRunMu     sync.Mutex //ensures only one compact runs at a time
	isCompacting     atomic.Bool
	compactionMu     sync.Mutex
	compactionBuffer []CompactionDiff
	stopCompactor    chan struct{}
	closeOnce        sync.Once
	closeErr         error
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

	//a leftover temp file from the previous crashed compaction before the rename
	//the main log is still authoritative, so discard it safely
	_ = os.Remove(path + ".tmp")

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
	d.swapMu.RLock()
	defer d.swapMu.RUnlock()

	s := d.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	//log first, then memory, all strictly inside the shard lock
	if err := d.aof.Write(OpSet, key, value); err != nil {
		return err
	}
	s.data[key] = value

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
	d.swapMu.RLock()
	defer d.swapMu.RUnlock()

	s := d.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[key]; !ok {
		return false, nil
	}

	//log first: if the disk write fails, memory is left untouched
	if err := d.aof.Write(OpDelete, key, ""); err != nil {
		return false, err
	}
	delete(s.data, key)

	d.trackCompaction(OpDelete, key, "")
	return true, nil
}

// close is safe to call more than once
func (d *DiskStore) Close() error {
	d.closeOnce.Do(func() {
		close(d.stopCompactor)
		d.swapMu.Lock()
		defer d.swapMu.Unlock()
		d.closeErr = d.aof.Close()
	})
	return d.closeErr
}
