package store

type Store interface {
	Set(key, value string)
	Get(key string) (string, bool)
	Delete(key string) bool
}

type MemoryStore struct {
	data map[string]string
}

func New() Store {
	return &MemoryStore{
		data: make(map[string]string),
	}
}

func (m *MemoryStore) Set(key, value string) {
	m.data[key] = value
}

func (m *MemoryStore) Get(key string) (string, bool) {
	value, ok := m.data[key]
	return value, ok
}

func (m *MemoryStore) Delete(key string) bool {
	_, ok := m.data[key]
	delete(m.data, key)
	return ok
}