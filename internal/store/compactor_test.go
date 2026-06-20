package store

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCompactorSquash(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "compactor.aof")

	db, err := NewDiskStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	for i := 0; i < 1000; i++ {
		db.Set("player_score", "100")
	}

	info, _ := os.Stat(dbPath)
	bloatedSize := info.Size()

	err = db.Compact()
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	infoAfter, _ := os.Stat(dbPath)
	compactedSize := infoAfter.Size()

	if compactedSize >= bloatedSize {
		t.Fatalf("Expected file to shrink! Bloated: %d bytes, Compacted: %d bytes", bloatedSize, compactedSize)
	}

	val, ok := db.Get("player_score")
	if !ok || val != "100" {
		t.Fatalf("Lost active data during compaction!")
	}
	db.Close()
}

func BenchmarkDiskStore_Compact(b *testing.B) {
	dir := b.TempDir()
	db, _ := NewDiskStore(filepath.Join(dir, "bench_compact.aof"))
	defer db.Close()

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		db.Set(key, "standard_length_value_string")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := db.Compact()
		if err != nil {
			b.Fatalf("Compaction failed during benchmark: %v", err)
		}
	}
}
