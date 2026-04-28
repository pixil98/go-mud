package assets

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/storage"
)

func TestRoom_Validate_ExtraDescs(t *testing.T) {
	base := func() Room {
		return Room{
			Name: "test-room",
			Zone: storage.NewSmartIdentifier[*Zone]("test-zone"),
		}
	}

	tests := map[string]struct {
		descs  []ExtraDesc
		expErr string
	}{
		"valid extra desc": {
			descs: []ExtraDesc{{Keywords: []string{"a"}, Description: "test"}},
		},
		"missing keyword": {
			descs:  []ExtraDesc{{Description: "test"}},
			expErr: "extra_descs[0]: at least one keyword is required",
		},
		"missing description": {
			descs:  []ExtraDesc{{Keywords: []string{"a"}}},
			expErr: "extra_descs[0]: description is required",
		},
		"no extra descs is valid": {
			descs: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := base()
			r.ExtraDescs = tc.descs
			err := r.Validate()
			if tc.expErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.expErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.expErr)
			}
		})
	}
}
