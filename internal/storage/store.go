package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type Storer[T ValidatingSpec] interface {
	Save(string, T) error
	Get(string) T
	GetAll() map[string]T
}

type FileStore[T ValidatingSpec] struct {
	path    string
	records map[string]T

	mu sync.RWMutex
}

func NewFileStore[T ValidatingSpec](path string) (*FileStore[T], error) {
	s := &FileStore[T]{
		path:    path,
		records: map[string]T{},
	}

	err := s.load()
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *FileStore[T]) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing records when loading
	s.records = map[string]T{}

	err := filepath.Walk(s.path, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Load all json files in the assets path
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			asset, err := s.loadAsset(path)
			if err != nil {
				return err
			}

			err = asset.Validate()
			if err != nil {
				return fmt.Errorf("validating %s: %w", filepath.Base(path), err)
			}

			// Error if the key is already in use
			_, ok := s.records[asset.Id()]
			if ok {
				return fmt.Errorf("duplicate key detected: %s", asset.Id())
			}

			s.records[asset.Id()] = asset.Spec
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (s *FileStore[T]) Save(id string, o T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update cached value
	s.records[id] = o

	// Save asset to file
	asset := &Asset[T]{
		Version:    1,
		Identifier: id,
		Spec:       o,
	}

	jsonData, err := json.Marshal(asset)
	if err != nil {
		return fmt.Errorf("marshalling json: %w", err)
	}

	return atomicWrite(s.filePath(asset.Id()), jsonData, 0644)
}

// atomicWrite writes data to a temp file then renames it to the target path.
// This prevents partial or empty files if the process is interrupted.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		if removeErr := os.Remove(tmp); removeErr != nil {
			slog.Warn("failed to remove temp file after rename failure", "path", tmp, "error", removeErr)
		}
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func (s *FileStore[T]) Get(id string) T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.records[id]

	if !ok {
		//TODO: maybe this should error(?) I wouldn't want to kill the server for it though
		var nilVal T
		return nilVal
	}

	return val
}

func (s *FileStore[T]) GetAll() map[string]T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vals := map[string]T{}
	for id, v := range s.records {
		vals[id] = v
	}

	return vals
}

func (s *FileStore[T]) filePath(id string) string {
	return filepath.Join(s.path, fmt.Sprintf("%s.json", id))
}

func (s *FileStore[T]) loadAsset(path string) (*Asset[T], error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	// Ignoring close error - file is read-only, error is not actionable
	defer func() { _ = file.Close() }()

	jsonData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var spec T
	asset := &Asset[T]{
		Spec: spec,
	}
	err = json.Unmarshal(jsonData, asset)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling asset: %w", err)
	}

	return asset, nil
}
