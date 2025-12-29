package storage

import (
	"encoding/json"
	"fmt"
)

type ExtensionState map[string]json.RawMessage

// Set stores v under key after marshalling it to JSON.
func (e *ExtensionState) Set(k string, v any) error {
	if *e == nil {
		*e = ExtensionState{}
	}

	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal extension %q: %w", k, err)
	}

	(*e)[k] = json.RawMessage(b)
	return nil
}

// Get unmarshals the extension value at key into out.
// Returns (found=false, nil) if not present.
func (e ExtensionState) Get(key string, out any) (bool, error) {
	if e == nil {
		return false, nil
	}

	raw, ok := e[key]
	if !ok || len(raw) == 0 {
		return false, nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return true, fmt.Errorf("unmarshal extension %q: %w", key, err)
	}
	return true, nil
}

// Delete removes the extension key, if present.
func (e ExtensionState) Delete(key string) {
	if e == nil {
		return
	}
	delete(e, key)
}
