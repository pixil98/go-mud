// Package gametest provides test doubles for the game package.
package gametest

import (
	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
)

// BaseActor is a test double for game.Actor with sensible defaults.
// All methods return zero/empty values unless overridden via fields.
// Tests set only the fields they care about.
type BaseActor struct {
	ActorId        string
	ActorName      string
	ActorRoom      *game.RoomInstance
	Alive          bool
	ActorLevel     int
	Character      bool
	InCombat       bool
	CombatTarget   string
	Notified       []string
	ActorFollowing game.Actor
	ActorFollowers []game.Actor
	GroupedIds     map[string]bool
	Resources      map[string][2]int // name → {current, max}
	Grants         map[string]bool
	GrantValues    map[string][]string
	SpendAPFails   bool
	SpentAP        int
	Moved          bool

	// Override hooks for behaviors that vary per-test.
	IsAliveFunc func() bool
	OnDeathFunc func() []*game.ObjectInstance
}

var _ game.Actor = (*BaseActor)(nil)

func (a *BaseActor) Id() string   { return a.ActorId }
func (a *BaseActor) Name() string { return a.ActorName }

func (a *BaseActor) Room() *game.RoomInstance { return a.ActorRoom }

func (a *BaseActor) IsInCombat() bool      { return a.InCombat }
func (a *BaseActor) SetInCombat(v bool)    { a.InCombat = v }
func (a *BaseActor) IsCharacter() bool     { return a.Character }
func (a *BaseActor) Level() int            { return a.ActorLevel }
func (a *BaseActor) CombatTargetId() string { return a.CombatTarget }
func (a *BaseActor) SetCombatTargetId(id string) { a.CombatTarget = id }

func (a *BaseActor) IsAlive() bool {
	if a.IsAliveFunc != nil {
		return a.IsAliveFunc()
	}
	return a.Alive
}

func (a *BaseActor) Resource(name string) (int, int) {
	if r, ok := a.Resources[name]; ok {
		return r[0], r[1]
	}
	return 0, 0
}

func (a *BaseActor) AdjustResource(name string, delta int, _ bool) {
	if r, ok := a.Resources[name]; ok {
		r[0] += delta
		a.Resources[name] = r
	}
}

func (a *BaseActor) ForEachResource(fn func(name string, current, maximum int)) {
	for name, r := range a.Resources {
		fn(name, r[0], r[1])
	}
}

func (a *BaseActor) SpendAP(cost int) bool {
	a.SpentAP = cost
	return !a.SpendAPFails
}

func (a *BaseActor) HasGrant(key, _ string) bool { return a.Grants[key] }
func (a *BaseActor) ModifierValue(string) int    { return 0 }

func (a *BaseActor) GrantArgs(key string) []string {
	return a.GrantValues[key]
}

func (a *BaseActor) AddTimedPerks(string, []assets.Perk, int) {}

func (a *BaseActor) OnDeath() []*game.ObjectInstance {
	if a.OnDeathFunc != nil {
		return a.OnDeathFunc()
	}
	return nil
}

func (a *BaseActor) Inventory() *game.Inventory  { return nil }
func (a *BaseActor) Equipment() *game.Equipment  { return nil }
func (a *BaseActor) Notify(msg string)            { a.Notified = append(a.Notified, msg) }

func (a *BaseActor) Following() game.Actor          { return a.ActorFollowing }
func (a *BaseActor) SetFollowing(ft game.Actor)      { a.ActorFollowing = ft }
func (a *BaseActor) Followers() []game.Actor          { return a.ActorFollowers }
func (a *BaseActor) AddFollower(ft game.Actor)        { a.ActorFollowers = append(a.ActorFollowers, ft) }
func (a *BaseActor) RemoveFollower(string)             {}

func (a *BaseActor) SetFollowerGrouped(id string, grouped bool) {
	if a.GroupedIds == nil {
		a.GroupedIds = make(map[string]bool)
	}
	if grouped {
		a.GroupedIds[id] = true
	} else {
		delete(a.GroupedIds, id)
	}
}

func (a *BaseActor) IsFollowerGrouped(id string) bool {
	return a.GroupedIds[id]
}

func (a *BaseActor) GroupedFollowers() []game.Actor {
	var out []game.Actor
	for _, ft := range a.ActorFollowers {
		if a.GroupedIds[ft.Id()] {
			out = append(out, ft)
		}
	}
	return out
}

func (a *BaseActor) Move(_, to *game.RoomInstance) {
	a.Moved = true
	a.ActorRoom = to
}

func (a *BaseActor) EnsureThreat(string, game.Actor) {}
func (a *BaseActor) AddThreatFrom(string, int)       {}
func (a *BaseActor) SetThreatFrom(string, int)       {}
func (a *BaseActor) TopThreatFrom(string)             {}
func (a *BaseActor) HasThreatFrom(string) bool        { return false }
func (a *BaseActor) ThreatSnapshot() map[string]int   { return nil }
func (a *BaseActor) ClaimDeath() bool                  { return true }
func (a *BaseActor) QueueTickMsg(msg string)          { a.Notified = append(a.Notified, msg) }
func (a *BaseActor) StatSections() []game.StatSection  { return nil }
