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

func newTestMobInstance(instanceId, name string) *game.MobileInstance {
	return &game.MobileInstance{
		InstanceId: instanceId,
		Mobile:     storage.NewResolvedSmartIdentifier(instanceId+"-spec", &assets.Mobile{ShortDesc: name}),
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
