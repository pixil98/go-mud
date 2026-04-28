package game

// GroupLeader returns the root group leader for actor, or nil if actor is
// not in a group. Walks up the follow tree through grouped links to find
// the top-most leader. An actor with grouped followers but no leader above
// them is their own leader.
func GroupLeader(actor Actor) Actor {
	parent := actor.Following()
	if parent == nil || !parent.IsFollowerGrouped(actor.Id()) {
		if len(actor.GroupedFollowers()) > 0 {
			return actor
		}
		return nil
	}
	for i := 0; i < 100; i++ {
		next := parent.Following()
		if next == nil || !next.IsFollowerGrouped(parent.Id()) {
			return parent
		}
		parent = next
	}
	return parent
}

// WalkGroup calls fn for every member of the grouped sub-tree rooted at
// leader (leader + all grouped descendants, depth-first). Non-grouped
// followers are skipped. Safe to call on a solo actor — fn is called once
// with that actor.
func WalkGroup(leader Actor, fn func(Actor)) {
	fn(leader)
	for _, member := range leader.GroupedFollowers() {
		WalkGroup(member, fn)
	}
}

// IsPlayerSide reports whether actor is on the player side — either a
// character, or a mob somewhere down a character's follow chain (pets,
// charms, escorted NPCs). Mobs with no player ancestor in their follow
// chain are mob-side.
func IsPlayerSide(actor Actor) bool {
	cur := actor
	for i := 0; i < 100; i++ {
		if cur == nil {
			return false
		}
		if cur.IsCharacter() {
			return true
		}
		cur = cur.Following()
	}
	return false
}

// AreAllies reports whether a and b belong to the same group. An actor is
// always their own ally; solo actors have no other allies.
func AreAllies(a, b Actor) bool {
	if a.Id() == b.Id() {
		return true
	}
	la := GroupLeader(a)
	if la == nil {
		return false
	}
	lb := GroupLeader(b)
	if lb == nil {
		return false
	}
	return la.Id() == lb.Id()
}

// GroupPublishTarget returns a target that yields the leader and all grouped
// character followers recursively. Mobs are skipped since they have no client
// connection to publish to.
func GroupPublishTarget(leader Actor) groupPublishTarget {
	return groupPublishTarget{leader: leader}
}

type groupPublishTarget struct {
	leader Actor
}

func (g groupPublishTarget) ForEachPlayer(fn func(string, *CharacterInstance)) {
	WalkGroup(g.leader, func(a Actor) {
		if a.IsCharacter() {
			fn(a.Id(), nil)
		}
	})
}

// Publish delivers data to every grouped character in the leader's group,
// skipping any whose id appears in exclude.
func (g groupPublishTarget) Publish(data []byte, exclude []string) {
	WalkGroup(g.leader, func(a Actor) {
		if a.IsCharacter() {
			a.Publish(data, exclude)
		}
	})
}
