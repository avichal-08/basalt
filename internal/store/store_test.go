package store

import "testing"

func TestSetAndGet(t *testing.T) {
	s := New()

	s.Set("name", "avichal")

	value, ok := s.Get("name")

	if !ok {
		t.Fatal("expected key to exist")
	}

	if value != "avichal" {
		t.Fatalf("expected avichal, got %s", value)
	}
}

func TestDelete(t *testing.T) {
	s := New()

	s.Set("name", "avichal")

	s.Delete("name")

	_, ok := s.Get("name")

	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestGetMissingKey(t *testing.T) {
	s := New()

	_, ok := s.Get("does-not-exist")

	if ok {
		t.Fatal("expected key to not exist")
	}
}

func TestUpdateExistingKey(t *testing.T) {
	s := New()

	s.Set("name", "avichal")
	s.Set("name", "aditya")

	value, ok := s.Get("name")

	if !ok {
		t.Fatal("expected key to exist")
	}

	if value != "aditya" {
		t.Fatalf("expected aditya, got %s", value)
	}
}
