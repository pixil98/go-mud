package assets

import (
	"strings"
	"testing"
)

func TestPerk_validate(t *testing.T) {
	tests := map[string]struct {
		perk   Perk
		expErr string
	}{
		"valid modifier": {
			perk: Perk{Type: PerkTypeModifier, Key: "core.stats.str", Value: 2},
		},
		"valid grant no arg": {
			perk: Perk{Type: PerkTypeGrant, Key: PerkGrantUnlockAbility, Arg: "fireball"},
		},
		"valid grant with arg": {
			perk: Perk{Type: PerkTypeGrant, Key: PerkGrantAttack, Arg: "2d6"},
		},
		"missing type": {
			perk:   Perk{Key: "core.stats.str", Value: 1},
			expErr: "type is required",
		},
		"unknown type": {
			perk:   Perk{Type: "tag", Key: "something"},
			expErr: "unknown perk type",
		},
		"modifier missing key": {
			perk:   Perk{Type: PerkTypeModifier, Value: 1},
			expErr: "modifier perk requires key",
		},
		"modifier zero value": {
			perk:   Perk{Type: PerkTypeModifier, Key: "core.stats.str"},
			expErr: "modifier perk requires non-zero value",
		},
		"grant missing key": {
			perk:   Perk{Type: PerkTypeGrant},
			expErr: "grant perk requires key",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.perk.validate()
			if tc.expErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tc.expErr)
				return
			}
			if !strings.Contains(err.Error(), tc.expErr) {
				t.Errorf("expected error containing %q, got %q", tc.expErr, err.Error())
			}
		})
	}
}
