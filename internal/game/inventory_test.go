package game

import "testing"

func TestInventory_RemoveObj(t *testing.T) {
	tests := map[string]struct {
		wantNil bool
	}{
		"removes existing item and returns it": {wantNil: false},
		"missing id returns nil":               {wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inv := NewInventory()
			oi := newTestObj("a")
			inv.AddObj(oi)

			var removeId string
			if tc.wantNil {
				removeId = "nonexistent-instance-id"
			} else {
				removeId = oi.InstanceId
			}

			got := inv.RemoveObj(removeId)
			if tc.wantNil {
				if got != nil {
					t.Errorf("RemoveObj(%q) = %v, want nil", removeId, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("RemoveObj(%q) = nil, want object", removeId)
			}
			if inv.Len() != 0 {
				t.Errorf("Len() after RemoveObj = %d, want 0", inv.Len())
			}
		})
	}
}

func TestInventory_FindObjs(t *testing.T) {
	tests := map[string]struct {
		matchId   string
		wantCount int
	}{
		"match returns item":     {matchId: "a", wantCount: 1},
		"no match returns empty": {matchId: "z", wantCount: 0},
		"match all":              {matchId: "all", wantCount: 2},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inv := NewInventory()
			inv.AddObj(newTestObj("a"))
			inv.AddObj(newTestObj("b"))

			got := inv.FindObjs(func(oi *ObjectInstance) bool {
				return tc.matchId == "all" || oi.Object.Id() == tc.matchId
			})
			if len(got) != tc.wantCount {
				t.Errorf("FindObjs count = %d, want %d", len(got), tc.wantCount)
			}
		})
	}
}

func TestInventory_FindObjByDef(t *testing.T) {
	tests := map[string]struct {
		defId   string
		wantNil bool
	}{
		"finds by definition id":         {defId: "sword", wantNil: false},
		"missing definition returns nil": {defId: "axe", wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inv := NewInventory()
			inv.AddObj(newTestObj("sword"))

			got := inv.FindObjByDef(tc.defId)
			if tc.wantNil {
				if got != nil {
					t.Errorf("FindObjByDef(%q) = %v, want nil", tc.defId, got)
				}
				return
			}
			if got == nil {
				t.Errorf("FindObjByDef(%q) = nil, want object", tc.defId)
			}
		})
	}
}

func TestInventory_Drain(t *testing.T) {
	tests := map[string]struct {
		itemCount int
	}{
		"drain returns all items and empties inventory": {itemCount: 3},
		"drain of empty inventory returns nil slice":   {itemCount: 0},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			inv := NewInventory()
			for i := 0; i < tc.itemCount; i++ {
				inv.AddObj(newTestObj(string(rune('a' + i))))
			}
			got := inv.Drain()
			if len(got) != tc.itemCount {
				t.Errorf("Drain() returned %d items, want %d", len(got), tc.itemCount)
			}
			if inv.Len() != 0 {
				t.Errorf("Len() after Drain = %d, want 0", inv.Len())
			}
		})
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
			items:    []int{5},
			activate: false,
			ticks:    10,
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
