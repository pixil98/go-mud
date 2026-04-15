package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/gametest"
	"github.com/pixil98/go-mud/internal/storage"
)

// --- Mock finders ---

// mockFinder implements TargetFinder using simple maps keyed by name.
type mockFinder struct {
	players map[string]*game.CharacterInstance
	mobs    map[string]*game.MobileInstance
	objects map[string]*game.ObjectInstance
	exits   map[string]*game.ResolvedExit
}

func (f *mockFinder) FindPlayers(match func(*game.CharacterInstance) bool) []*game.CharacterInstance {
	var out []*game.CharacterInstance
	for _, ci := range f.players {
		if match(ci) {
			out = append(out, ci)
		}
	}
	return out
}

func (f *mockFinder) FindMobs(match func(*game.MobileInstance) bool) []*game.MobileInstance {
	var out []*game.MobileInstance
	for _, mi := range f.mobs {
		if match(mi) {
			out = append(out, mi)
		}
	}
	return out
}

func (f *mockFinder) FindObjs(match func(*game.ObjectInstance) bool) []*game.ObjectInstance {
	var out []*game.ObjectInstance
	for _, oi := range f.objects {
		if match(oi) {
			out = append(out, oi)
		}
	}
	return out
}

func (f *mockFinder) FindExit(name string) (string, *game.ResolvedExit) {
	name = strings.ToLower(name)
	if re, ok := f.exits[name]; ok {
		return name, re
	}
	return "", nil
}

// mockRemover tracks removed instance IDs.
type mockRemover struct {
	removed []string
}

func (r *mockRemover) RemoveObj(instanceId string) *game.ObjectInstance {
	r.removed = append(r.removed, instanceId)
	return nil
}

// mockScopes implements TargetScopes for testing ResolveSpecs.
type mockScopes struct {
	spaces []SearchSpace
}

func (s *mockScopes) SpacesFor(scope, game.Actor) ([]SearchSpace, error) {
	return s.spaces, nil
}

// --- FindTarget tests ---

func TestFindTarget(t *testing.T) {
	bob := &game.CharacterInstance{
		Character: storage.NewResolvedSmartIdentifier("bob", &assets.Character{Name: "Bob"}),
	}
	guard := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("guard", &assets.Mobile{Aliases: []string{"guard", "soldier"}, ShortDesc: "a burly guard"}),
		ActorInstance: game.ActorInstance{InstanceId: "guard-1"},
	}
	wolf1 := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("wolf-a", &assets.Mobile{Aliases: []string{"wolf"}, ShortDesc: "a grey wolf"}),
		ActorInstance: game.ActorInstance{InstanceId: "wolf-1"},
	}
	wolf2 := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("wolf-b", &assets.Mobile{Aliases: []string{"wolf"}, ShortDesc: "a black wolf"}),
		ActorInstance: game.ActorInstance{InstanceId: "wolf-2"},
	}
	goblin := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("goblin-a", &assets.Mobile{Aliases: []string{"goblin"}, ShortDesc: "a goblin"}),
		ActorInstance: game.ActorInstance{InstanceId: "goblin-1"},
	}
	sword := &game.ObjectInstance{
		InstanceId: "sword-1",
		Object:     storage.NewResolvedSmartIdentifier("sword", &assets.Object{Aliases: []string{"sword"}, ShortDesc: "a rusty sword"}),
	}

	tests := map[string]struct {
		raw       string
		tt        targetType
		spaces    []SearchSpace
		expCount  int
		expType   targetType
		expName   string  // first result name (Actor.Name or Obj.Name)
		expId     string  // first result actor ID (for cross-space tests)
		expSource bool    // object source non-nil
		expDir    string  // exit direction
		expErr    string
	}{
		// --- Basic name resolution ---
		"player by name": {
			raw:     "bob",
			tt:      targetTypePlayer,
			spaces:  []SearchSpace{{Finder: &mockFinder{players: map[string]*game.CharacterInstance{"bob": bob}}}},
			expType: targetTypeActor, expName: "Bob", expCount: 1,
		},
		"player case insensitive": {
			raw:     "BOB",
			tt:      targetTypePlayer,
			spaces:  []SearchSpace{{Finder: &mockFinder{players: map[string]*game.CharacterInstance{"bob": bob}}}},
			expType: targetTypeActor, expName: "Bob", expCount: 1,
		},
		"mob by alias": {
			raw:     "soldier",
			tt:      targetTypeMobile,
			spaces:  []SearchSpace{{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"guard-1": guard}}}},
			expType: targetTypeActor, expName: "a burly guard", expCount: 1,
		},
		"object with source": {
			raw:     "sword",
			tt:      targetTypeObject,
			spaces:  []SearchSpace{{Finder: &mockFinder{objects: map[string]*game.ObjectInstance{"sword-1": sword}}, Remover: &mockRemover{}}},
			expType: targetTypeObject, expName: "a rusty sword", expSource: true, expCount: 1,
		},
		"exit by direction": {
			raw:     "north",
			tt:      targetTypeExit,
			spaces:  []SearchSpace{{Finder: &mockFinder{exits: map[string]*game.ResolvedExit{"north": {}}}}},
			expType: targetTypeExit, expDir: "north", expCount: 1,
		},
		"exit case insensitive": {
			raw:     "NORTH",
			tt:      targetTypeExit,
			spaces:  []SearchSpace{{Finder: &mockFinder{exits: map[string]*game.ResolvedExit{"north": {}}}}},
			expType: targetTypeExit, expDir: "north", expCount: 1,
		},

		// --- Not found ---
		"player not found":  {raw: "nobody", tt: targetTypePlayer, spaces: []SearchSpace{{Finder: &mockFinder{}}}, expErr: "not found"},
		"mob not found":     {raw: "nobody", tt: targetTypeMobile, spaces: []SearchSpace{{Finder: &mockFinder{}}}, expErr: "not found"},
		"object not found":  {raw: "nothing", tt: targetTypeObject, spaces: []SearchSpace{{Finder: &mockFinder{}}}, expErr: "not found"},
		"empty spaces":      {raw: "bob", tt: targetTypePlayer, spaces: []SearchSpace{}, expErr: "not found"},

		// --- Type filtering ---
		"type filter skips player when only mob requested": {
			raw: "bob",
			tt:  targetTypeMobile,
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.CharacterInstance{"bob": bob},
				mobs:    map[string]*game.MobileInstance{"bob-1": {Mobile: storage.NewResolvedSmartIdentifier("bob-m", &assets.Mobile{Aliases: []string{"bob"}, ShortDesc: "a mob bob"}), ActorInstance: game.ActorInstance{InstanceId: "bob-1"}}},
			}}},
			expType: targetTypeActor, expCount: 1,
		},
		"object preferred over exit": {
			raw: "north",
			tt:  targetTypeObject | targetTypeExit,
			spaces: []SearchSpace{{Finder: &mockFinder{
				objects: map[string]*game.ObjectInstance{"door-1": {InstanceId: "door-1", Object: storage.NewResolvedSmartIdentifier("door", &assets.Object{Aliases: []string{"north"}, ShortDesc: "a door"})}},
				exits:   map[string]*game.ResolvedExit{"north": {}},
			}}},
			expType: targetTypeObject, expCount: 1,
		},

		// --- Space ordering ---
		"searches spaces in order": {
			raw: "bob",
			tt:  targetTypePlayer,
			spaces: []SearchSpace{
				{Finder: &mockFinder{}},
				{Finder: &mockFinder{players: map[string]*game.CharacterInstance{"bob": bob}}},
			},
			expType: targetTypeActor, expName: "Bob", expCount: 1,
		},
		"bare name stops at first space with match": {
			raw: "wolf",
			tt:  targetTypeMobile,
			spaces: []SearchSpace{
				{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1}}},
				{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-2": wolf2}}},
			},
			expType: targetTypeActor, expId: "wolf-1", expCount: 1,
		},

		// --- N.keyword indexing ---
		"2.wolf selects second match": {
			raw:     "2.wolf",
			tt:      targetTypeMobile,
			spaces:  []SearchSpace{{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1, "wolf-2": wolf2}}}},
			expType: targetTypeActor, expCount: 1,
		},
		"3.wolf not found with only 2": {
			raw:    "3.wolf",
			tt:     targetTypeMobile,
			spaces: []SearchSpace{{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1, "wolf-2": wolf2}}}},
			expErr: "not found",
		},
		"2.wolf across spaces": {
			raw: "2.wolf",
			tt:  targetTypeMobile,
			spaces: []SearchSpace{
				{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1}}},
				{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-2": wolf2}}},
			},
			expType: targetTypeActor, expId: "wolf-2", expCount: 1,
		},

		// --- all.keyword ---
		"all.wolf returns all wolves": {
			raw:     "all.wolf",
			tt:      targetTypeMobile,
			spaces:  []SearchSpace{{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1, "wolf-2": wolf2, "goblin-1": goblin}}}},
			expCount: 2,
		},
		"bare all returns everything": {
			raw:      "all",
			tt:       targetTypeMobile,
			spaces:   []SearchSpace{{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1, "wolf-2": wolf2, "goblin-1": goblin}}}},
			expCount: 3,
		},
		"all.dragon not found": {
			raw:    "all.dragon",
			tt:     targetTypeMobile,
			spaces: []SearchSpace{{Finder: &mockFinder{mobs: map[string]*game.MobileInstance{"wolf-1": wolf1}}}},
			expErr: "not found",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			results, err := FindTarget(tc.raw, tc.tt, tc.spaces)

			if tc.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.expErr)
				}
				if !strings.Contains(err.Error(), tc.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.expErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tc.expCount {
				t.Fatalf("expected %d results, got %d", tc.expCount, len(results))
			}
			if tc.expCount == 0 {
				return
			}
			r := results[0]
			if tc.expType != 0 && r.Type != tc.expType {
				t.Errorf("Type = %q, want %q", r.Type, tc.expType)
			}
			if tc.expName != "" {
				got := ""
				if r.Actor != nil {
					got = r.Actor.Name
				} else if r.Obj != nil {
					got = r.Obj.Name
				}
				if got != tc.expName {
					t.Errorf("Name = %q, want %q", got, tc.expName)
				}
			}
			if tc.expId != "" && r.Actor != nil && r.Actor.actor.Id() != tc.expId {
				t.Errorf("Id = %q, want %q", r.Actor.actor.Id(), tc.expId)
			}
			if tc.expSource && r.Obj != nil && r.Obj.source == nil {
				t.Error("source is nil, want non-nil")
			}
			if tc.expDir != "" && r.Exit != nil && r.Exit.Direction != tc.expDir {
				t.Errorf("Direction = %q, want %q", r.Exit.Direction, tc.expDir)
			}
		})
	}
}

// --- ResolveSpecs tests ---

func TestResolveSpecs(t *testing.T) {
	bob := &game.CharacterInstance{
		Character: storage.NewResolvedSmartIdentifier("bob", &assets.Character{Name: "Bob"}),
	}
	finder := &mockFinder{
		players: map[string]*game.CharacterInstance{"bob": bob},
	}

	tests := map[string]struct {
		specs      []assets.TargetSpec
		inputs     map[string]any
		spaces     []SearchSpace
		expTargets map[string]targetType
		expNil     []string
		expErr     string
	}{
		"empty specs returns empty map": {
			specs:      []assets.TargetSpec{},
			inputs:     map[string]any{},
			expTargets: map[string]targetType{},
		},
		"resolves player target": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs:     map[string]any{"who": "bob"},
			spaces:     []SearchSpace{{Finder: finder}},
			expTargets: map[string]targetType{"target": targetTypeActor},
		},
		"resolves multiple targets": {
			specs: []assets.TargetSpec{
				{Name: "first", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who1"},
				{Name: "second", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who2", Optional: true},
			},
			inputs:     map[string]any{"who1": "bob", "who2": ""},
			spaces:     []SearchSpace{{Finder: finder}},
			expTargets: map[string]targetType{"first": targetTypeActor},
			expNil:     []string{"second"},
		},
		"optional target with empty input": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who", Optional: true},
			},
			inputs: map[string]any{"who": ""},
			spaces: []SearchSpace{{Finder: finder}},
			expNil: []string{"target"},
		},
		"required target with empty input": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs: map[string]any{"who": ""},
			expErr: `Input "who" is required.`,
		},
		"not found": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs: map[string]any{"who": "nobody"},
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			expErr: `Player "nobody" not found.`,
		},
		"not found with custom message": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who", NotFound: "You don't see '{{ .Inputs.who }}' here."},
			},
			inputs: map[string]any{"who": "nobody"},
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			expErr: "You don't see 'nobody' here.",
		},
		"allow_unresolved with no match returns nil": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who", Optional: true, AllowUnresolved: true},
			},
			inputs: map[string]any{"who": "nobody"},
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			expNil: []string{"target"},
		},
		"allow_unresolved with match still resolves": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who", Optional: true, AllowUnresolved: true},
			},
			inputs:     map[string]any{"who": "bob"},
			spaces:     []SearchSpace{{Finder: finder}},
			expTargets: map[string]targetType{"target": targetTypeActor},
		},
		"all rejected without allow_all": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs: map[string]any{"who": "all.bob"},
			spaces: []SearchSpace{{Finder: finder}},
			expErr: "multiple",
		},
		"all permitted with allow_all": {
			specs: []assets.TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who", AllowAll: true},
			},
			inputs:     map[string]any{"who": "all.bob"},
			spaces:     []SearchSpace{{Finder: finder}},
			expTargets: map[string]targetType{"target": targetTypeActor},
		},
	}

	session := &gametest.BaseActor{ActorId: "actor", ActorName: "Actor"}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := NewTargetResolver(&mockScopes{spaces: tt.spaces})
			targets, err := resolver.ResolveSpecs(tt.specs, tt.inputs, session)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for tgtName, expType := range tt.expTargets {
				refs := targets[tgtName]
				if len(refs) == 0 {
					t.Errorf("target[%q] is empty, expected type %q", tgtName, expType)
					continue
				}
				if refs[0].Type != expType {
					t.Errorf("target[%q].Type = %q, expected %q", tgtName, refs[0].Type, expType)
				}
			}

			for _, nilName := range tt.expNil {
				if len(targets[nilName]) != 0 {
					t.Errorf("target[%q] should be empty", nilName)
				}
			}
		})
	}
}

func TestResolveSpecs_ScopeTarget(t *testing.T) {
	chestDef := &assets.Object{Aliases: []string{"chest"}, ShortDesc: "a wooden chest", Flags: []string{"container"}}
	torchDef := &assets.Object{Aliases: []string{"torch"}, ShortDesc: "a lit torch"}
	swordDef := &assets.Object{Aliases: []string{"sword"}, ShortDesc: "a rusty sword"}

	torch := &game.ObjectInstance{InstanceId: "torch-1", Object: storage.NewResolvedSmartIdentifier("torch", torchDef)}
	chestWithTorch := &game.ObjectInstance{
		InstanceId: "chest-1", Object: storage.NewResolvedSmartIdentifier("chest", chestDef),
		Contents: func() *game.Inventory {
			inv := game.NewInventory()
			inv.AddObj(torch)
			return inv
		}(),
	}
	emptyChest := &game.ObjectInstance{
		InstanceId: "chest-1", Object: storage.NewResolvedSmartIdentifier("chest", chestDef),
	}
	swordObj := &game.ObjectInstance{
		InstanceId: "sword-1", Object: storage.NewResolvedSmartIdentifier("sword", swordDef),
	}

	tests := map[string]struct {
		roomObjects []*game.ObjectInstance
		specs       []assets.TargetSpec
		inputs      map[string]any
		expTargets  map[string]targetType
		expNil      []string
		expErr      string
	}{
		"resolves from container contents": {
			roomObjects: []*game.ObjectInstance{chestWithTorch},
			specs: []assets.TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs:     map[string]any{"from": "chest", "item": "torch"},
			expTargets: map[string]targetType{"container": targetTypeObject, "target": targetTypeObject},
		},
		"falls back to room when container not provided": {
			roomObjects: []*game.ObjectInstance{swordObj},
			specs: []assets.TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs:     map[string]any{"from": "", "item": "sword"},
			expTargets: map[string]targetType{"target": targetTypeObject},
			expNil:     []string{"container"},
		},
		"restricts to contents when container resolved": {
			roomObjects: []*game.ObjectInstance{emptyChest, swordObj},
			specs: []assets.TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs: map[string]any{"from": "chest", "item": "sword"},
			expErr: `Object "sword" not found.`,
		},
		"empty container contents not found": {
			roomObjects: []*game.ObjectInstance{
				{InstanceId: "chest-2", Object: storage.NewResolvedSmartIdentifier("chest", chestDef),
					Contents: game.NewInventory()},
			},
			specs: []assets.TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs: map[string]any{"from": "chest", "item": "torch"},
			expErr: `Object "torch" not found.`,
		},
		"rejects non-container": {
			roomObjects: []*game.ObjectInstance{swordObj},
			specs: []assets.TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs: map[string]any{"from": "sword", "item": "torch"},
			expErr: `A rusty sword is not a container.`,
		},
	}

	session := &gametest.BaseActor{ActorId: "actor", ActorName: "Actor"}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Build a mock finder from room objects
			objMap := make(map[string]*game.ObjectInstance)
			for _, oi := range tt.roomObjects {
				objMap[oi.InstanceId] = oi
			}
			finder := &mockFinder{objects: objMap}
			scopes := &mockScopes{spaces: []SearchSpace{{Finder: finder}}}

			resolver := NewTargetResolver(scopes)
			targets, err := resolver.ResolveSpecs(tt.specs, tt.inputs, session)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for tgtName, expType := range tt.expTargets {
				refs := targets[tgtName]
				if len(refs) == 0 {
					t.Errorf("target[%q] is empty, expected type %q", tgtName, expType)
					continue
				}
				if refs[0].Type != expType {
					t.Errorf("target[%q].Type = %q, expected %q", tgtName, refs[0].Type, expType)
				}
			}

			for _, nilName := range tt.expNil {
				if len(targets[nilName]) != 0 {
					t.Errorf("target[%q] should be empty", nilName)
				}
			}
		})
	}
}

