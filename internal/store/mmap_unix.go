//go:build !windows

package store

import (
	"os"
	"syscall"
)

func mmapFile(f *os.File) ([]byte, func() error, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	size := info.Size()

	if size == 0 {
		return nil, func() error { return nil }, nil
	}

	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return nil, nil, err
	}

	unmap := func() error {
		return syscall.Munmap(data)
	}

	return data, unmap, nil
}
