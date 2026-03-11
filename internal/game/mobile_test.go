package game

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func newTestMob(level int) *MobileInstance {
	mob := storage.NewResolvedSmartIdentifier("test-mob", &assets.Mobile{
		ShortDesc: "a test mob",
		Level:     level,
	})
	eq := NewEquipment()
	mi := &MobileInstance{
		Mobile: mob,
		ActorInstance: ActorInstance{
			InstanceId: "test-instance",
			inventory:  NewInventory(),
			equipment: eq,
			level:     level,
			PerkCache: *NewPerkCache(nil, map[string]PerkSource{"equipment": eq}),
		},
	}
	return mi
}

func newTestObj(id string) *ObjectInstance {
	obj := storage.NewResolvedSmartIdentifier(id, &assets.Object{
		Aliases:   []string{id},
		ShortDesc: id,
	})
	oi, _ := NewObjectInstance(obj)
	return oi
}

func TestNewCorpse(t *testing.T) {
	tests := map[string]struct {
		inventoryItems []string
		equippedItems  []string
		wantItemCount  int
	}{
		"empty mob yields empty corpse": {
			wantItemCount: 0,
		},
		"inventory items move to corpse": {
			inventoryItems: []string{"sword", "potion"},
			wantItemCount:  2,
		},
		"equipped items move to corpse": {
			equippedItems: []string{"helm", "boots"},
			wantItemCount: 2,
		},
		"both inventory and equipped items": {
			inventoryItems: []string{"coin"},
			equippedItems:  []string{"ring"},
			wantItemCount:  2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mi := newTestMob(5)
			for _, id := range tc.inventoryItems {
				mi.inventory.AddObj(newTestObj(id))
			}
			for _, id := range tc.equippedItems {
				_ = mi.equipment.Equip("slot", 0, newTestObj(id))
			}

			corpse := newCorpse(mi)

			if corpse == nil {
				t.Fatal("newCorpse returned nil")
			}
			if corpse.Contents == nil {
				t.Fatal("corpse Contents is nil")
			}
			if !strings.Contains(corpse.Object.Get().ShortDesc, "test mob") {
				t.Errorf("ShortDesc %q does not contain mob name", corpse.Object.Get().ShortDesc)
			}
			if !corpse.Object.Get().HasFlag(assets.ObjectFlagContainer) {
				t.Error("corpse does not have container flag")
			}

			var count int
			corpse.Contents.ForEachObj(func(_ string, _ *ObjectInstance) { count++ })
			if count != tc.wantItemCount {
				t.Errorf("corpse item count = %d, want %d", count, tc.wantItemCount)
			}

			// Mob's inventory and equipment should be drained.
			if mi.inventory.Len() != 0 {
				t.Errorf("mob inventory not drained: %d items remain", mi.inventory.Len())
			}
			if mi.equipment.Len() != 0 {
				t.Errorf("mob equipment not drained: %d items remain", mi.equipment.Len())
			}
		})
	}
}

func TestMobileInstance_OnDeath(t *testing.T) {
	// Build a room with the mob in it.
	room := storage.NewResolvedSmartIdentifier("test-room", &assets.Room{
		Name: "Test Room",
	})
	ri, _ := NewRoomInstance(room)

	mi := newTestMob(3)
	mi.inventory.AddObj(newTestObj("coin"))
	ri.mobiles[mi.Id()] = mi

	drops := mi.OnDeath()

	if len(drops) != 1 {
		t.Fatalf("OnDeath returned %d drops, want 1", len(drops))
	}
	corpse := drops[0].(*ObjectInstance)
	if corpse == nil {
		t.Fatal("drop is nil")
	}
	var itemCount int
	corpse.Contents.ForEachObj(func(_ string, _ *ObjectInstance) { itemCount++ })
	if itemCount != 1 {
		t.Errorf("corpse has %d items, want 1", itemCount)
	}
}
