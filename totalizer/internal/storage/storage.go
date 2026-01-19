package storage

import (
	"os"
	"strconv"
	"sync"
)

type Storage interface {
	Save(total int) error
	Load() (int, error)
}

type FileStorage struct {
	Filename string
	mu       sync.Mutex
}

func NewFileStorage(filename string) *FileStorage {
	if _, err := os.Stat(filename); err != nil {
		_ = os.WriteFile(filename, []byte("10"), 0644)
	}
	return &FileStorage{
		Filename: filename,
	}
}

func (f *FileStorage) Save(total int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return os.WriteFile(f.Filename, []byte(strconv.Itoa(total)), 0644)
}

func (f *FileStorage) Load() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.Filename)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return value, nil
}
