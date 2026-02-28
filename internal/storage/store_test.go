package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pixil98/go-testutil"
)

// mockStoreSpec implements ValidatingSpec for testing FileStore
type mockStoreSpec struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func (s *mockStoreSpec) Validate() error {
	return nil
}

func TestNewFileStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertEqual(t, "path", store.path, tmpDir)
	testutil.AssertEqual(t, "records length", len(store.records), 0)
}

func TestNewFileStore_NonExistentDirectory(t *testing.T) {
	_, err := NewFileStore[*mockStoreSpec]("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestNewFileStore_WithExistingAssets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid test assets
	assets := []struct {
		id   string
		spec *mockStoreSpec
	}{
		{"item-1", &mockStoreSpec{Name: "First", Value: 1}},
		{"item-2", &mockStoreSpec{Name: "Second", Value: 2}},
	}

	for _, a := range assets {
		asset := Asset[*mockStoreSpec]{
			Version:    1,
			Identifier: a.id,
			Spec:       a.spec,
		}
		data, err := json.Marshal(asset)
		if err != nil {
			t.Fatalf("failed to marshal test asset: %v", err)
		}
		filePath := filepath.Join(tmpDir, a.id+".json")
		err = os.WriteFile(filePath, data, 0644)
		if err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertEqual(t, "record count", len(store.records), 2)

	item1 := store.Get("item-1")
	if item1 == nil {
		t.Fatal("expected item-1 to be loaded")
	}
	testutil.AssertEqual(t, "item-1 name", item1.Name, "First")
	testutil.AssertEqual(t, "item-1 value", item1.Value, 1)
}

func TestNewFileStore_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "bad.json")
	err := os.WriteFile(filePath, []byte(`{invalid json`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = NewFileStore[*mockStoreSpec](tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewFileStore_ValidationError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create asset with invalid version (0)
	asset := Asset[*mockStoreSpec]{
		Version:    0,
		Identifier: "test",
		Spec:       &mockStoreSpec{Name: "Test", Value: 1},
	}
	data, err := json.Marshal(asset)
	if err != nil {
		t.Fatalf("failed to marshal test asset: %v", err)
	}
	filePath := filepath.Join(tmpDir, "test.json")
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = NewFileStore[*mockStoreSpec](tmpDir)
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestNewFileStore_DuplicateKey(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create two files with the same ID in different directories
	asset := Asset[*mockStoreSpec]{
		Version:    1,
		Identifier: "duplicate-id",
		Spec:       &mockStoreSpec{Name: "Test", Value: 1},
	}
	data, err := json.Marshal(asset)
	if err != nil {
		t.Fatalf("failed to marshal test asset: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "file1.json"), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	err = os.WriteFile(filepath.Join(subDir, "file2.json"), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err = NewFileStore[*mockStoreSpec](tmpDir)
	if err == nil {
		t.Error("expected duplicate key error")
	}
}

func TestNewFileStore_IgnoresNonJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid JSON asset
	asset := Asset[*mockStoreSpec]{
		Version:    1,
		Identifier: "valid",
		Spec:       &mockStoreSpec{Name: "Valid", Value: 1},
	}
	data, err := json.Marshal(asset)
	if err != nil {
		t.Fatalf("failed to marshal test asset: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "valid.json"), data, 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create non-JSON files that should be ignored
	err = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("ignore me"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "data.yaml"), []byte("ignore: me"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertEqual(t, "record count", len(store.records), 1)
}

func TestFileStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}
	store.records = map[string]*mockStoreSpec{
		"existing": {Name: "Test", Value: 42},
	}

	tests := map[string]struct {
		id       string
		expNil   bool
		expName  string
		expValue int
	}{
		"get existing record": {
			id:       "existing",
			expNil:   false,
			expName:  "Test",
			expValue: 42,
		},
		"get non-existing record": {
			id:     "nonexistent",
			expNil: true,
		},
		"get empty id": {
			id:     "",
			expNil: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := store.Get(tt.id)

			if tt.expNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("expected non-nil result")
					return
				}
				testutil.AssertEqual(t, "name", result.Name, tt.expName)
				testutil.AssertEqual(t, "value", result.Value, tt.expValue)
			}
		})
	}
}

func TestFileStore_GetAll(t *testing.T) {
	tests := map[string]struct {
		records  map[string]*mockStoreSpec
		expCount int
	}{
		"empty records": {
			records:  map[string]*mockStoreSpec{},
			expCount: 0,
		},
		"single record": {
			records: map[string]*mockStoreSpec{
				"one": {Name: "One", Value: 1},
			},
			expCount: 1,
		},
		"multiple records": {
			records: map[string]*mockStoreSpec{
				"one":   {Name: "One", Value: 1},
				"two":   {Name: "Two", Value: 2},
				"three": {Name: "Three", Value: 3},
			},
			expCount: 3,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}
			store.records = tt.records

			result := store.GetAll()

			testutil.AssertEqual(t, "count", len(result), tt.expCount)

			// Verify it's a copy, not the original
			if len(tt.records) > 0 {
				for k := range result {
					delete(result, k)
					break
				}
				if len(store.records) != tt.expCount {
					t.Errorf("GetAll should return a copy, not the original map")
				}
			}
		})
	}
}

func TestFileStore_Save(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	spec := &mockStoreSpec{Name: "TestItem", Value: 100}

	err = store.Save("test-id", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify in-memory cache was updated
	cached := store.Get("test-id")
	if cached == nil {
		t.Fatal("expected cached record")
	}
	testutil.AssertEqual(t, "cached name", cached.Name, "TestItem")
	testutil.AssertEqual(t, "cached value", cached.Value, 100)

	// Verify file was written
	filePath := filepath.Join(tmpDir, "test-id.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	var asset Asset[*mockStoreSpec]
	err = json.Unmarshal(data, &asset)
	if err != nil {
		t.Fatalf("failed to unmarshal saved data: %v", err)
	}

	testutil.AssertEqual(t, "asset version", asset.Version, uint(1))
	testutil.AssertEqual(t, "asset id", asset.Identifier, "test-id")
	testutil.AssertEqual(t, "spec name", asset.Spec.Name, "TestItem")
	testutil.AssertEqual(t, "spec value", asset.Spec.Value, 100)
}

func TestFileStore_Save_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	// Save initial
	err = store.Save("test-id", &mockStoreSpec{Name: "Initial", Value: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Save updated
	err = store.Save("test-id", &mockStoreSpec{Name: "Updated", Value: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify update
	cached := store.Get("test-id")
	testutil.AssertEqual(t, "name", cached.Name, "Updated")
	testutil.AssertEqual(t, "value", cached.Value, 2)
}

func TestFileStore_filePath(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore[*mockStoreSpec](tmpDir)
	if err != nil {
		t.Fatalf("unexpected error creating store: %v", err)
	}

	result := store.filePath("test-id")

	expected := filepath.Join(tmpDir, "test-id.json")
	testutil.AssertEqual(t, "file path", result, expected)
}
