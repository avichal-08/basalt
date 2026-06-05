package store

type Store struct {
	data map[string]string
}

func New() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

func (s *Store) Set(key, value string) {
	s.data[key] = value
}

func (s *Store) Get(key string) (string, bool) {
	value, ok := s.data[key]
	return value, ok
}

func (s *Store) Delete(key string) bool {
	_, ok := s.data[key]
	delete(s.data, key)
	return ok
}
