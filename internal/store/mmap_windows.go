package store

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
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

	hMap, err := syscall.CreateFileMapping(syscall.Handle(f.Fd()), nil, syscall.PAGE_READONLY, 0, 0, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateFileMapping failed: %v", err)
	}
	defer syscall.CloseHandle(hMap)

	addr, err := syscall.MapViewOfFile(hMap, syscall.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		return nil, nil, fmt.Errorf("MapViewOfFile failed: %v", err)
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)

	unmap := func() error {
		return syscall.UnmapViewOfFile(addr)
	}

	return data, unmap, nil
}
