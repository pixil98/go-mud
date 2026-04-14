package game

// GroupPublishTarget returns a PlayerGroup that yields the leader and all
// grouped followers recursively down the tree. Only characters are included;
// mobs are skipped since they have no NATS subscriptions.
func GroupPublishTarget(leader Actor) PlayerGroup {
	return groupPublishTarget{leader: leader}
}

type groupPublishTarget struct {
	leader Actor
}

func (g groupPublishTarget) ForEachPlayer(fn func(string, *CharacterInstance)) {
	walkGrouped(g.leader, fn)
}

func walkGrouped(ft Actor, fn func(string, *CharacterInstance)) {
	if ft.IsCharacter() {
		fn(ft.Id(), nil)
	}
	for _, member := range ft.GroupedFollowers() {
		walkGrouped(member, fn)
	}
}
