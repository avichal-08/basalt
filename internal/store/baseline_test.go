package store

import (
	"sync"
	"testing"
)

type mutexMap struct {
	mu sync.RWMutex
	m  map[string]string
}

func BenchmarkMutexMap_Get(b *testing.B) {
	db := &mutexMap{
		m: map[string]string{
			"bench_key": "bench_value",
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		db.mu.RLock()
		_, _ = db.m["bench_key"]
		db.mu.RUnlock()
	}
}

func BenchmarkMutexMap_GetConcurrent(b *testing.B) {
	db := &mutexMap{
		m: map[string]string{
			"bench_key": "bench_value",
		},
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db.mu.RLock()
			_, _ = db.m["bench_key"]
			db.mu.RUnlock()
		}
	})
}
