package commands

import (
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
	"github.com/pixil98/go-mud/internal/storage"
)

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
// Use only where concrete CharacterInstance is unavoidable (e.g. room infrastructure tests).
func newTestPlayer(charId, name string, room *game.RoomInstance) *game.CharacterInstance {
	msgs := make(chan []byte, 10)
	charRef := storage.NewResolvedSmartIdentifier(charId, &assets.Character{Name: name})
	ci, _ := game.NewCharacterInstance(charRef, msgs, room)
	room.AddPlayer(charId, ci)
	return ci
}

// mockActor is a common test double that satisfies shared.Actor,
// game.FollowTarget, and AssistedPlayer.
type mockActor struct {
	id             string
	name           string
	notified       []string
	following      game.FollowTarget
	followers      []game.FollowTarget
	groupedIds     map[string]bool
	inCombat       bool
	combatTargetId string
	grants         map[string]bool
	room           *game.RoomInstance
	zoneId         string
	roomId         string
	moved        bool
	resources    map[string][2]int // name -> {current, max}
	spendAPFails bool              // when true, SpendAP returns false
	spentAP      int               // records cost passed to last SpendAP call
}

var _ shared.Actor = (*mockActor)(nil)
var _ game.FollowTarget = (*mockActor)(nil)
var _ shared.Actor = (*mockActor)(nil)

func (m *mockActor) Id() string                               { return m.id }
func (m *mockActor) Name() string                             { return m.name }
func (m *mockActor) Notify(msg string)                        { m.notified = append(m.notified, msg) }
func (m *mockActor) Room() *game.RoomInstance                  { return m.room }
func (m *mockActor) Location() (string, string)               { return m.zoneId, m.roomId }
func (m *mockActor) IsInCombat() bool                         { return m.inCombat }
func (m *mockActor) IsAlive() bool                            { return true }
func (m *mockActor) Level() int                               { return 1 }
func (m *mockActor) Resource(name string) (int, int) {
	if r, ok := m.resources[name]; ok {
		return r[0], r[1]
	}
	return 0, 0
}
func (m *mockActor) AdjustResource(name string, delta int, _ bool) {
	if r, ok := m.resources[name]; ok {
		r[0] += delta
		m.resources[name] = r
	}
}
func (m *mockActor) SpendAP(cost int) bool { m.spentAP = cost; return !m.spendAPFails }
func (m *mockActor) HasGrant(key, _ string) bool              { return m.grants[key] }
func (m *mockActor) ModifierValue(string) int                 { return 0 }
func (m *mockActor) GrantArgs(string) []string                { return nil }
func (m *mockActor) AddTimedPerks(string, []assets.Perk, int) {}
func (m *mockActor) SetInCombat(bool)                         {}
func (m *mockActor) CombatTargetId() string                   { return m.combatTargetId }
func (m *mockActor) SetCombatTargetId(string)                 {}
func (m *mockActor) OnDeath() []*game.ObjectInstance          { return nil }
func (m *mockActor) IsCharacter() bool                        { return true }
func (m *mockActor) Inventory() *game.Inventory               { return nil }
func (m *mockActor) Following() game.FollowTarget             { return m.following }
func (m *mockActor) SetFollowing(ft game.FollowTarget)        { m.following = ft }
func (m *mockActor) Followers() []game.FollowTarget           { return m.followers }
func (m *mockActor) AddFollower(ft game.FollowTarget)         { m.followers = append(m.followers, ft) }
func (m *mockActor) RemoveFollower(string)                    {}
func (m *mockActor) Equipment() *game.Equipment                { return nil }

func (m *mockActor) SetFollowerGrouped(id string, grouped bool) {
	if m.groupedIds == nil {
		m.groupedIds = make(map[string]bool)
	}
	if grouped {
		m.groupedIds[id] = true
	} else {
		delete(m.groupedIds, id)
	}
}

func (m *mockActor) IsFollowerGrouped(id string) bool {
	return m.groupedIds[id]
}

func (m *mockActor) GroupedFollowers() []game.FollowTarget {
	var out []game.FollowTarget
	for _, ft := range m.followers {
		if m.groupedIds[ft.Id()] {
			out = append(out, ft)
		}
	}
	return out
}

func (m *mockActor) Move(_, to *game.RoomInstance) {
	m.moved = true
	m.roomId = to.Room.Id()
}
