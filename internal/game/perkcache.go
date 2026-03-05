package game

import (
	"slices"

	"github.com/pixil98/go-mud/internal/assets"
)

// ResolvedPerks holds pre-aggregated perk data for fast lookup.
// Modifier values are summed per key; grant args are collected per key.
type ResolvedPerks struct {
	modifiers map[string]int
	grants    map[string][]string
}

func newResolvedPerks() *ResolvedPerks {
	return &ResolvedPerks{
		modifiers: make(map[string]int),
		grants:    make(map[string][]string),
	}
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

// PerkSource is implemented by any type that provides a PerkCache.
// Types that embed PerkCache satisfy this automatically.
type PerkSource interface {
	perkCache() *PerkCache
}

// perkCache returns the receiver, satisfying PerkSource for types that embed PerkCache.
func (pc *PerkCache) perkCache() *PerkCache { return pc }

// PerkCache is a composable, lazy-resolving perk aggregator.
// It holds its own perks and optional nested sources (any PerkSource).
// Resolution is lazy: the first query after invalidation rebuilds the cache.
//
// Thread safety: PerkCache is NOT internally locked. The owning struct
// must hold its own mutex when calling PerkCache methods.
type PerkCache struct {
	own      []assets.Perk
	sources  []PerkSource
	resolved *ResolvedPerks
}

// NewPerkCache creates a PerkCache with the given own perks and nested sources.
func NewPerkCache(own []assets.Perk, sources ...PerkSource) *PerkCache {
	return &PerkCache{
		own:     own,
		sources: sources,
	}
}

// SetOwn replaces the cache's own perks and invalidates the resolved state.
func (pc *PerkCache) SetOwn(perks []assets.Perk) {
	pc.own = perks
	pc.Invalidate()
}

// Invalidate clears the resolved state, forcing re-resolution on the next query.
func (pc *PerkCache) Invalidate() {
	pc.resolved = nil
}

// isDirty returns true if this cache or any nested source needs resolution.
func (pc *PerkCache) isDirty() bool {
	if pc.resolved == nil {
		return true
	}
	for _, s := range pc.sources {
		if s.perkCache().isDirty() {
			return true
		}
	}
	return false
}

// resolve lazily builds the ResolvedPerks if dirty.
func (pc *PerkCache) resolve() *ResolvedPerks {
	if !pc.isDirty() {
		return pc.resolved
	}
	r := newResolvedPerks()
	r.addPerks(pc.own)
	for _, s := range pc.sources {
		r.merge(s.perkCache().resolve())
	}
	pc.resolved = r
	return r
}

// ModifierValue returns the summed value for a modifier perk key.
func (pc *PerkCache) ModifierValue(key string) int {
	return pc.resolve().modifiers[key]
}

// Modifiers returns the full modifier map. Do not mutate the returned map.
func (pc *PerkCache) Modifiers() map[string]int {
	return pc.resolve().modifiers
}

// GrantArgs returns all args for a grant perk key.
func (pc *PerkCache) GrantArgs(key string) []string {
	return pc.resolve().grants[key]
}

// HasGrant returns true if any grant perk matches both key and arg.
func (pc *PerkCache) HasGrant(key, arg string) bool {
	return slices.Contains(pc.resolve().grants[key], arg)
}

// Grants returns the full grants map. Do not mutate the returned map.
func (pc *PerkCache) Grants() map[string][]string {
	return pc.resolve().grants
}
