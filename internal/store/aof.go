package store

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"sync"
	"time"
)

const (
	OpSet    byte = 1
	OpDelete byte = 2
)

var recordPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 4096)
		return &b
	},
}

type AOF struct {
	file *os.File
	done chan struct{}
	wg   sync.WaitGroup
	mu   sync.Mutex
}

func NewAOF(path string) (*AOF, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	aof := &AOF{
		file: f,
		done: make(chan struct{}),
	}

	aof.wg.Add(1)
	go aof.syncEverySecond()

	return aof, nil
}

func (a *AOF) syncEverySecond() {

	defer a.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.file.Sync()
		case <-a.done:
			return
		}
	}
}

func (a *AOF) Write(op byte, key, value string) error {
	kLen := len(key)
	vLen := len(value)

	var vbuf [binary.MaxVarintLen64 * 2]byte
	n1 := binary.PutUvarint(vbuf[:], uint64(kLen))
	n2 := binary.PutUvarint(vbuf[n1:], uint64(vLen))

	totalLen := 4 + 1 + n1 + n2 + kLen + vLen

	bufPtr := recordPool.Get().(*[]byte)
	buf := *bufPtr

	if cap(buf) < totalLen {
		buf = make([]byte, totalLen)
	} else {
		buf = buf[:totalLen]
	}

	buf[4] = op
	copy(buf[5:], vbuf[:n1+n2])
	offset := 5 + n1 + n2

	copy(buf[offset:], key)
	offset += kLen
	copy(buf[offset:], value)

	checksum := crc32.ChecksumIEEE(buf[4:totalLen])
	binary.LittleEndian.PutUint32(buf[0:4], checksum)

	a.mu.Lock()
	_, err := a.file.Write(buf[:totalLen])
	a.mu.Unlock()

	*bufPtr = buf[:0]
	recordPool.Put(bufPtr)

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

	data, unmap, err := mmapFile(a.file)
	if err != nil {
		return err
	}

	offset := 0
	lastValidOffset := 0
	length := len(data)

	for offset < length {
		if offset+7 > length {
			break
		}

		storedCRC := binary.LittleEndian.Uint32(data[offset : offset+4])
		payloadStart := offset + 4
		curr := payloadStart

		op := data[curr]
		curr++

		keyLen, n1 := binary.Uvarint(data[curr:])
		if n1 <= 0 || curr+n1 > length {
			break
		}
		curr += n1

		valLen, n2 := binary.Uvarint(data[curr:])
		if n2 <= 0 || curr+n2 > length {
			break
		}
		curr += n2

		remaining := uint64(length - curr)
		if keyLen > remaining || valLen > remaining-keyLen {
			break
		}
		totalRecordLen := curr + int(keyLen) + int(valLen)

		keyBytes := data[curr : curr+int(keyLen)]
		curr += int(keyLen)
		valBytes := data[curr : curr+int(valLen)]
		curr += int(valLen)

		actualCRC := crc32.ChecksumIEEE(data[payloadStart:totalRecordLen])
		if actualCRC != storedCRC {
			err := fmt.Errorf("data corruption detected on key: %s", string(keyBytes))
			_ = unmap()
			return err
		}

		fn(op, string(keyBytes), string(valBytes))
		offset = totalRecordLen
		lastValidOffset = totalRecordLen
	}

	err = unmap()
	if err != nil {
		return err
	}

	if int64(lastValidOffset) < fileSize {
		err = a.file.Truncate(int64(lastValidOffset))
		if err != nil {
			fmt.Printf("Warning: failed to truncate torn write: %v\n", err)
		}
	}

	return nil
}

func (a *AOF) Close() error {
	close(a.done)
	a.wg.Wait()
	a.file.Sync()
	return a.file.Close()
}
