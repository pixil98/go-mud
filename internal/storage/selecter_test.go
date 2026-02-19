package storage

import (
	"bytes"
	"errors"
	"testing"

	"github.com/pixil98/go-testutil"
)

var errMockInvalid = errors.New("mock spec is invalid")

// mockSelectableSpec implements validatingSelectable for testing
type mockSelectableSpec struct {
	name  string
	valid bool
}

func (s *mockSelectableSpec) Validate() error {
	if !s.valid {
		return errMockInvalid
	}
	return nil
}

func (s *mockSelectableSpec) Selector() string {
	return s.name
}

// mockSelectableStorer implements Storer[*mockSelectableSpec] for testing
type mockSelectableStorer struct {
	records map[Identifier]*mockSelectableSpec
}

func (m *mockSelectableStorer) Save(id string, o *mockSelectableSpec) error {
	m.records[Identifier(id)] = o
	return nil
}

func (m *mockSelectableStorer) Get(id string) *mockSelectableSpec {
	return m.records[Identifier(id)]
}

func (m *mockSelectableStorer) GetAll() map[Identifier]*mockSelectableSpec {
	return m.records
}

// mockReadWriter implements io.ReadWriter for testing Prompt
type mockReadWriter struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func (m *mockReadWriter) Read(p []byte) (n int, err error) {
	return m.readBuf.Read(p)
}

func (m *mockReadWriter) Write(p []byte) (n int, err error) {
	return m.writeBuf.Write(p)
}

func TestNewSelectableStorer(t *testing.T) {
	tests := map[string]struct {
		records     map[Identifier]*mockSelectableSpec
		expOptCount int
		expNonEmpty bool
	}{
		"empty store": {
			records:     map[Identifier]*mockSelectableSpec{},
			expOptCount: 0,
			expNonEmpty: false,
		},
		"single item": {
			records: map[Identifier]*mockSelectableSpec{
				"item-1": {name: "Item One", valid: true},
			},
			expOptCount: 1,
			expNonEmpty: true,
		},
		"multiple items": {
			records: map[Identifier]*mockSelectableSpec{
				"item-1": {name: "Item One", valid: true},
				"item-2": {name: "Item Two", valid: true},
				"item-3": {name: "Item Three", valid: true},
			},
			expOptCount: 3,
			expNonEmpty: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mock := &mockSelectableStorer{records: tt.records}
			ss := NewSelectableStorer(mock)

			testutil.AssertEqual(t, "option count", len(ss.options), tt.expOptCount)

			if tt.expNonEmpty {
				if len(ss.output) == 0 {
					t.Errorf("expected output to be non-empty")
				}
			}
		})
	}
}

func TestSelectableStorer_Select(t *testing.T) {
	records := map[Identifier]*mockSelectableSpec{
		"item-a": {name: "Alpha", valid: true},
		"item-b": {name: "Beta", valid: true},
		"item-c": {name: "Gamma", valid: true},
	}
	mock := &mockSelectableStorer{records: records}
	ss := NewSelectableStorer(mock)

	tests := map[string]struct {
		index    int
		expEmpty bool
	}{
		"valid index 1": {
			index:    1,
			expEmpty: false,
		},
		"valid index 2": {
			index:    2,
			expEmpty: false,
		},
		"valid index 3": {
			index:    3,
			expEmpty: false,
		},
		"index 0 is invalid": {
			index:    0,
			expEmpty: true,
		},
		"negative index": {
			index:    -1,
			expEmpty: true,
		},
		"index too large": {
			index:    4,
			expEmpty: true,
		},
		"index way too large": {
			index:    100,
			expEmpty: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := ss.Select(tt.index)

			if tt.expEmpty {
				testutil.AssertEqual(t, "result", result, Identifier(""))
			} else {
				if result == "" {
					t.Errorf("expected non-empty identifier, got empty")
				}
				// Verify the returned identifier exists in original records
				if _, ok := records[result]; !ok {
					t.Errorf("returned identifier %q not found in records", result)
				}
			}
		})
	}
}

func TestSelectableStorer_Select_Empty(t *testing.T) {
	mock := &mockSelectableStorer{records: map[Identifier]*mockSelectableSpec{}}
	ss := NewSelectableStorer(mock)

	result := ss.Select(1)
	testutil.AssertEqual(t, "result", result, Identifier(""))
}

func TestSelectableStorer_Build(t *testing.T) {
	tests := map[string]struct {
		records       map[Identifier]*mockSelectableSpec
		expOutputRows int
	}{
		"empty produces default rows": {
			records:       map[Identifier]*mockSelectableSpec{},
			expOutputRows: defaultSelectorRowCount,
		},
		"few items produces default rows": {
			records: map[Identifier]*mockSelectableSpec{
				"a": {name: "A", valid: true},
				"b": {name: "B", valid: true},
			},
			expOutputRows: defaultSelectorRowCount,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mock := &mockSelectableStorer{records: tt.records}
			ss := NewSelectableStorer(mock)

			testutil.AssertEqual(t, "output rows", len(ss.output), tt.expOutputRows)
		})
	}
}

func TestSelectableStorer_Prompt(t *testing.T) {
	records := map[Identifier]*mockSelectableSpec{
		"item-a": {name: "Alpha", valid: true},
		"item-b": {name: "Beta", valid: true},
	}
	mock := &mockSelectableStorer{records: records}
	ss := NewSelectableStorer(mock)

	tests := map[string]struct {
		input    string
		expErr   bool
		expEmpty bool
	}{
		"valid selection 1": {
			input:    "1\n",
			expErr:   false,
			expEmpty: false,
		},
		"valid selection 2": {
			input:    "2\n",
			expErr:   false,
			expEmpty: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rw := &mockReadWriter{
				readBuf:  bytes.NewBufferString(tt.input),
				writeBuf: &bytes.Buffer{},
			}

			result, err := ss.Prompt(rw, "Select an item:")

			if tt.expErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.expEmpty {
				testutil.AssertEqual(t, "result", result, Identifier(""))
			} else {
				if result == "" {
					t.Errorf("expected non-empty identifier")
				}
				if _, ok := records[result]; !ok {
					t.Errorf("returned identifier %q not in records", result)
				}
			}

			// Verify prompt was written
			if rw.writeBuf.Len() == 0 {
				t.Errorf("expected output to be written")
			}
		})
	}
}

func TestSelectableStorer_Prompt_InvalidThenValid(t *testing.T) {
	records := map[Identifier]*mockSelectableSpec{
		"item-a": {name: "Alpha", valid: true},
	}
	mock := &mockSelectableStorer{records: records}
	ss := NewSelectableStorer(mock)

	// First invalid input, then valid
	rw := &mockReadWriter{
		readBuf:  bytes.NewBufferString("invalid\n1\n"),
		writeBuf: &bytes.Buffer{},
	}

	result, err := ss.Prompt(rw, "Select:")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result == "" {
		t.Errorf("expected non-empty identifier")
	}
}
