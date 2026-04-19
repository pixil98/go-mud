package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestSpawnObject(t *testing.T) {
	tests := map[string]struct {
		spec          assets.ObjectSpawn
		wantContents  int
		wantContainer bool
	}{
		"simple object has no contents": {
			spec: assets.ObjectSpawn{
				Object: storage.NewResolvedSmartIdentifier("sword", &assets.Object{
					Aliases: []string{"sword"}, ShortDesc: "a sword",
				}),
			},
			wantContents:  0,
			wantContainer: false,
		},
		"container with contents spawned recursively": {
			spec: assets.ObjectSpawn{
				Object: storage.NewResolvedSmartIdentifier("box", &assets.Object{
					Aliases: []string{"box"}, ShortDesc: "a box",
					Flags: []string{"container"},
				}),
				Contents: []assets.ObjectSpawn{
					{Object: storage.NewResolvedSmartIdentifier("coin", &assets.Object{
						Aliases: []string{"coin"}, ShortDesc: "a coin",
					})},
					{Object: storage.NewResolvedSmartIdentifier("gem", &assets.Object{
						Aliases: []string{"gem"}, ShortDesc: "a gem",
					})},
				},
			},
			wantContents:  2,
			wantContainer: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oi, err := SpawnObject(tc.spec)
			if err != nil {
				t.Fatalf("SpawnObject: %v", err)
			}
			if oi == nil {
				t.Fatal("SpawnObject returned nil")
			}
			if tc.wantContainer {
				if oi.Contents == nil {
					t.Fatal("Contents is nil, want non-nil for container")
				}
				count := 0
				oi.Contents.ForEachObj(func(string, *ObjectInstance) { count++ })
				if count != tc.wantContents {
					t.Errorf("contents count = %d, want %d", count, tc.wantContents)
				}
			} else {
				if oi.Contents != nil {
					t.Errorf("Contents = %v, want nil for non-container", oi.Contents)
				}
			}
		})
	}
}

func TestObjectInstance_ActivateDecay(t *testing.T) {
	tests := map[string]struct {
		lifetime      int
		callTwice     bool
		wantRemaining int
	}{
		"permanent item is not activated": {
			lifetime:      0,
			wantRemaining: 0,
		},
		"decayable item is activated": {
			lifetime:      10,
			wantRemaining: 10,
		},
		"second call is a no-op": {
			lifetime:      10,
			callTwice:     true,
			wantRemaining: 5,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oi := newTestObj("a", tc.lifetime)
			oi.ActivateDecay()
			if tc.callTwice {
				oi.RemainingTicks = 5
				oi.ActivateDecay()
			}
			if oi.RemainingTicks != tc.wantRemaining {
				t.Errorf("RemainingTicks = %d, want %d", oi.RemainingTicks, tc.wantRemaining)
			}
		})
	}
}

func TestObjectInstance_Tick(t *testing.T) {
	tests := map[string]struct {
		lifetime      int
		activate      bool
		ticks         int
		wantRemaining int
	}{
		"permanent item stays at zero": {
			lifetime:      0,
			ticks:         3,
			wantRemaining: 0,
		},
		"inactive decayable item is not decremented": {
			lifetime:      5,
			activate:      false,
			ticks:         3,
			wantRemaining: 0,
		},
		"active item decrements each tick": {
			lifetime:      5,
			activate:      true,
			ticks:         3,
			wantRemaining: 2,
		},
		"item reaches zero": {
			lifetime:      2,
			activate:      true,
			ticks:         2,
			wantRemaining: 0,
		},
		"item does not go negative": {
			lifetime:      1,
			activate:      true,
			ticks:         3,
			wantRemaining: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oi := newTestObj("a", tc.lifetime)
			if tc.activate {
				oi.ActivateDecay()
			}
			for range tc.ticks {
				oi.Tick()
			}
			if oi.RemainingTicks != tc.wantRemaining {
				t.Errorf("RemainingTicks = %d, want %d", oi.RemainingTicks, tc.wantRemaining)
			}
		})
	}
}

func TestObjectInstance_Tick_RecursesContents(t *testing.T) {
	container := storage.NewResolvedSmartIdentifier("test-container", &assets.Object{
		Aliases:   []string{"a"},
		ShortDesc: "a container",
		Flags:     []string{"container"},
	})
	oi, _ := NewObjectInstance(container)

	inner := newTestObj("b", 3)
	inner.ActivateDecay()
	oi.Contents.AddObj(inner)

	oi.Tick()

	if inner.RemainingTicks != 2 {
		t.Errorf("inner RemainingTicks = %d, want 2", inner.RemainingTicks)
	}
}

func TestObjectInstance_Resolve(t *testing.T) {
	tests := map[string]struct {
		obj     *assets.Object
		wantErr bool
	}{
		"non-container resolves cleanly": {
			obj: &assets.Object{Aliases: []string{"sword"}, ShortDesc: "a sword"},
		},
		"container without contents gets empty Contents initialized": {
			obj: &assets.Object{Aliases: []string{"box"}, ShortDesc: "a box", Flags: []string{"container"}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			si := storage.NewResolvedSmartIdentifier("obj", tc.obj)
			oi, _ := NewObjectInstance(si)
			store := newFakeStore[*assets.Object](map[string]*assets.Object{"obj": tc.obj})

			err := oi.Resolve(store)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Resolve: %v", err)
			}
			if tc.obj.HasFlag(assets.ObjectFlagContainer) && oi.Contents == nil {
				t.Error("Contents should be initialized for container after Resolve")
			}
		})
	}
}


