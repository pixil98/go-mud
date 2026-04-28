package assets

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/storage"
)

func TestRoom_HasFlag(t *testing.T) {
	tests := map[string]struct {
		flags []RoomFlag
		check RoomFlag
		want  bool
	}{
		"has death flag": {
			flags: []RoomFlag{RoomFlagDeath},
			check: RoomFlagDeath,
			want:  true,
		},
		"does not have flag": {
			flags: []RoomFlag{RoomFlagDeath},
			check: RoomFlagNoMob,
			want:  false,
		},
		"empty flags": {
			flags: nil,
			check: RoomFlagDeath,
			want:  false,
		},
		"multiple flags": {
			flags: []RoomFlag{RoomFlagNoMob, RoomFlagSingleOccupant},
			check: RoomFlagSingleOccupant,
			want:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := &Room{Flags: tc.flags}
			if got := r.HasFlag(tc.check); got != tc.want {
				t.Errorf("HasFlag(%q) = %v, want %v", tc.check, got, tc.want)
			}
		})
	}
}

func TestRoom_Validate_Flags(t *testing.T) {
	base := func() Room {
		return Room{
			Name: "test-room",
			Zone: storage.NewSmartIdentifier[*Zone]("test-zone"),
		}
	}

	tests := map[string]struct {
		flags  []RoomFlag
		expErr string
	}{
		"valid flags": {
			flags: []RoomFlag{RoomFlagDeath, RoomFlagNoMob, RoomFlagSingleOccupant, RoomFlagDark},
		},
		"unknown flag rejected": {
			flags:  []RoomFlag{"bogus"},
			expErr: `unknown flag "bogus"`,
		},
		"no flags is valid": {
			flags: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := base()
			r.Flags = tc.flags
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
