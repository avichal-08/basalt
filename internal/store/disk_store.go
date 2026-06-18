package store

import "sync"

type DiskStore struct {
	mu   sync.RWMutex
	data map[string]string
	aof  *AOF
}

func NewDiskStore(path string) (*DiskStore, error) {
	aof, err := NewAOF(path)
	if err != nil {
		return nil, err
	}

	store := &DiskStore{
		data: make(map[string]string, 100000),
		aof:  aof,
	}

	err = store.aof.Read(func(op byte, key, value string) {
		switch op {
		case OpSet:
			store.data[key] = value
		case OpDelete:
			delete(store.data, key)
		}
	})

	if err != nil {
		return nil, err
	}

	return store, nil
}

func (d *DiskStore) Set(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	err := d.aof.Write(OpSet, key, value)
	if err != nil {
		return err
	}

	d.data[key] = value
	return nil
}

func (d *DiskStore) Get(key string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	value, ok := d.data[key]
	return value, ok
}

func (d *DiskStore) Delete(key string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, ok := d.data[key]
	if !ok {
		return false, nil
	}

	err := d.aof.Write(OpDelete, key, "")
	if err != nil {
		return false, err
	}

	delete(d.data, key)
	return true, nil
}

func (d *DiskStore) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.aof.Close()
}
