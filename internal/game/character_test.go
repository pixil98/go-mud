package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func newTestCharacterInstance() *CharacterInstance {
	char := storage.NewResolvedSmartIdentifier("test-char", &assets.Character{Name: "Tester"})
	ci, _ := NewCharacterInstance(char, make(chan []byte, 1), "test-zone", "test-room")
	return ci
}

func TestCharacterInstance_SpendAP(t *testing.T) {
	tests := map[string]struct {
		startAP int
		spend   int
		wantOk  bool
		wantAP  int
	}{
		"spend within budget": {
			startAP: 3,
			spend:   2,
			wantOk:  true,
			wantAP:  1,
		},
		"spend exact budget": {
			startAP: 2,
			spend:   2,
			wantOk:  true,
			wantAP:  0,
		},
		"spend exceeds budget": {
			startAP: 1,
			spend:   2,
			wantOk:  false,
			wantAP:  1,
		},
		"spend from empty": {
			startAP: 0,
			spend:   1,
			wantOk:  false,
			wantAP:  0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			ci.currentAP = tc.startAP

			got := ci.SpendAP(tc.spend)
			if got != tc.wantOk {
				t.Errorf("SpendAP(%d) = %v, want %v", tc.spend, got, tc.wantOk)
			}
			if ci.currentAP != tc.wantAP {
				t.Errorf("currentAP after SpendAP = %d, want %d", ci.currentAP, tc.wantAP)
			}
		})
	}
}

func TestCharacterInstance_ResetAP(t *testing.T) {
	tests := map[string]struct {
		perks  []assets.Perk
		wantAP int
	}{
		"no perk gives zero": {
			wantAP: 0,
		},
		"perk sets max ap": {
			perks:  []assets.Perk{{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 3}},
			wantAP: 3,
		},
		"perk value 1 gives 1": {
			perks:  []assets.Perk{{Type: assets.PerkTypeModifier, Key: assets.PerkKeyActionPointsMax, Value: 1}},
			wantAP: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ci := newTestCharacterInstance()
			ci.PerkCache.SetOwn(tc.perks)
			ci.currentAP = 0 // exhaust AP before reset to confirm it restores

			ci.ResetAP()

			if ci.currentAP != tc.wantAP {
				t.Errorf("currentAP after ResetAP = %d, want %d", ci.currentAP, tc.wantAP)
			}
		})
	}
}
