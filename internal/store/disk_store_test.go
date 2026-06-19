package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetAndGet(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewDiskStore(filepath.Join(dir, "test.aof"))
	defer s.Close()

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
	dir := t.TempDir()
	s, _ := NewDiskStore(filepath.Join(dir, "test.aof"))
	defer s.Close()

	s.Set("name", "avichal")
	s.Delete("name")

	_, ok := s.Get("name")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestGetMissingKey(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewDiskStore(filepath.Join(dir, "test.aof"))
	defer s.Close()

	_, ok := s.Get("does-not-exist")
	if ok {
		t.Fatal("expected key to not exist")
	}
}

func TestUpdateExistingKey(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewDiskStore(filepath.Join(dir, "test.aof"))
	defer s.Close()

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

func TestDataCorruptionDetection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "corruption.aof")

	db, _ := NewDiskStore(dbPath)
	db.Set("sensor_data", "98.6")
	db.Close()

	file, err := os.OpenFile(dbPath, os.O_RDWR, 0666)
	if err != nil {
		t.Fatalf("Failed to open raw file: %v", err)
	}

	_, err = file.WriteAt([]byte("X"), 15)
	file.Close()

	_, err = NewDiskStore(dbPath)

	if err == nil {
		t.Fatal("Expected database to refuse boot on corrupted data, but it succeeded")
	}
	if !strings.Contains(err.Error(), "data corruption detected") {
		t.Fatalf("Expected CRC32 corruption error, got: %v", err)
	}
}

func BenchmarkDiskStore_Set(b *testing.B) {
	dir := b.TempDir()
	db, _ := NewDiskStore(filepath.Join(dir, "bench_set.aof"))
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Set("bench_key", "bench_value")
	}
}

func BenchmarkDiskStore_Get(b *testing.B) {
	dir := b.TempDir()
	db, _ := NewDiskStore(filepath.Join(dir, "bench_get.aof"))
	defer db.Close()

	db.Set("bench_key", "bench_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.Get("bench_key")
	}
}

func BenchmarkDiskStore_GetConcurrent(b *testing.B) {
	dir := b.TempDir()
	db, _ := NewDiskStore(filepath.Join(dir, "bench_conc.aof"))
	defer db.Close()

	db.Set("bench_key", "bench_value")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = db.Get("bench_key")
		}
	})
}
