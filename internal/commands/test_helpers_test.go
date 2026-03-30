package commands

import (
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// mockZoneLocator is a test double for ZoneLocator.
type mockZoneLocator struct {
	zones map[string]*game.ZoneInstance
}

func (m *mockZoneLocator) GetZone(zoneId string) *game.ZoneInstance {
	if m.zones == nil {
		return nil
	}
	return m.zones[zoneId]
}

// mockBuffWorld is a test double for buffWorld (ZoneLocator + Perks()).
type mockBuffWorld struct {
	zones     map[string]*game.ZoneInstance
	perkCache *game.PerkCache
}

func (m *mockBuffWorld) GetZone(zoneId string) *game.ZoneInstance {
	if m.zones == nil {
		return nil
	}
	return m.zones[zoneId]
}

func (m *mockBuffWorld) Perks() *game.PerkCache {
	return m.perkCache
}

func newTestMobInstance(instanceId, name string, perks []assets.Perk) *game.MobileInstance {
	return &game.MobileInstance{
		Mobile: storage.NewResolvedSmartIdentifier(instanceId+"-spec", &assets.Mobile{ShortDesc: name}),
		ActorInstance: game.ActorInstance{
			InstanceId: instanceId,
			PerkCache:  *game.NewPerkCache(perks, nil),
		},
	}
}

func newTestZone(id string) (*game.ZoneInstance, error) {
	zone := &assets.Zone{ResetMode: assets.ZoneResetNever}
	return game.NewZoneInstance(storage.NewResolvedSmartIdentifier(id, zone))
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
// Use only where concrete CharacterInstance is unavoidable (e.g. room infrastructure tests).
func newTestPlayer(charId, name string, room *game.RoomInstance) *game.CharacterInstance {
	msgs := make(chan []byte, 10)
	charRef := storage.NewResolvedSmartIdentifier(charId, &assets.Character{Name: name})
	ps, _ := game.NewCharacterInstance(charRef, msgs, room.Room.Get().Zone.Id(), room.Room.Id())
	room.AddPlayer(charId, ps)
	return ps
}

// newTestPlayerWithMsgs creates a CharacterInstance and returns it with its msgs channel
// so tests can assert on messages delivered via Notify.
func newTestPlayerWithMsgs(charId, name string, room *game.RoomInstance) (*game.CharacterInstance, chan []byte) {
	msgs := make(chan []byte, 10)
	charRef := storage.NewResolvedSmartIdentifier(charId, &assets.Character{Name: name})
	ps, _ := game.NewCharacterInstance(charRef, msgs, room.Room.Get().Zone.Id(), room.Room.Id())
	room.AddPlayer(charId, ps)
	return ps, msgs
}

// newCharacterInstance creates a CharacterInstance in a throwaway room.
// Use only where concrete CharacterInstance is unavoidable (e.g. room infrastructure tests).
func newCharacterInstance(charId, name string) *game.CharacterInstance {
	room, _ := newTestRoom(charId+"-room", "Room", charId+"-zone")
	return newTestPlayer(charId, name, room)
}
