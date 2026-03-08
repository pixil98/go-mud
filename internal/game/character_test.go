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

func TestCharacterInstance_OnDeath(t *testing.T) {
	tests := map[string]struct {
		startHP    int
		maxHP      int
		wantMsg    bool
		wantQuit   bool
		wantDone   bool
		wantFullHP bool
	}{
		"death sends message, sets quit, closes done, restores HP": {
			startHP:    0,
			maxHP:      20,
			wantMsg:    true,
			wantQuit:   true,
			wantDone:   true,
			wantFullHP: true,
		},
		"death with nil msgs channel does not panic": {
			startHP:  0,
			maxHP:    20,
			wantQuit: true,
			wantDone: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hpMaxPerk := assets.Perk{
				Type:  assets.PerkTypeModifier,
				Key:   assets.ResourceKey(assets.ResourceHp, assets.ResourceAspectMax),
				Value: tc.maxHP,
			}
			char := storage.NewResolvedSmartIdentifier("test-char", &assets.Character{Name: "Tester"})
			var msgs chan []byte
			if tc.wantMsg {
				msgs = make(chan []byte, 1)
			}
			ci, _ := NewCharacterInstance(char, msgs, "test-zone", "test-room")
			ci.PerkCache.SetOwn([]assets.Perk{hpMaxPerk})
			ci.initResources()
			ci.setResourceCurrent(assets.ResourceHp, tc.startHP)

			ci.OnDeath()

			if tc.wantMsg {
				select {
				case msg := <-msgs:
					if len(msg) == 0 {
						t.Error("expected non-empty death message")
					}
				default:
					t.Error("expected message in channel, got none")
				}
			}
			if got := ci.IsQuit(); got != tc.wantQuit {
				t.Errorf("IsQuit() = %v, want %v", got, tc.wantQuit)
			}
			select {
			case <-ci.Done():
				if !tc.wantDone {
					t.Error("done channel closed but should not be")
				}
			default:
				if tc.wantDone {
					t.Error("done channel not closed but should be")
				}
			}
			if tc.wantFullHP {
				cur, mx := ci.Resource(assets.ResourceHp)
				if cur != mx {
					t.Errorf("HP after OnDeath = %d, want %d (max)", cur, mx)
				}
			}
		})
	}
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
