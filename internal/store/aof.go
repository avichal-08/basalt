package store

import (
	"encoding/binary"
	"io"
	"os"
	"time"
	"unsafe"
)

const (
	OpSet    byte = 1
	OpDelete byte = 2
)

type AOF struct {
	file *os.File
}

func NewAOF(path string) (*AOF, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	aof := &AOF{file: f}
	go aof.syncEverySecond()

	return aof, nil
}

func (a *AOF) syncEverySecond() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		a.file.Sync()
	}
}

func (a *AOF) Write(op byte, key, value string) error {
	keyBytes := []byte(key)
	valBytes := []byte(value)

	keyLenBuf := make([]byte, binary.MaxVarintLen64)
	valLenBuf := make([]byte, binary.MaxVarintLen64)

	n1 := binary.PutUvarint(keyLenBuf, uint64(len(keyBytes)))
	n2 := binary.PutUvarint(valLenBuf, uint64(len(valBytes)))

	totalSize := 1 + n1 + n2 + len(keyBytes) + len(valBytes)
	record := make([]byte, 0, totalSize)

	record = append(record, op)
	record = append(record, keyLenBuf[:n1]...)
	record = append(record, valLenBuf[:n2]...)
	record = append(record, keyBytes...)
	record = append(record, valBytes...)

	_, err := a.file.Write(record)
	return err
}

func (a *AOF) Read(fn func(op byte, key, value string)) error {
	info, err := a.file.Stat()
	if err != nil {
		return err
	}
	fileSize := info.Size()

	if fileSize == 0 {
		return nil
	}

	data := make([]byte, fileSize)

	a.file.Seek(0, 0)
	if _, err := io.ReadFull(a.file, data); err != nil {
		return err
	}

	offset := 0
	length := len(data)

	for offset < length {
		op := data[offset]
		offset++

		keyLen, n := binary.Uvarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n

		valLen, n := binary.Uvarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n

		keyBytes := data[offset : offset+int(keyLen)]
		keyStr := unsafe.String(unsafe.SliceData(keyBytes), len(keyBytes))
		offset += int(keyLen)

		valBytes := data[offset : offset+int(valLen)]
		valStr := unsafe.String(unsafe.SliceData(valBytes), len(valBytes))
		offset += int(valLen)

		fn(op, keyStr, valStr)
	}

	return nil
}

func (a *AOF) Close() error {
	a.file.Sync()
	return a.file.Close()
}
