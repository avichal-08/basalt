package store

import (
	"bufio"
	"encoding/binary"
	"io"
	"os"
	"time"
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
	a.file.Seek(0, 0)

	reader := bufio.NewReader(a.file)

	for {
		op, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		keyLen, err := binary.ReadUvarint(reader)
		if err != nil {
			return err
		}

		valLen, err := binary.ReadUvarint(reader)
		if err != nil {
			return err
		}

		keyBuf := make([]byte, keyLen)
		if _, err := io.ReadFull(reader, keyBuf); err != nil {
			return err
		}

		valBuf := make([]byte, valLen)
		if _, err := io.ReadFull(reader, valBuf); err != nil {
			return err
		}

		fn(op, string(keyBuf), string(valBuf))
	}

	return nil
}

func (a *AOF) Close() error {
	a.file.Sync()
	return a.file.Close()
}
