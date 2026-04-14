package game

// threatEntry holds the accumulated threat value and a reference to the enemy actor.
type threatEntry struct {
	threat int
	actor  Actor
}

// ThreatTable tracks which enemies an actor is fighting and the accumulated
// threat each enemy has generated. All methods assume the caller holds the
// owning ActorInstance's write lock.
type ThreatTable struct {
	entries map[string]*threatEntry // enemy ID → entry
}

// ensureEntry idempotently adds an enemy with an initial threat of 1.
func (t *ThreatTable) ensureEntry(enemyId string, actor Actor) {
	if t.entries == nil {
		t.entries = make(map[string]*threatEntry)
	}
	if _, ok := t.entries[enemyId]; !ok {
		t.entries[enemyId] = &threatEntry{threat: 1, actor: actor}
	}
}

// addThreat increments the threat that sourceId has generated on this actor.
func (t *ThreatTable) addThreat(sourceId string, amount int) {
	if e, ok := t.entries[sourceId]; ok {
		e.threat += amount
	}
}

// setThreat sets the threat that sourceId has on this actor to an absolute value.
func (t *ThreatTable) setThreat(sourceId string, amount int) {
	if e, ok := t.entries[sourceId]; ok {
		e.threat = amount
	}
}

// topThreat sets sourceId's threat to one more than the current highest entry,
// guaranteeing sourceId becomes the top-threat enemy.
func (t *ThreatTable) topThreat(sourceId string) {
	maxThreat := 0
	for _, e := range t.entries {
		if e.threat > maxThreat {
			maxThreat = e.threat
		}
	}
	if e, ok := t.entries[sourceId]; ok {
		e.threat = maxThreat + 1
	}
}

// hasEntry reports whether enemyId is on this actor's threat table.
func (t *ThreatTable) hasEntry(enemyId string) bool {
	_, ok := t.entries[enemyId]
	return ok
}

// hasEntries reports whether the threat table has any entries at all.
func (t *ThreatTable) hasEntries() bool {
	return len(t.entries) > 0
}

// removeEntry removes an enemy from the threat table.
func (t *ThreatTable) removeEntry(enemyId string) {
	delete(t.entries, enemyId)
}

// snapshot returns a copy of the threat values for safe iteration outside
// the lock (e.g. XP distribution after death).
func (t *ThreatTable) snapshot() map[string]int {
	if len(t.entries) == 0 {
		return nil
	}
	snap := make(map[string]int, len(t.entries))
	for k, e := range t.entries {
		snap[k] = e.threat
	}
	return snap
}

// clear removes all threat entries.
func (t *ThreatTable) clear() {
	clear(t.entries)
}

// resolveTarget picks the best target for this actor. If preferredId is
// non-empty and present in the table, it wins. Otherwise the highest-threat
// entry is returned. Returns nil if the table is empty.
func (t *ThreatTable) resolveTarget(preferredId string) Actor {
	if preferredId != "" {
		if e, ok := t.entries[preferredId]; ok {
			return e.actor
		}
	}
	var best *threatEntry
	for _, e := range t.entries {
		if best == nil || e.threat > best.threat {
			best = e
		}
	}
	if best == nil {
		return nil
	}
	return best.actor
}

// enemies returns a snapshot of all enemy Actor references.
func (t *ThreatTable) enemies() []Actor {
	if len(t.entries) == 0 {
		return nil
	}
	out := make([]Actor, 0, len(t.entries))
	for _, e := range t.entries {
		out = append(out, e.actor)
	}
	return out
}
