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
	tmpPath := d.path + ".tmp"
	tmpAOF, err := NewAOF(tmpPath)
	if err != nil {
		return err
	}

	d.compactionMu.Lock()
	d.compactionBuffer = make([]CompactionDiff, 0, 1024)
	d.isCompacting.Store(true)
	d.compactionMu.Unlock()

	for i := 0; i < numShards; i++ {
		s := d.shards[i]

		s.mu.RLock()
		keys := make([]string, 0, len(s.data))
		for k := range s.data {
			keys = append(keys, k)
		}
		s.mu.RUnlock()

		for _, k := range keys {
			s.mu.RLock()
			val, exists := s.data[k]
			s.mu.RUnlock()

			if exists {
				if err := tmpAOF.Write(OpSet, k, val); err != nil {
					d.abortCompaction(tmpPath, tmpAOF)
					return err
				}
			}
		}
	}

	d.compactionMu.Lock()
	defer d.compactionMu.Unlock()

	d.isCompacting.Store(false)

	for _, diff := range d.compactionBuffer {
		if err := tmpAOF.Write(diff.op, diff.key, diff.value); err != nil {
			tmpAOF.Close()
			os.Remove(tmpPath)
			d.compactionBuffer = nil
			return err
		}
	}

	d.compactionBuffer = nil

	if err := tmpAOF.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := d.aof.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, d.path); err != nil {
		return err
	}

	newAof, err := NewAOF(d.path)
	if err != nil {
		return err
	}

	d.aof = newAof
	return nil
}

func (d *DiskStore) abortCompaction(tmpPath string, tmpAOF *AOF) {
	tmpAOF.Close()
	os.Remove(tmpPath)

	d.compactionMu.Lock()
	defer d.compactionMu.Unlock()
	d.isCompacting.Store(false)
	d.compactionBuffer = nil
}
