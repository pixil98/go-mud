package storage

import (
	"testing"

	"github.com/pixil98/go-testutil"
)

func TestExtensionState_Set(t *testing.T) {
	tests := map[string]struct {
		initial ExtensionState
		key     string
		value   any
		expErr  bool
	}{
		"set on nil map": {
			initial: nil,
			key:     "test",
			value:   map[string]string{"foo": "bar"},
			expErr:  false,
		},
		"set on existing map": {
			initial: ExtensionState{},
			key:     "test",
			value:   map[string]int{"count": 42},
			expErr:  false,
		},
		"set string value": {
			initial: ExtensionState{},
			key:     "name",
			value:   "hello",
			expErr:  false,
		},
		"set struct value": {
			initial: ExtensionState{},
			key:     "data",
			value:   struct{ Name string }{"test"},
			expErr:  false,
		},
		"marshal error with channel": {
			initial: ExtensionState{},
			key:     "bad",
			value:   make(chan int),
			expErr:  true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.initial
			err := e.Set(tt.key, tt.value)

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

			if e == nil {
				t.Errorf("map should not be nil after Set")
				return
			}

			if _, ok := e[tt.key]; !ok {
				t.Errorf("key %q not found after Set", tt.key)
			}
		})
	}
}

func TestExtensionState_Get(t *testing.T) {
	type testData struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	preloaded := ExtensionState{}
	if err := preloaded.Set("data", testData{Name: "test", Count: 5}); err != nil {
		t.Fatalf("failed to set preloaded data: %v", err)
	}
	if err := preloaded.Set("string", "hello"); err != nil {
		t.Fatalf("failed to set preloaded string: %v", err)
	}

	tests := map[string]struct {
		state    ExtensionState
		key      string
		expFound bool
		expErr   bool
		expValue any
	}{
		"get from nil map": {
			state:    nil,
			key:      "anything",
			expFound: false,
			expErr:   false,
		},
		"get missing key": {
			state:    preloaded,
			key:      "nonexistent",
			expFound: false,
			expErr:   false,
		},
		"get existing struct": {
			state:    preloaded,
			key:      "data",
			expFound: true,
			expErr:   false,
			expValue: testData{Name: "test", Count: 5},
		},
		"get existing string": {
			state:    preloaded,
			key:      "string",
			expFound: true,
			expErr:   false,
			expValue: "hello",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			switch exp := tt.expValue.(type) {
			case testData:
				var v testData
				found, err := tt.state.Get(tt.key, &v)
				checkGetResult(t, found, err, tt.expFound, tt.expErr)
				if tt.expFound && !tt.expErr {
					testutil.AssertEqual(t, "value", v, exp)
				}
			case string:
				var v string
				found, err := tt.state.Get(tt.key, &v)
				checkGetResult(t, found, err, tt.expFound, tt.expErr)
				if tt.expFound && !tt.expErr {
					testutil.AssertEqual(t, "value", v, exp)
				}
			default:
				var v any
				found, err := tt.state.Get(tt.key, &v)
				checkGetResult(t, found, err, tt.expFound, tt.expErr)
			}
		})
	}
}

func TestExtensionState_Get_UnmarshalError(t *testing.T) {
	e := ExtensionState{
		"bad": []byte(`{"invalid json`),
	}

	var out map[string]string
	found, err := e.Get("bad", &out)

	testutil.AssertEqual(t, "found", found, true)
	testutil.AssertErrorContains(t, err, "unmarshal extension")
}

func checkGetResult(t *testing.T, found bool, err error, expFound bool, expErr bool) {
	t.Helper()

	if expErr {
		if err == nil {
			t.Errorf("expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	testutil.AssertEqual(t, "found", found, expFound)
}

func TestExtensionState_Delete(t *testing.T) {
	tests := map[string]struct {
		initial ExtensionState
		key     string
	}{
		"delete from nil map": {
			initial: nil,
			key:     "anything",
		},
		"delete missing key": {
			initial: ExtensionState{"other": []byte(`"value"`)},
			key:     "nonexistent",
		},
		"delete existing key": {
			initial: ExtensionState{"target": []byte(`"value"`), "other": []byte(`"keep"`)},
			key:     "target",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := tt.initial
			e.Delete(tt.key)

			if e != nil {
				if _, ok := e[tt.key]; ok {
					t.Errorf("key %q should have been deleted", tt.key)
				}
			}
		})
	}
}
