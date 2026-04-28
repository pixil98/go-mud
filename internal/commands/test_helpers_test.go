package commands

import (
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func newTestZone(id string) (*game.ZoneInstance, error) {
	zone := &assets.Zone{ResetMode: assets.ZoneResetNever}
	return game.NewZoneInstance(storage.NewResolvedSmartIdentifier(id, zone), nil)
}

func newTestRoom(id, name, zoneId string) (*game.RoomInstance, error) {
	zone := &assets.Zone{ResetMode: assets.ZoneResetNever}
	room := &assets.Room{
		Name: name,
		Zone: storage.NewResolvedSmartIdentifier(zoneId, zone),
	}
	return game.NewRoomInstance(storage.NewResolvedSmartIdentifier(id, room))
}

// newTestPlayer creates a CharacterInstance and adds it to the given room.
func newTestPlayer(charId, name string, room *game.RoomInstance) *game.CharacterInstance {
	ci, _ := newRecordingPlayer(charId, name, room)
	return ci
}

// newRecordingPlayer creates a CharacterInstance with a buffered msgs channel
// and adds it to the given room. The returned channel can be drained to inspect
// what was published to this player.
func newRecordingPlayer(charId, name string, room *game.RoomInstance) (*game.CharacterInstance, chan []byte) {
	msgs := make(chan []byte, 10)
	charRef := storage.NewResolvedSmartIdentifier(charId, &assets.Character{Name: name})
	ci, _ := game.NewCharacterInstance(charRef, msgs, room)
	room.AddPlayer(charId, ci)
	return ci, msgs
}

// newCombatMob creates a mob with enough HP to be alive for combat tests.
// Use when code calls room.GetMob() or needs a real MobileInstance.
func newCombatMob(instanceId, name string) *game.MobileInstance {
	hpPerks := []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: assets.BuildKey(assets.ResourcePrefix, assets.ResourceHp, assets.ResourceAspectMax), Value: 100},
	}
	mobRef := storage.NewResolvedSmartIdentifier(instanceId+"-spec", &assets.Mobile{
		ShortDesc: name,
		Perks:     hpPerks,
	})
	mi, err := game.NewMobileInstance(mobRef)
	if err != nil {
		panic(err)
	}
	mi.InstanceId = instanceId
	mi.SetResource(assets.ResourceHp, 100)
	return mi
}
