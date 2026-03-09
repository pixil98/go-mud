package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

func TestPerkCache(t *testing.T) {
	tests := map[string]struct {
		own     []assets.Perk
		sources map[string]PerkSource
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
			sources: map[string]PerkSource{
				"src-a": NewPerkCache([]assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 3},
				}, nil),
			},
			key:     "test-key",
			wantMod: 5,
		},
		"nested sources aggregate": {
			sources: map[string]PerkSource{
				"src-a": NewPerkCache([]assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 1},
				}, map[string]PerkSource{
					"nested": NewPerkCache([]assets.Perk{
						{Type: assets.PerkTypeModifier, Key: "test-key", Value: 2},
					}, nil),
				}),
			},
			key:     "test-key",
			wantMod: 3,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pc := NewPerkCache(tc.own, tc.sources)
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
		map[string]PerkSource{
			"src-a": NewPerkCache([]assets.Perk{
				{Type: assets.PerkTypeGrant, Key: "test-grant", Arg: "b"},
			}, nil),
		},
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

func TestPerkCacheSourceVersionDetection(t *testing.T) {
	source := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
	}, nil)
	pc := NewPerkCache(nil, map[string]PerkSource{"src-a": source})

	if got := pc.ModifierValue("test-key"); got != 5 {
		t.Fatalf("initial ModifierValue = %d, want 5", got)
	}

	// Update source via SetOwn — parent should detect version change.
	source.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 10},
	})
	if got := pc.ModifierValue("test-key"); got != 10 {
		t.Errorf("after SetOwn ModifierValue = %d, want 10", got)
	}
}

func TestPerkCacheSetOwn(t *testing.T) {
	pc := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
	}, nil)

	pc.ModifierValue("test-key") // force resolve

	pc.SetOwn([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 99},
	})
	if got := pc.ModifierValue("test-key"); got != 99 {
		t.Errorf("after SetOwn ModifierValue = %d, want 99", got)
	}
}

func TestPerkCacheAddRemoveSource(t *testing.T) {
	pc := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 1},
	}, nil)

	src := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 10},
	}, nil)

	if got := pc.ModifierValue("test-key"); got != 1 {
		t.Fatalf("before AddSource = %d, want 1", got)
	}

	pc.AddSource("buffs", src)
	if got := pc.ModifierValue("test-key"); got != 11 {
		t.Errorf("after AddSource = %d, want 11", got)
	}

	pc.RemoveSource("buffs")
	if got := pc.ModifierValue("test-key"); got != 1 {
		t.Errorf("after RemoveSource = %d, want 1", got)
	}

	// RemoveSource of non-existent key is a no-op.
	pc.RemoveSource("nonexistent")
	if got := pc.ModifierValue("test-key"); got != 1 {
		t.Errorf("after removing nonexistent = %d, want 1", got)
	}
}

func TestPerkCacheTimed(t *testing.T) {
	tests := map[string]struct {
		setup   func(*PerkCache)
		ticks   int
		key     string
		wantMod int
	}{
		"empty cache returns zero": {
			key:     "test-key",
			wantMod: 0,
		},
		"single timed perk": {
			setup: func(pc *PerkCache) {
				pc.AddTimedPerks("test-entry", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
				}, 3)
			},
			key:     "test-key",
			wantMod: 5,
		},
		"multiple entries sum": {
			setup: func(pc *PerkCache) {
				pc.AddTimedPerks("entry-a", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 3},
				}, 5)
				pc.AddTimedPerks("entry-b", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 7},
				}, 5)
			},
			key:     "test-key",
			wantMod: 10,
		},
		"replacement replaces": {
			setup: func(pc *PerkCache) {
				pc.AddTimedPerks("test-entry", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
				}, 3)
				pc.AddTimedPerks("test-entry", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 99},
				}, 3)
			},
			key:     "test-key",
			wantMod: 99,
		},
		"expires after ticks": {
			setup: func(pc *PerkCache) {
				pc.AddTimedPerks("test-entry", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
				}, 2)
			},
			ticks:   2,
			key:     "test-key",
			wantMod: 0,
		},
		"partial expiry": {
			setup: func(pc *PerkCache) {
				pc.AddTimedPerks("short", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 3},
				}, 1)
				pc.AddTimedPerks("long", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 7},
				}, 3)
			},
			ticks:   1,
			key:     "test-key",
			wantMod: 7,
		},
		"timed and own perks sum": {
			setup: func(pc *PerkCache) {
				pc.SetOwn([]assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 10},
				})
				pc.AddTimedPerks("buff", []assets.Perk{
					{Type: assets.PerkTypeModifier, Key: "test-key", Value: 5},
				}, 3)
			},
			key:     "test-key",
			wantMod: 15,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pc := NewPerkCache(nil, nil)
			if tc.setup != nil {
				tc.setup(pc)
			}
			for i := 0; i < tc.ticks; i++ {
				pc.Tick()
			}
			if got := pc.ModifierValue(tc.key); got != tc.wantMod {
				t.Errorf("ModifierValue(%q) = %d, want %d", tc.key, got, tc.wantMod)
			}
		})
	}
}


func TestPerkCacheTimedAsSource(t *testing.T) {
	// A PerkCache with timed perks used as a source for another PerkCache.
	src := NewPerkCache(nil, nil)
	src.AddTimedPerks("test-entry", []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 10},
	}, 3)

	pc := NewPerkCache([]assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 1},
	}, map[string]PerkSource{"timed": src})

	if got := pc.ModifierValue("test-key"); got != 11 {
		t.Errorf("with timed source = %d, want 11", got)
	}

	// Expire the timed perk; parent should detect version change.
	for i := 0; i < 3; i++ {
		src.Tick()
	}
	if got := pc.ModifierValue("test-key"); got != 1 {
		t.Errorf("after expiry = %d, want 1", got)
	}
}

func TestPerkCacheChain(t *testing.T) {
	world := NewPerkCache(nil, nil)
	zone := NewPerkCache(nil, map[string]PerkSource{"world": world})
	room := NewPerkCache(nil, map[string]PerkSource{"zone": zone})

	world.AddTimedPerks("test-world", []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 1},
	}, 5)
	zone.AddTimedPerks("test-zone", []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 10},
	}, 5)
	room.AddTimedPerks("test-room", []assets.Perk{
		{Type: assets.PerkTypeModifier, Key: "test-key", Value: 100},
	}, 5)

	resolved, _ := room.Snapshot()
	got := resolved.modifiers["test-key"]
	if got != 111 {
		t.Errorf("chained value = %d, want 111", got)
	}
}
