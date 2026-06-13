package store

import (
	"bufio"
	"os"
	"time"
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

func (a *AOF) Write(cmd string) error {
	_, err := a.file.WriteString(cmd + "\n")
	return err
}

func (a *AOF) Read(fn func(string)) error {
	a.file.Seek(0, 0)

	scanner := bufio.NewScanner(a.file)
	for scanner.Scan() {
		fn(scanner.Text())
	}

	return scanner.Err()
}

func (a *AOF) Close() error {
	a.file.Sync()
	return a.file.Close()
}
