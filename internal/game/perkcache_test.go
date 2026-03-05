package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

func TestPerkCache(t *testing.T) {
	tests := map[string]struct {
		own     []assets.Perk
		sources []PerkSource
		key     string
		wantMod int
	}{
		"empty cache returns zero": {
			key:     "test-key",
			wantMod: 0,
		},
		"single modifier": {
			own: []assets.Perk{
				{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
			},
			key:     "test-key",
			wantMod: 5,
		},
		"modifiers sum across own": {
			own: []assets.Perk{
				{Type: assets.PerkTypeModifier, Key: "test-key", Value: 3},
				{Type: assets.PerkTypeModifier, Key: "test-key", Value: 7},
			},
			key:     "test-key",
			wantMod: 10,
		},
		"modifiers sum across sources": {
			own: []assets.Perk{
				{Type: assets.PerkTypeModifier, Key: "test-key", Value: 2},
			},
			sources: []PerkSource{
				NewPerkCache([]assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 3},
				}),
			},
			key:     "test-key",
			wantMod: 5,
		},
		"nested sources aggregate": {
			sources: []PerkSource{
				NewPerkCache([]assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 1},
				}, NewPerkCache([]assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 2},
				})),
			},
			key:     "test-key",
			wantMod: 3,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pc := &PerkCache{own: tc.own, sources: tc.sources}
			got := pc.ModifierValue(tc.key)
			if got != tc.wantMod {
				t.Errorf("ModifierValue(%q) = %d, want %d", tc.key, got, tc.wantMod)
			}
		})
	}
}

func TestPerkCacheGrants(t *testing.T) {
	pc := NewPerkCache(
		[]assets.Perk{
			{Type: assets.PerkTypeGrant, Key: "test-grant", Arg: "a"},
		},
		NewPerkCache([]assets.Perk{
			{Type: assets.PerkTypeGrant, Key: "test-grant", Arg: "b"},
		}),
	)

	args := pc.GrantArgs("test-grant")
	if len(args) != 2 {
		t.Fatalf("GrantArgs returned %d args, want 2", len(args))
	}
	if !pc.HasGrant("test-grant", "a") {
		t.Error("HasGrant(test-grant, a) = false, want true")
	}
	if !pc.HasGrant("test-grant", "b") {
		t.Error("HasGrant(test-grant, b) = false, want true")
	}
	if pc.HasGrant("test-grant", "c") {
		t.Error("HasGrant(test-grant, c) = true, want false")
	}
}

func TestPerkCacheLazyResolution(t *testing.T) {
	source := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
	})
	pc := NewPerkCache(nil, source)

	// Initial resolve
	if got := pc.ModifierValue("test-key"); got != 5 {
		t.Fatalf("initial ModifierValue = %d, want 5", got)
	}

	// Update source via SetOwn, parent should see new value
	source.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 10},
	})
	if got := pc.ModifierValue("test-key"); got != 10 {
		t.Errorf("after SetOwn ModifierValue = %d, want 10", got)
	}
}

func TestPerkCacheInvalidate(t *testing.T) {
	pc := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
	})

	// Resolve cache
	pc.ModifierValue("test-key")

	// Invalidate and change own
	pc.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 99},
	})
	if got := pc.ModifierValue("test-key"); got != 99 {
		t.Errorf("after invalidate ModifierValue = %d, want 99", got)
	}
}
