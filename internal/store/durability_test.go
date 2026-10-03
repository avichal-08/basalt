package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// concurrent sets on one key must leave memory and the log in the same order,
// so the value seen after a restart equals the value seen before it
func TestConcurrentSetOrderSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "order.aof")
	db, err := NewDiskStore(path)
	if err != nil {
		t.Fatal(err)
	}

	const keys = 200
	var wg sync.WaitGroup
	for k := 0; k < keys; k++ {
		key := fmt.Sprintf("k%d", k)
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				db.Set(key, fmt.Sprintf("v%d", g))
			}(g)
		}
	}
	wg.Wait()

	before := make(map[string]string, keys)
	for k := 0; k < keys; k++ {
		key := fmt.Sprintf("k%d", k)
		v, _ := db.Get(key)
		before[key] = v
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := NewDiskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	for key, want := range before {
		if got, _ := db2.Get(key); got != want {
			t.Fatalf("%s: memory had %q before restart, log replays to %q", key, want, got)
		}
	}
}

// writers hammer the store while Compact runs repeatedly. After a restart every
// key must hold the last value written to it
func TestCompactionUnderConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.aof")
	db, err := NewDiskStore(path)
	if err != nil {
		t.Fatal(err)
	}

	const writers = 4
	const keysPerWriter = 50
	stop := make(chan struct{})
	var wg sync.WaitGroup
	var counter [writers]atomic.Int64

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				n := counter[w].Add(1)
				key := fmt.Sprintf("w%d-k%d", w, n%keysPerWriter)
				if n%7 == 0 {
					db.Delete(key)
				} else {
					db.Set(key, fmt.Sprintf("%d", n))
				}
			}
		}(w)
	}

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if err := db.Compact(); err != nil {
			t.Fatalf("compact: %v", err)
		}
	}
	close(stop)
	wg.Wait()

	want := map[string]string{}
	for w := 0; w < writers; w++ {
		for k := 0; k < keysPerWriter; k++ {
			key := fmt.Sprintf("w%d-k%d", w, k)
			if v, ok := db.Get(key); ok {
				want[key] = v
			}
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := NewDiskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	for w := 0; w < writers; w++ {
		for k := 0; k < keysPerWriter; k++ {
			key := fmt.Sprintf("w%d-k%d", w, k)
			got, ok := db2.Get(key)
			exp, expOK := want[key]
			if ok != expOK || got != exp {
				t.Fatalf("%s: before restart (%q,%v), after restart (%q,%v)", key, exp, expOK, got, ok)
			}
		}
	}
}

// a partial record at the tail (crash mid-write) is truncated; earlier data survives
func TestTornWriteIsTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torn.aof")
	db, _ := NewDiskStore(path)
	db.Set("a", "1")
	db.Set("b", "2")
	db.Close()

	good, _ := os.Stat(path)

	//append the first half of a valid record by hand
	tmp := filepath.Join(t.TempDir(), "rec.aof")
	r, _ := NewAOF(tmp)
	r.Write(OpSet, "c", "3333333333")
	r.Close()
	rec, _ := os.ReadFile(tmp)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0666)
	f.Write(rec[:len(rec)/2])
	f.Close()

	db2, err := NewDiskStore(path)
	if err != nil {
		t.Fatalf("torn tail should be recovered, got: %v", err)
	}
	defer db2.Close()
	if v, _ := db2.Get("a"); v != "1" {
		t.Fatalf("lost a")
	}
	if v, _ := db2.Get("b"); v != "2" {
		t.Fatalf("lost b")
	}
	if _, ok := db2.Get("c"); ok {
		t.Fatalf("partial record must not be applied")
	}
	after, _ := os.Stat(path)
	if after.Size() != good.Size() {
		t.Fatalf("expected truncation to %d bytes, file is %d", good.Size(), after.Size())
	}
}

// leftovers from a crashed compaction must never leak into the new log
func TestStaleTmpFileIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.aof")
	stale, _ := NewAOF(path + ".tmp")
	stale.Write(OpSet, "ghost", "boo")
	stale.Close()

	db, err := NewDiskStore(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Set("real", "1")
	if err := db.Compact(); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db2, _ := NewDiskStore(path)
	defer db2.Close()
	if _, ok := db2.Get("ghost"); ok {
		t.Fatal("stale .tmp content leaked into compacted log")
	}
	if v, _ := db2.Get("real"); v != "1" {
		t.Fatal("lost real key")
	}
}

func TestCloseTwiceDoesNotPanic(t *testing.T) {
	db, _ := NewDiskStore(filepath.Join(t.TempDir(), "close.aof"))
	db.Close()
	db.Close()
}
