package store

import (
	"os"
	"time"
)

func (d *DiskStore) startBackgroundCompactor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		_ = d.Compact()
	}
}

func (d *DiskStore) Compact() error {
	tmpPath := d.path + ".tmp"

	tmpAOF, err := NewAOF(tmpPath)
	if err != nil {
		return err
	}

	d.mu.RLock()
	for k, v := range d.data {
		if err := tmpAOF.Write(OpSet, k, v); err != nil {
			d.mu.RUnlock()
			tmpAOF.Close()
			os.Remove(tmpPath)
			return err
		}
	}
	d.mu.RUnlock()

	if err := tmpAOF.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

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
