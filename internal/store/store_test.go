package store

import (
	"fmt"
	"os"
	"testing"
)

func BenchmarkAOFStartup(b *testing.B) {
	tempFile := "benchmark_test.aof"

	aof, err := NewAOF(tempFile)
	if err != nil {
		b.Fatalf("failed to create temp aof: %v", err)
	}

	for i := 0; i < 100000; i++ {
		key := fmt.Sprintf("key_%d", i)
		val := fmt.Sprintf("value_%d", i)
		aof.Write(OpSet, key, val)
	}

	aof.Close()

	defer os.Remove(tempFile)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := NewDiskStore(tempFile)
		if err != nil {
			b.Fatalf("failed to boot disk store: %v", err)
		}
	}
}

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

func TestDeleteExistingKey(t *testing.T) {
	s := New()
	s.Set("name", "avichal")
	s.Delete("name")

	_, ok := s.Get("name")

	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestDeletMissingKey(t *testing.T) {
	s := New()
	ok := s.Delete("key1")

	if ok {
		t.Fatal("expected key to not exist")
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
