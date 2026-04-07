package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
	"github.com/pixil98/go-mud/internal/storage"
)

// --- Mock finders ---

// mockFinder implements TargetFinder using simple maps keyed by name.
type mockFinder struct {
	players map[string]*game.CharacterInstance
	mobs    map[string]*game.MobileInstance
	objects map[string]*game.ObjectInstance
	exits   map[string]assets.Exit
}

func (f *mockFinder) FindPlayer(name string) *game.CharacterInstance {
	for _, ps := range f.players {
		if ps.Character.Get().MatchName(name) {
			return ps
		}
	}
	return nil
}

func (f *mockFinder) FindMob(name string) *game.MobileInstance {
	for _, mi := range f.mobs {
		if mi.Mobile.Get() != nil && mi.Mobile.Get().MatchName(name) {
			return mi
		}
	}
	return nil
}

func (f *mockFinder) FindObj(name string) *game.ObjectInstance {
	for _, oi := range f.objects {
		if oi.Object.Get() != nil && oi.Object.Get().MatchName(name) {
			return oi
		}
	}
	return nil
}

func (f *mockFinder) FindExit(name string) (string, *assets.Exit) {
	name = strings.ToLower(name)
	if exit, ok := f.exits[name]; ok {
		return name, &exit
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

func (s *mockScopes) SpacesFor(scope, shared.Actor) ([]SearchSpace, error) {
	return s.spaces, nil
}

// --- FindTarget tests ---

func TestFindTarget_Player(t *testing.T) {
	bob := &game.CharacterInstance{
		Character: storage.NewResolvedSmartIdentifier("bob", &assets.Character{Name: "Bob"}),
	}

	tests := map[string]struct {
		spaces        []SearchSpace
		name          string
		expPlayerName string
		expErr        string
	}{
		"finds player": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.CharacterInstance{"bob": bob},
			}}},
			name:          "bob",
			expPlayerName: "Bob",
		},
		"case insensitive": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.CharacterInstance{"bob": bob},
			}}},
			name:          "BOB",
			expPlayerName: "Bob",
		},
		"not found": {
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			name:   "nobody",
			expErr: `Player "nobody" not found.`,
		},
		"empty spaces": {
			spaces: []SearchSpace{},
			name:   "bob",
			expErr: `Player "bob" not found.`,
		},
		"searches spaces in order": {
			spaces: []SearchSpace{
				{Finder: &mockFinder{}}, // empty
				{Finder: &mockFinder{players: map[string]*game.CharacterInstance{"bob": bob}}},
			},
			name:          "bob",
			expPlayerName: "Bob",
		},
		"first space match wins": {
			spaces: []SearchSpace{
				{Finder: &mockFinder{players: map[string]*game.CharacterInstance{"bob": bob}}},
				{Finder: &mockFinder{players: map[string]*game.CharacterInstance{
					"bob2": {Character: storage.NewResolvedSmartIdentifier("bob2", &assets.Character{Name: "Bob2"})},
				}}},
			},
			name:          "bob",
			expPlayerName: "Bob",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := FindTarget(tt.name, targetTypePlayer, tt.spaces)

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
			if result.Type != targetTypeActor {
				t.Errorf("Type = %q, expected %q", result.Type.String(), targetTypeActor.String())
			}
			if result.Actor.Name != tt.expPlayerName {
				t.Errorf("Name = %q, expected %q", result.Actor.Name, tt.expPlayerName)
			}
		})
	}
}

func TestFindTarget_Mobile(t *testing.T) {
	guard := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("guard", &assets.Mobile{Aliases: []string{"guard", "soldier"}, ShortDesc: "a burly guard"}),
		ActorInstance: game.ActorInstance{InstanceId: "guard-1"},
	}

	tests := map[string]struct {
		spaces     []SearchSpace
		name       string
		expMobName string
		expErr     string
	}{
		"finds mob": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				mobs: map[string]*game.MobileInstance{"guard-1": guard},
			}}},
			name:       "guard",
			expMobName: "a burly guard",
		},
		"matches by alias": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				mobs: map[string]*game.MobileInstance{"guard-1": guard},
			}}},
			name:       "soldier",
			expMobName: "a burly guard",
		},
		"not found": {
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			name:   "nobody",
			expErr: `Mobile "nobody" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := FindTarget(tt.name, targetTypeMobile, tt.spaces)

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
			if result.Type != targetTypeActor {
				t.Errorf("Type = %q, expected %q", result.Type.String(), targetTypeActor.String())
			}
			if result.Actor.Name != tt.expMobName {
				t.Errorf("Name = %q, expected %q", result.Actor.Name, tt.expMobName)
			}
		})
	}
}

func TestFindTarget_Object(t *testing.T) {
	sword := &game.ObjectInstance{
		InstanceId: "sword-1",
		Object:     storage.NewResolvedSmartIdentifier("sword", &assets.Object{Aliases: []string{"sword"}, ShortDesc: "a rusty sword"}),
	}

	tests := map[string]struct {
		spaces     []SearchSpace
		name       string
		expObjName string
		expSource  bool // true if Source should be non-nil
		expErr     string
	}{
		"finds object": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				objects: map[string]*game.ObjectInstance{"sword-1": sword},
			}}},
			name:       "sword",
			expObjName: "a rusty sword",
		},
		"nil remover sets nil source": {
			spaces: []SearchSpace{{
				Finder: &mockFinder{
					objects: map[string]*game.ObjectInstance{"sword-1": sword},
				},
			}},
			name:       "sword",
			expObjName: "a rusty sword",
		},
		"remover is set as source": {
			spaces: []SearchSpace{{
				Finder: &mockFinder{
					objects: map[string]*game.ObjectInstance{"sword-1": sword},
				},
				Remover: &mockRemover{},
			}},
			name:       "sword",
			expObjName: "a rusty sword",
			expSource:  true,
		},
		"not found": {
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			name:   "nothing",
			expErr: `Object "nothing" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := FindTarget(tt.name, targetTypeObject, tt.spaces)

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
			if result.Type != targetTypeObject {
				t.Errorf("Type = %q, expected %q", result.Type, "object")
			}
			if result.Obj.Name != tt.expObjName {
				t.Errorf("Name = %q, expected %q", result.Obj.Name, tt.expObjName)
			}
			if tt.expSource && result.Obj.source == nil {
				t.Error("Source is nil, expected non-nil")
			}
			if !tt.expSource && result.Obj.source != nil {
				t.Error("Source is non-nil, expected nil")
			}
		})
	}
}

func TestFindTarget_Combined(t *testing.T) {
	bob := &game.CharacterInstance{
		Character: storage.NewResolvedSmartIdentifier("bob", &assets.Character{Name: "Bob"}),
	}
	guard := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("guard", &assets.Mobile{Aliases: []string{"guard"}, ShortDesc: "a burly guard"}),
		ActorInstance: game.ActorInstance{InstanceId: "guard-1"},
	}

	tests := map[string]struct {
		spaces  []SearchSpace
		name    string
		expType targetType
		expErr  string
	}{
		"prefers player over mob": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.CharacterInstance{"bob": bob},
				mobs:    map[string]*game.MobileInstance{"guard-1": guard},
			}}},
			name:    "bob",
			expType: targetTypeActor,
		},
		"falls through to mob when no player": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				mobs: map[string]*game.MobileInstance{"guard-1": guard},
			}}},
			name:    "guard",
			expType: targetTypeActor,
		},
		"not found returns generic label": {
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			name:   "nobody",
			expErr: `Target "nobody" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := FindTarget(tt.name, targetTypePlayer|targetTypeMobile|targetTypeObject, tt.spaces)

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
			if result.Type != tt.expType {
				t.Errorf("Type = %q, expected %q", result.Type, tt.expType)
			}
		})
	}
}

func TestFindTarget_Exit(t *testing.T) {
	tests := map[string]struct {
		spaces       []SearchSpace
		name         string
		expDirection string
		expErr       string
	}{
		"finds exit": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				exits: map[string]assets.Exit{"north": {}},
			}}},
			name:         "north",
			expDirection: "north",
		},
		"case insensitive": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				exits: map[string]assets.Exit{"north": {}},
			}}},
			name:         "NORTH",
			expDirection: "north",
		},
		"not found": {
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			name:   "north",
			expErr: `Exit "north" not found.`,
		},
		"prefers object over exit": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				objects: map[string]*game.ObjectInstance{
					"door-1": {InstanceId: "door-1", Object: storage.NewResolvedSmartIdentifier("door", &assets.Object{Aliases: []string{"north"}, ShortDesc: "a door"})},
				},
				exits: map[string]assets.Exit{"north": {}},
			}}},
			name:         "north",
			expDirection: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			targetType := targetTypeExit
			if tt.expDirection == "" && tt.expErr == "" {
				// "prefers object over exit" case
				targetType = targetTypeObject | targetTypeExit
			}
			result, err := FindTarget(tt.name, targetType, tt.spaces)

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

			if tt.expDirection != "" {
				if result.Type != targetTypeExit {
					t.Errorf("Type = %q, expected %q", result.Type.String(), targetTypeExit.String())
				}
				if result.Exit.Direction != tt.expDirection {
					t.Errorf("Direction = %q, expected %q", result.Exit.Direction, tt.expDirection)
				}
			} else {
				// "prefers object over exit" — should resolve as object
				if result.Type != targetTypeObject {
					t.Errorf("Type = %q, expected %q", result.Type.String(), targetTypeObject.String())
				}
			}
		})
	}
}

func TestFindTarget_TypeFiltering(t *testing.T) {
	finder := &mockFinder{
		players: map[string]*game.CharacterInstance{
			"bob": {Character: storage.NewResolvedSmartIdentifier("bob", &assets.Character{Name: "Bob"})},
		},
		mobs: map[string]*game.MobileInstance{
			"bob-1": {Mobile: storage.NewResolvedSmartIdentifier("bob", &assets.Mobile{Aliases: []string{"bob"}, ShortDesc: "a mob named bob"}), ActorInstance: game.ActorInstance{InstanceId: "bob-1"}},
		},
	}
	spaces := []SearchSpace{{Finder: finder}}

	// When only mobile type is requested, player "bob" should be skipped
	result, err := FindTarget("bob", targetTypeMobile, spaces)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Type != targetTypeActor {
		t.Errorf("Type = %q, expected %q", result.Type.String(), targetTypeActor.String())
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
	}

	session := &mockActor{id: "actor", name: "Actor"}

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
				tgt := targets[tgtName]
				if tgt == nil {
					t.Errorf("target[%q] is nil, expected type %q", tgtName, expType)
					continue
				}
				if tgt.Type != expType {
					t.Errorf("target[%q].Type = %q, expected %q", tgtName, tgt.Type, expType)
				}
			}

			for _, nilName := range tt.expNil {
				if targets[nilName] != nil {
					t.Errorf("target[%q] should be nil", nilName)
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

	session := &mockActor{id: "actor", name: "Actor"}

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
				tgt := targets[tgtName]
				if tgt == nil {
					t.Errorf("target[%q] is nil, expected type %q", tgtName, expType)
					continue
				}
				if tgt.Type != expType {
					t.Errorf("target[%q].Type = %q, expected %q", tgtName, tgt.Type, expType)
				}
			}

			for _, nilName := range tt.expNil {
				if targets[nilName] != nil {
					t.Errorf("target[%q] should be nil", nilName)
				}
			}
		})
	}
}
