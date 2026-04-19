package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestEquipment_RemoveObj(t *testing.T) {
	tests := map[string]struct {
		wantNil bool
	}{
		"removes existing item":  {wantNil: false},
		"missing id returns nil": {wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eq := NewEquipment()
			oi := newTestObj("helm")
			eq.equip("head", oi)

			var removeId string
			if tc.wantNil {
				removeId = "nonexistent-instance-id"
			} else {
				removeId = oi.InstanceId
			}

			got := eq.RemoveObj(removeId)
			if tc.wantNil {
				if got != nil {
					t.Errorf("RemoveObj(%q) = %v, want nil", removeId, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("RemoveObj(%q) = nil, want object", removeId)
			}
			if eq.Len() != 0 {
				t.Errorf("Len() after RemoveObj = %d, want 0", eq.Len())
			}
		})
	}
}

func TestEquipment_SlotCount(t *testing.T) {
	tests := map[string]struct {
		equip     map[string]int // slot → item count
		query     string
		wantCount int
	}{
		"empty equipment returns zero":   {query: "finger", wantCount: 0},
		"slot not equipped returns zero": {equip: map[string]int{"head": 1}, query: "finger", wantCount: 0},
		"one item in slot":               {equip: map[string]int{"finger": 1}, query: "finger", wantCount: 1},
		"two items in same slot":         {equip: map[string]int{"finger": 2}, query: "finger", wantCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eq := NewEquipment()
			idx := 0
			for slot, count := range tc.equip {
				for i := 0; i < count; i++ {
					eq.equip(slot, newTestObj(string(rune('a'+idx))))
					idx++
				}
			}
			if got := eq.SlotCount(tc.query); got != tc.wantCount {
				t.Errorf("SlotCount(%q) = %d, want %d", tc.query, got, tc.wantCount)
			}
		})
	}
}

func TestEquipment_FindObjs(t *testing.T) {
	tests := map[string]struct {
		matchId   string
		wantCount int
	}{
		"match returns item":     {matchId: "helm", wantCount: 1},
		"no match returns empty": {matchId: "boots", wantCount: 0},
		"match all":              {matchId: "all", wantCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eq := NewEquipment()
			eq.equip("head", newTestObj("helm"))
			eq.equip("finger", newTestObj("ring"))

			got := eq.FindObjs(func(oi *ObjectInstance) bool {
				return tc.matchId == "all" || oi.Object.Id() == tc.matchId
			})
			if len(got) != tc.wantCount {
				t.Errorf("FindObjs count = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

func TestEquipment_ForEachSlot(t *testing.T) {
	tests := map[string]struct {
		slots     []string
		wantCount int
	}{
		"empty yields no iterations": {slots: nil, wantCount: 0},
		"two items yields two calls": {slots: []string{"head", "finger"}, wantCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eq := NewEquipment()
			for i, slot := range tc.slots {
				eq.equip(slot, newTestObj(string(rune('a'+i))))
			}
			count := 0
			eq.ForEachSlot(func(EquipSlot) { count++ })
			if count != tc.wantCount {
				t.Errorf("ForEachSlot called %d times, want %d", count, tc.wantCount)
			}
		})
	}
}

func TestEquipment_Drain(t *testing.T) {
	tests := map[string]struct {
		itemCount int
	}{
		"drain returns all items and empties equipment": {itemCount: 2},
		"drain of empty equipment returns nil":         {itemCount: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			eq := NewEquipment()
			for i := 0; i < tc.itemCount; i++ {
				eq.equip("slot", newTestObj(string(rune('a'+i))))
			}
			got := eq.Drain()
			if len(got) != tc.itemCount {
				t.Errorf("Drain() returned %d items, want %d", len(got), tc.itemCount)
			}
			if eq.Len() != 0 {
				t.Errorf("Len() after Drain = %d, want 0", eq.Len())
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
