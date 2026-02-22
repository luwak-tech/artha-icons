package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Storage struct {
	filePath string
	mu       sync.RWMutex
	Mapping  map[string]string // ISIN -> Filename
}

func New(filePath string) (*Storage, error) {
	s := &Storage{
		filePath: filePath,
		Mapping:  make(map[string]string),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

func (s *Storage) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &s.Mapping)
}

func (s *Storage) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.Mapping, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Storage) Has(isin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.Mapping[isin]
	return exists
}

func (s *Storage) Add(isin, filename string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Mapping[isin] = filename
}
