package store

import (
	"strconv"
	"sync"
	"testing"
)

type mutexMap struct {
	mu sync.RWMutex
	m  map[string]string
}

func BenchmarkMutexMap_GetConcurrent(b *testing.B) {
	db := &mutexMap{
		m: make(map[string]string),
	}

	keys := make([]string, 1024)
	for i := 0; i < 1024; i++ {
		keys[i] = "key_" + strconv.Itoa(i)
		db.m[keys[i]] = "bench_value"
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			db.mu.RLock()
			_, _ = db.m[keys[i&1023]]
			db.mu.RUnlock()
			i++
		}
	})
}
