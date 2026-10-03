package store

import (
	"os"
	"time"
)

func (d *DiskStore) startBackgroundCompactor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = d.Compact()
		case <-d.stopCompactor:
			return
		}
	}
}

func (d *DiskStore) Compact() error {
	if !d.compactRunMu.TryLock() {
		return nil // another compaction is already running
	}
	defer d.compactRunMu.Unlock()

	tmpPath := d.path + ".tmp"
	_ = os.Remove(tmpPath) // never inherit leftovers from a crashed run

	tmpAOF, err := NewAOF(tmpPath)
	if err != nil {
		return err
	}

	//turn on the sidecar buffer to catch incoming writes
	d.compactionMu.Lock()
	d.isCompacting.Store(true)
	d.compactionMu.Unlock()

	//phase 1: snapshot (writers keep running freely)
	for i := 0; i < numShards; i++ {
		s := d.shards[i]

		s.mu.RLock()
		for k, v := range s.data {
			if err := tmpAOF.Write(OpSet, k, v); err != nil {
				s.mu.RUnlock()
				d.abortCompaction(tmpPath, tmpAOF)
				return err
			}
		}
		s.mu.RUnlock()
	}

	//phase 2: drain the buffer and swap (writers paused)
	d.swapMu.Lock()
	defer d.swapMu.Unlock()

	d.compactionMu.Lock()
	d.isCompacting.Store(false)
	buf := d.compactionBuffer
	d.compactionBuffer = nil
	d.compactionMu.Unlock()

	for _, diff := range buf {
		if err := tmpAOF.Write(diff.op, diff.key, diff.value); err != nil {
			tmpAOF.Close()
			os.Remove(tmpPath)
			return err
		}
	}

	if err := tmpAOF.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := d.aof.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	renameErr := os.Rename(tmpPath, d.path)
	if renameErr != nil {
		os.Remove(tmpPath)
	}

	//whether or not the rename worked, d.path holds a valid log
	//reopen it so the store never keeps pointing at a closed file
	newAof, err := NewAOF(d.path)
	if err != nil {
		return err
	}
	d.aof = newAof

	return renameErr
}

func (d *DiskStore) abortCompaction(tmpPath string, tmpAOF *AOF) {
	tmpAOF.Close()
	os.Remove(tmpPath)

	d.compactionMu.Lock()
	defer d.compactionMu.Unlock()
	d.isCompacting.Store(false)
	d.compactionBuffer = nil
}
