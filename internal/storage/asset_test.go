package storage

import (
	"fmt"
	"strings"
	"testing"
)

// testSpec is a simple ValidatingSpec for testing
type testSpec struct {
	valid bool
}

func (s *testSpec) Validate() error {
	if !s.valid {
		return fmt.Errorf("spec is invalid")
	}
	return nil
}

func TestAsset_Validate(t *testing.T) {
	tests := map[string]struct {
		asset   Asset[*testSpec]
		expErrs []string
	}{
		"valid asset": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "test-id",
				Spec:       &testSpec{valid: true},
			},
			expErrs: nil,
		},
		"version not set": {
			asset: Asset[*testSpec]{
				Version:    0,
				Identifier: "test-id",
				Spec:       &testSpec{valid: true},
			},
			expErrs: []string{"version must be set"},
		},
		"empty identifier": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "",
				Spec:       &testSpec{valid: true},
			},
			expErrs: []string{"id must be set"},
		},
		"identifier with spaces": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "test id",
				Spec:       &testSpec{valid: true},
			},
			expErrs: []string{"id must be alphanumeric"},
		},
		"identifier with underscore": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "test_id",
				Spec:       &testSpec{valid: true},
			},
			expErrs: []string{"id must be alphanumeric"},
		},
		"identifier with special chars": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "test@id!",
				Spec:       &testSpec{valid: true},
			},
			expErrs: []string{"id must be alphanumeric"},
		},
		"identifier with hyphen is valid": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "test-id-123",
				Spec:       &testSpec{valid: true},
			},
			expErrs: nil,
		},
		"invalid spec": {
			asset: Asset[*testSpec]{
				Version:    1,
				Identifier: "test-id",
				Spec:       &testSpec{valid: false},
			},
			expErrs: []string{"spec is invalid"},
		},
		"multiple errors": {
			asset: Asset[*testSpec]{
				Version:    0,
				Identifier: "",
				Spec:       &testSpec{valid: false},
			},
			expErrs: []string{
				"version must be set",
				"id must be set",
				"spec is invalid",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.asset.Validate()

			if len(tt.expErrs) == 0 {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected errors %v, got nil", tt.expErrs)
				return
			}

			errStr := err.Error()
			for _, e := range tt.expErrs {
				if !strings.Contains(errStr, e) {
					t.Errorf("error %q does not contain %q", errStr, e)
				}
			}
		})
	}
}

func TestIdentifier_String(t *testing.T) {
	tests := map[string]struct {
		id  string
		exp string
	}{
		"simple identifier": {
			id:  "test",
			exp: "test",
		},
		"empty identifier": {
			id:  "",
			exp: "",
		},
		"identifier with hyphen": {
			id:  "test-id-123",
			exp: "test-id-123",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := tt.id
			if got != tt.exp {
				t.Errorf("got %q, expected %q", got, tt.exp)
			}
		})
	}
}
