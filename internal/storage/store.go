package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pixil98/go-log/log"
)

type Storer[T ValidatingSpec] interface {
	Load(context.Context) error
	Save(string, T) error
	Get(string) T
	GetAll() []T
}

type FileStore[T ValidatingSpec] struct {
	path    string
	records map[string]T

	mu sync.RWMutex
}

func NewFileStore[T ValidatingSpec](path string) *FileStore[T] {
	return &FileStore[T]{
		path: path,
	}
}

func (s *FileStore[T]) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing records when loading
	s.records = map[string]T{}

	err := filepath.Walk(s.path, func(path string, info os.FileInfo, err error) error {
		// Load all json files in the assets path
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			asset, err := s.loadAsset(ctx, path)
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

	err = os.WriteFile(s.filePath(asset.Id()), jsonData, 0644)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
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

func (s *FileStore[T]) GetAll() []T {
	vals := []T{}
	for _, v := range s.records {
		vals = append(vals, v)
	}

	return vals
}

func (s *FileStore[T]) filePath(id string) string {
	return filepath.Join(s.path, fmt.Sprintf("%s.json", id))
}

func (s *FileStore[T]) loadAsset(ctx context.Context, path string) (*Asset[T], error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	defer func() {
		err := file.Close()
		if err != nil {
			log.GetLogger(ctx).Errorf("closing file: %s", err.Error())
		}
	}()

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
