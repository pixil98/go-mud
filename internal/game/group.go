package game

import "sync"

// Group represents a runtime player group.
// A group has a leader and one or more members (including the leader).
// It implements PlayerGroup so it can be used as a publish target.
type Group struct {
	mu       sync.RWMutex
	LeaderId string
	members  map[string]*PlayerState
}

// NewGroup creates a new group with the given player as leader and sole member.
func NewGroup(leaderId string, leader *PlayerState) *Group {
	return &Group{
		LeaderId: leaderId,
		members:  map[string]*PlayerState{leaderId: leader},
	}
}

// ForEachPlayer calls fn for each member of the group.
// Satisfies the PlayerGroup interface.
func (g *Group) ForEachPlayer(fn func(string, *PlayerState)) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for id, ps := range g.members {
		fn(id, ps)
	}
}

// AddMember adds a player to the group.
func (g *Group) AddMember(charId string, ps *PlayerState) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.members[charId] = ps
}

// RemoveMember removes a player from the group.
// Returns true if the player was in the group.
func (g *Group) RemoveMember(charId string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.members[charId]; !ok {
		return false
	}
	delete(g.members, charId)
	return true
}

// HasMember reports whether charId is a member of the group.
func (g *Group) HasMember(charId string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.members[charId]
	return ok
}
