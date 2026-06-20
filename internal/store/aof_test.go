package store

import (
	"path/filepath"
	"testing"
)

func TestAOFWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	aofPath := filepath.Join(dir, "raw_test.aof")

	aof, err := NewAOF(aofPath)
	if err != nil {
		t.Fatalf("failed to create aof: %v", err)
	}

	err = aof.Write(OpSet, "core_key", "core_value")
	if err != nil {
		t.Fatalf("failed to write raw bytes: %v", err)
	}
	aof.Close()

	aof2, err := NewAOF(aofPath)
	if err != nil {
		t.Fatalf("failed to open aof: %v", err)
	}
	defer aof2.Close()

	var readOp byte
	var readKey, readVal string

	err = aof2.Read(func(op byte, key, value string) {
		readOp = op
		readKey = key
		readVal = value
	})

	if err != nil {
		t.Fatalf("failed to read raw bytes: %v", err)
	}

	if readOp != OpSet || readKey != "core_key" || readVal != "core_value" {
		t.Fatalf("Binary mismatch! Got op:%d key:%s val:%s", readOp, readKey, readVal)
	}
}

func BenchmarkAOF_WriteRaw(b *testing.B) {
	dir := b.TempDir()
	aof, _ := NewAOF(filepath.Join(dir, "bench_raw.aof"))
	defer aof.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aof.Write(OpSet, "bench_key", "bench_value")
	}
}
