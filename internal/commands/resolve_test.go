package commands

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// --- Mock finders ---

// mockFinder implements TargetFinder using simple maps keyed by name.
type mockFinder struct {
	players map[string]*game.PlayerState
	mobs    map[string]*game.MobileInstance
	objects map[string]*game.ObjectInstance
}

func (f *mockFinder) FindPlayer(name string) *game.PlayerState {
	for _, ps := range f.players {
		if ps.Character != nil && ps.Character.MatchName(name) {
			return ps
		}
	}
	return nil
}

func (f *mockFinder) FindMob(name string) *game.MobileInstance {
	for _, mi := range f.mobs {
		if mi.Mobile.Id() != nil && mi.Mobile.Id().MatchName(name) {
			return mi
		}
	}
	return nil
}

func (f *mockFinder) FindObj(name string) *game.ObjectInstance {
	for _, oi := range f.objects {
		if oi.Object.Id() != nil && oi.Object.Id().MatchName(name) {
			return oi
		}
	}
	return nil
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

func (s *mockScopes) SpacesFor(Scope, *game.Character, *game.PlayerState) ([]SearchSpace, error) {
	return s.spaces, nil
}

// --- FindTarget tests ---

func TestFindTarget_Player(t *testing.T) {
	bob := &game.PlayerState{
		CharId:    "bob",
		Character: &game.Character{Name: "Bob"},
	}

	tests := map[string]struct {
		spaces        []SearchSpace
		name          string
		expPlayerName string
		expErr        string
	}{
		"finds player": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.PlayerState{"bob": bob},
			}}},
			name:          "bob",
			expPlayerName: "Bob",
		},
		"case insensitive": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.PlayerState{"bob": bob},
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
				{Finder: &mockFinder{players: map[string]*game.PlayerState{"bob": bob}}},
			},
			name:          "bob",
			expPlayerName: "Bob",
		},
		"first space match wins": {
			spaces: []SearchSpace{
				{Finder: &mockFinder{players: map[string]*game.PlayerState{"bob": bob}}},
				{Finder: &mockFinder{players: map[string]*game.PlayerState{
					"bob2": {CharId: "bob2", Character: &game.Character{Name: "Bob2"}},
				}}},
			},
			name:          "bob",
			expPlayerName: "Bob",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := FindTarget(tt.name, TargetTypePlayer, tt.spaces)

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
			if result.Type != TargetTypePlayer {
				t.Errorf("Type = %q, expected %q", result.Type.String(), TargetTypePlayer.String())
			}
			if result.Player.Name != tt.expPlayerName {
				t.Errorf("Name = %q, expected %q", result.Player.Name, tt.expPlayerName)
			}
		})
	}
}

func TestFindTarget_Mobile(t *testing.T) {
	guard := &game.MobileInstance{
		InstanceId: "guard-1",
		Mobile:     storage.NewResolvedSmartIdentifier("guard", &game.Mobile{Aliases: []string{"guard", "soldier"}, ShortDesc: "a burly guard"}),
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
			result, err := FindTarget(tt.name, TargetTypeMobile, tt.spaces)

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
			if result.Type != TargetTypeMobile {
				t.Errorf("Type = %q, expected %q", result.Type.String(), TargetTypeMobile.String())
			}
			if result.Mob.Name != tt.expMobName {
				t.Errorf("Name = %q, expected %q", result.Mob.Name, tt.expMobName)
			}
		})
	}
}

func TestFindTarget_Object(t *testing.T) {
	sword := &game.ObjectInstance{
		InstanceId: "sword-1",
		Object:     storage.NewResolvedSmartIdentifier("sword", &game.Object{Aliases: []string{"sword"}, ShortDesc: "a rusty sword"}),
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
			result, err := FindTarget(tt.name, TargetTypeObject, tt.spaces)

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
			if result.Type != TargetTypeObject {
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
	bob := &game.PlayerState{
		CharId:    "bob",
		Character: &game.Character{Name: "Bob"},
	}
	guard := &game.MobileInstance{
		InstanceId: "guard-1",
		Mobile:     storage.NewResolvedSmartIdentifier("guard", &game.Mobile{Aliases: []string{"guard"}, ShortDesc: "a burly guard"}),
	}

	tests := map[string]struct {
		spaces  []SearchSpace
		name    string
		expType TargetType
		expErr  string
	}{
		"prefers player over mob": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				players: map[string]*game.PlayerState{"bob": bob},
				mobs:    map[string]*game.MobileInstance{"guard-1": guard},
			}}},
			name:    "bob",
			expType: TargetTypePlayer,
		},
		"falls through to mob when no player": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				mobs: map[string]*game.MobileInstance{"guard-1": guard},
			}}},
			name:    "guard",
			expType: TargetTypeMobile,
		},
		"not found returns generic label": {
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			name:   "nobody",
			expErr: `Target "nobody" not found.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := FindTarget(tt.name, TargetTypePlayer|TargetTypeMobile|TargetTypeObject, tt.spaces)

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

func TestFindTarget_TypeFiltering(t *testing.T) {
	finder := &mockFinder{
		players: map[string]*game.PlayerState{
			"bob": {CharId: "bob", Character: &game.Character{Name: "Bob"}},
		},
		mobs: map[string]*game.MobileInstance{
			"bob-1": {InstanceId: "bob-1", Mobile: storage.NewResolvedSmartIdentifier("bob", &game.Mobile{Aliases: []string{"bob"}, ShortDesc: "a mob named bob"})},
		},
	}
	spaces := []SearchSpace{{Finder: finder}}

	// When only mobile type is requested, player "bob" should be skipped
	result, err := FindTarget("bob", TargetTypeMobile, spaces)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Type != TargetTypeMobile {
		t.Errorf("Type = %q, expected %q", result.Type.String(), TargetTypeMobile.String())
	}
}

// --- ResolveSpecs tests ---

func TestResolveSpecs(t *testing.T) {
	bob := &game.PlayerState{
		CharId:    "bob",
		Character: &game.Character{Name: "Bob"},
	}
	finder := &mockFinder{
		players: map[string]*game.PlayerState{"bob": bob},
	}

	tests := map[string]struct {
		specs      []TargetSpec
		inputs     map[string]any
		spaces     []SearchSpace
		expTargets map[string]TargetType
		expNil     []string
		expErr     string
	}{
		"empty specs returns empty map": {
			specs:      []TargetSpec{},
			inputs:     map[string]any{},
			expTargets: map[string]TargetType{},
		},
		"resolves player target": {
			specs: []TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs:     map[string]any{"who": "bob"},
			spaces:     []SearchSpace{{Finder: finder}},
			expTargets: map[string]TargetType{"target": TargetTypePlayer},
		},
		"resolves multiple targets": {
			specs: []TargetSpec{
				{Name: "first", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who1"},
				{Name: "second", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who2", Optional: true},
			},
			inputs:     map[string]any{"who1": "bob", "who2": ""},
			spaces:     []SearchSpace{{Finder: finder}},
			expTargets: map[string]TargetType{"first": TargetTypePlayer},
			expNil:     []string{"second"},
		},
		"optional target with empty input": {
			specs: []TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who", Optional: true},
			},
			inputs: map[string]any{"who": ""},
			spaces: []SearchSpace{{Finder: finder}},
			expNil: []string{"target"},
		},
		"required target with empty input": {
			specs: []TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs: map[string]any{"who": ""},
			expErr: `Input "who" is required.`,
		},
		"not found": {
			specs: []TargetSpec{
				{Name: "target", Types: []string{"player"}, Scopes: []string{"world"}, Input: "who"},
			},
			inputs: map[string]any{"who": "nobody"},
			spaces: []SearchSpace{{Finder: &mockFinder{}}},
			expErr: `Player "nobody" not found.`,
		},
	}

	actor := &game.Character{Name: "Actor"}
	session := &game.PlayerState{CharId: "actor", Character: actor}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resolver := NewTargetResolver(&mockScopes{spaces: tt.spaces})
			targets, err := resolver.ResolveSpecs(tt.specs, tt.inputs, actor, session)

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
	chestDef := &game.Object{Aliases: []string{"chest"}, ShortDesc: "a wooden chest", Flags: []string{"container"}}
	torchDef := &game.Object{Aliases: []string{"torch"}, ShortDesc: "a lit torch"}
	swordDef := &game.Object{Aliases: []string{"sword"}, ShortDesc: "a rusty sword"}

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
		specs       []TargetSpec
		inputs      map[string]any
		expTargets  map[string]TargetType
		expNil      []string
		expErr      string
	}{
		"resolves from container contents": {
			roomObjects: []*game.ObjectInstance{chestWithTorch},
			specs: []TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs:     map[string]any{"from": "chest", "item": "torch"},
			expTargets: map[string]TargetType{"container": TargetTypeObject, "target": TargetTypeObject},
		},
		"falls back to room when container not provided": {
			roomObjects: []*game.ObjectInstance{swordObj},
			specs: []TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs:     map[string]any{"from": "", "item": "sword"},
			expTargets: map[string]TargetType{"target": TargetTypeObject},
			expNil:     []string{"container"},
		},
		"restricts to contents when container resolved": {
			roomObjects: []*game.ObjectInstance{emptyChest, swordObj},
			specs: []TargetSpec{
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
			specs: []TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs: map[string]any{"from": "chest", "item": "torch"},
			expErr: `Object "torch" not found.`,
		},
		"rejects non-container": {
			roomObjects: []*game.ObjectInstance{swordObj},
			specs: []TargetSpec{
				{Name: "container", Types: []string{"object"}, Scopes: []string{"room"}, Input: "from", Optional: true},
				{Name: "target", Types: []string{"object"}, Scopes: []string{"room", "contents"}, Input: "item", ScopeTarget: "container"},
			},
			inputs: map[string]any{"from": "sword", "item": "torch"},
			expErr: `A rusty sword is not a container.`,
		},
	}

	actor := &game.Character{Name: "Actor"}
	session := &game.PlayerState{CharId: "actor", Character: actor}

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
			targets, err := resolver.ResolveSpecs(tt.specs, tt.inputs, actor, session)

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
