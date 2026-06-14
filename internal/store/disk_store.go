package store

type DiskStore struct {
	data map[string]string
	aof  *AOF
}

func NewDiskStore(path string) (Store, error) {
	aof, err := NewAOF(path)
	if err != nil {
		return nil, err
	}

	store := &DiskStore{
		data: make(map[string]string),
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

func (d *DiskStore) Set(key, value string) {
	d.data[key] = value

	d.aof.Write(OpSet, key, value)
}

func (d *DiskStore) Get(key string) (string, bool) {
	value, ok := d.data[key]
	return value, ok
}

func (d *DiskStore) Delete(key string) bool {
	_, ok := d.data[key]
	if !ok {
		return false
	}

	delete(d.data, key)

	d.aof.Write(OpDelete, key, "")

	return true
}
