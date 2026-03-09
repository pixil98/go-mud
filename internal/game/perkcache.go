package game

import (
	"slices"
	"sync"

	"github.com/pixil98/go-mud/internal/assets"
)

// ResolvedPerks holds pre-aggregated perk data for fast lookup.
// Modifier values are summed per key; grant args are collected per key.
type ResolvedPerks struct {
	modifiers map[string]int
	grants    map[string][]string
}

// NewResolvedPerks creates a ResolvedPerks, optionally pre-populated from a perk list.
func NewResolvedPerks(perks []assets.Perk) *ResolvedPerks {
	r := &ResolvedPerks{
		modifiers: make(map[string]int),
		grants:    make(map[string][]string),
	}
	r.addPerks(perks)
	return r
}

func (r *ResolvedPerks) addPerks(perks []assets.Perk) {
	for _, p := range perks {
		switch p.Type {
		case assets.PerkTypeModifier:
			r.modifiers[p.Key] += p.Value
		case assets.PerkTypeGrant:
			r.grants[p.Key] = append(r.grants[p.Key], p.Arg)
		}
	}
}

func (r *ResolvedPerks) merge(other *ResolvedPerks) {
	for k, v := range other.modifiers {
		r.modifiers[k] += v
	}
	for k, args := range other.grants {
		r.grants[k] = append(r.grants[k], args...)
	}
}

// PerkSource provides pre-resolved perks for composition into a PerkCache.
// Snapshot returns the resolved perks and a version counter atomically.
// The version must increment whenever the perks change.
type PerkSource interface {
	Snapshot() (resolved *ResolvedPerks, version uint64)
}

// timedPerk is a named set of perks with a remaining tick count.
type timedPerk struct {
	perks     []assets.Perk
	remaining int
}

// PerkCache is a lazy-resolving perk aggregator. It holds static own perks,
// timed perks that expire after a set number of ticks, and optional named
// PerkSources. Resolution is lazy: the first query after a change rebuilds
// the cache.
//
// PerkCache is safe for concurrent use.
type PerkCache struct {
	mu             *sync.Mutex // pointer so copying the struct does not copy the mutex
	own            []assets.Perk
	timedEntries   map[string]*timedPerk
	sources        map[string]PerkSource
	sourceVersions map[string]uint64
	version        uint64
	resolved       *ResolvedPerks
}

// NewPerkCache creates a PerkCache with the given own perks and named sources.
func NewPerkCache(own []assets.Perk, sources map[string]PerkSource) *PerkCache {
	if sources == nil {
		sources = make(map[string]PerkSource)
	}
	return &PerkCache{
		mu:             &sync.Mutex{},
		own:            own,
		timedEntries:   make(map[string]*timedPerk),
		sources:        sources,
		sourceVersions: make(map[string]uint64),
	}
}

// SetOwn replaces the cache's own perks and invalidates the resolved state.
func (pc *PerkCache) SetOwn(perks []assets.Perk) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.own = perks
	pc.invalidate()
}

// AddSource adds a named PerkSource and invalidates the cache.
func (pc *PerkCache) AddSource(name string, s PerkSource) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.sources[name] = s
	pc.invalidate()
}

// RemoveSource removes a named PerkSource and invalidates the cache.
// No-op if the name is not found.
func (pc *PerkCache) RemoveSource(name string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if _, ok := pc.sources[name]; ok {
		delete(pc.sources, name)
		delete(pc.sourceVersions, name)
		pc.invalidate()
	}
}

// AddTimedPerks registers a named set of perks with a tick duration.
// If an entry with the same name already exists, it is replaced.
func (pc *PerkCache) AddTimedPerks(name string, perks []assets.Perk, ticks int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.timedEntries[name] = &timedPerk{perks: perks, remaining: ticks}
	pc.invalidate()
}

// Tick decrements all timed perk timers and removes expired entries.
// Returns true if any entries were removed.
func (pc *PerkCache) Tick() bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	changed := false
	for name, e := range pc.timedEntries {
		e.remaining--
		if e.remaining <= 0 {
			delete(pc.timedEntries, name)
			changed = true
		}
	}
	if changed {
		pc.invalidate()
	}
	return changed
}

// invalidate clears the resolved state and increments the version.
// Caller must hold pc.mu.
func (pc *PerkCache) invalidate() {
	pc.resolved = nil
	pc.version++
}

// isDirty returns true if the cache needs re-resolution.
// Caller must hold pc.mu.
func (pc *PerkCache) isDirty() bool {
	if pc.resolved == nil {
		return true
	}
	for name, s := range pc.sources {
		_, v := s.Snapshot()
		if v != pc.sourceVersions[name] {
			return true
		}
	}
	return false
}

// resolve lazily builds the ResolvedPerks if dirty.
// Caller must hold pc.mu.
func (pc *PerkCache) resolve() *ResolvedPerks {
	if !pc.isDirty() {
		return pc.resolved
	}
	r := NewResolvedPerks(pc.own)
	for _, e := range pc.timedEntries {
		r.addPerks(e.perks)
	}
	for name, s := range pc.sources {
		resolved, v := s.Snapshot()
		r.merge(resolved)
		pc.sourceVersions[name] = v
	}
	pc.resolved = r
	return r
}

// Snapshot returns the resolved perks and a composite version that
// reflects changes in both own perks and all sources.
func (pc *PerkCache) Snapshot() (*ResolvedPerks, uint64) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	resolved := pc.resolve()
	v := pc.version
	for _, s := range pc.sources {
		_, sv := s.Snapshot()
		v += sv
	}
	return resolved, v
}

// ModifierValue returns the summed value for a modifier perk key.
func (pc *PerkCache) ModifierValue(key string) int {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.resolve().modifiers[key]
}

// Modifiers returns the full modifier map. Do not mutate the returned map.
func (pc *PerkCache) Modifiers() map[string]int {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.resolve().modifiers
}

// GrantArgs returns all args for a grant perk key.
func (pc *PerkCache) GrantArgs(key string) []string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.resolve().grants[key]
}

// HasGrant returns true if any grant perk matches both key and arg.
func (pc *PerkCache) HasGrant(key, arg string) bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return slices.Contains(pc.resolve().grants[key], arg)
}

// Grants returns the full grants map. Do not mutate the returned map.
func (pc *PerkCache) Grants() map[string][]string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.resolve().grants
}
