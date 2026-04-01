package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestObjectInstance_ActivateDecay(t *testing.T) {
	tests := map[string]struct {
		lifetime      int
		callTwice     bool
		wantRemaining int
	}{
		"permanent item is not activated": {
			lifetime:      0,
			wantRemaining: 0,
		},
		"decayable item is activated": {
			lifetime:      10,
			wantRemaining: 10,
		},
		"second call is a no-op": {
			lifetime:      10,
			callTwice:     true,
			wantRemaining: 5,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oi := newTestObj("a", tc.lifetime)
			oi.ActivateDecay()
			if tc.callTwice {
				oi.RemainingTicks = 5
				oi.ActivateDecay()
			}
			if oi.RemainingTicks != tc.wantRemaining {
				t.Errorf("RemainingTicks = %d, want %d", oi.RemainingTicks, tc.wantRemaining)
			}
		})
	}
}

func TestObjectInstance_Tick(t *testing.T) {
	tests := map[string]struct {
		lifetime      int
		activate      bool
		ticks         int
		wantRemaining int
	}{
		"permanent item stays at zero": {
			lifetime:      0,
			ticks:         3,
			wantRemaining: 0,
		},
		"inactive decayable item is not decremented": {
			lifetime:      5,
			activate:      false,
			ticks:         3,
			wantRemaining: 0,
		},
		"active item decrements each tick": {
			lifetime:      5,
			activate:      true,
			ticks:         3,
			wantRemaining: 2,
		},
		"item reaches zero": {
			lifetime:      2,
			activate:      true,
			ticks:         2,
			wantRemaining: 0,
		},
		"item does not go negative": {
			lifetime:      1,
			activate:      true,
			ticks:         3,
			wantRemaining: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oi := newTestObj("a", tc.lifetime)
			if tc.activate {
				oi.ActivateDecay()
			}
			for range tc.ticks {
				oi.Tick()
			}
			if oi.RemainingTicks != tc.wantRemaining {
				t.Errorf("RemainingTicks = %d, want %d", oi.RemainingTicks, tc.wantRemaining)
			}
		})
	}
}

func TestObjectInstance_Tick_RecursesContents(t *testing.T) {
	container := storage.NewResolvedSmartIdentifier("test-container", &assets.Object{
		Aliases:   []string{"a"},
		ShortDesc: "a container",
		Flags:     []string{"container"},
	})
	oi, _ := NewObjectInstance(container)

	inner := newTestObj("b", 3)
	inner.ActivateDecay()
	oi.Contents.AddObj(inner)

	oi.Tick()

	if inner.RemainingTicks != 2 {
		t.Errorf("inner RemainingTicks = %d, want 2", inner.RemainingTicks)
	}
}

func TestInventory_Tick(t *testing.T) {
	tests := map[string]struct {
		items     []int // lifetime per item (0 = permanent)
		activate  bool
		ticks     int
		wantCount int
	}{
		"permanent items are not removed": {
			items:     []int{0, 0},
			ticks:     10,
			wantCount: 2,
		},
		"inactive decayable items are not removed": {
			items:     []int{5},
			activate:  false,
			ticks:     10,
			wantCount: 1,
		},
		"expired item is removed": {
			items:     []int{2},
			activate:  true,
			ticks:     2,
			wantCount: 0,
		},
		"mixed permanent and decayable": {
			items:     []int{0, 3, 0, 1},
			activate:  true,
			ticks:     2,
			wantCount: 3,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inv := NewInventory()
			for i, lt := range tc.items {
				oi := newTestObj(string(rune('a'+i)), lt)
				if tc.activate {
					oi.ActivateDecay()
				}
				inv.AddObj(oi)
			}
			for range tc.ticks {
				inv.Tick()
			}
			if inv.Len() != tc.wantCount {
				t.Errorf("Len() = %d, want %d", inv.Len(), tc.wantCount)
			}
		})
	}
}

func TestEquipment_Tick(t *testing.T) {
	tests := map[string]struct {
		items     []int // lifetime per item
		activate  bool
		ticks     int
		wantCount int
	}{
		"permanent items survive": {
			items:     []int{0, 0},
			ticks:     10,
			wantCount: 2,
		},
		"expired item is removed": {
			items:     []int{2},
			activate:  true,
			ticks:     2,
			wantCount: 0,
		},
		"mixed items": {
			items:     []int{0, 1},
			activate:  true,
			ticks:     1,
			wantCount: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eq := NewEquipment()
			for i, lt := range tc.items {
				obj := storage.NewResolvedSmartIdentifier(string(rune('a'+i)), &assets.Object{
					Aliases:   []string{"a"},
					ShortDesc: "a wearable",
					Flags:     []string{"wearable"},
					WearSlots: []string{"test"},
					Lifetime:  lt,
				})
				oi, _ := NewObjectInstance(obj)
				if tc.activate {
					oi.ActivateDecay()
				}
				eq.equip("test", oi)
			}
			for range tc.ticks {
				eq.Tick()
			}
			if eq.Len() != tc.wantCount {
				t.Errorf("Len() = %d, want %d", eq.Len(), tc.wantCount)
			}
		})
	}
}
