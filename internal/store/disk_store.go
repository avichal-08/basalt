package store

import (
	"fmt"
	"strings"
)

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

	err = store.aof.Read(func(line string) {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			return
		}

		command := parts[0]
		switch command {
		case "SET":
			if len(parts) == 3 {
				store.data[parts[1]] = parts[2]
			}
		case "DELETE":
			if len(parts) == 2 {
				delete(store.data, parts[1])
			}
		}
	})

	if err != nil {
		return nil, err
	}

	return store, nil
}

func (d *DiskStore) Set(key, value string) {
	d.data[key] = value

	cmd := fmt.Sprintf("SET %s %s", key, value)
	d.aof.Write(cmd)
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

	cmd := fmt.Sprintf("DELETE %s", key)
	d.aof.Write(cmd)

	return true
}
