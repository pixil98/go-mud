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
			results, err := FindTarget(tt.name, targetTypePlayer, tt.spaces)

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
			if len(results) == 0 {
				t.Fatal("expected at least one result")
			}
			result := results[0]
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
			results, err := FindTarget(tt.name, targetTypeMobile, tt.spaces)

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
			if len(results) == 0 {
				t.Fatal("expected at least one result")
			}
			result := results[0]
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
			results, err := FindTarget(tt.name, targetTypeObject, tt.spaces)

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
			if len(results) == 0 {
				t.Fatal("expected at least one result")
			}
			result := results[0]
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
			results, err := FindTarget(tt.name, targetTypePlayer|targetTypeMobile|targetTypeObject, tt.spaces)

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
			if len(results) == 0 {
				t.Fatal("expected at least one result")
			}
			result := results[0]
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
				exits: map[string]*game.ResolvedExit{"north": {}},
			}}},
			name:         "north",
			expDirection: "north",
		},
		"case insensitive": {
			spaces: []SearchSpace{{Finder: &mockFinder{
				exits: map[string]*game.ResolvedExit{"north": {}},
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
				exits: map[string]*game.ResolvedExit{"north": {}},
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
			results, err := FindTarget(tt.name, targetType, tt.spaces)

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
			if len(results) == 0 {
				t.Fatal("expected at least one result")
			}
			result := results[0]

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
	results, err := FindTarget("bob", targetTypeMobile, spaces)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	result := results[0]
	if result.Type != targetTypeActor {
		t.Errorf("Type = %q, expected %q", result.Type.String(), targetTypeActor.String())
	}
}

// --- Prefix parsing tests ---

func TestParseTargetPrefix(t *testing.T) {
	tests := map[string]struct {
		raw       string
		wantName  string
		wantIndex int
		wantAll   bool
	}{
		"bare name":                       {raw: "wolf", wantName: "wolf", wantIndex: 1},
		"indexed":                         {raw: "2.wolf", wantName: "wolf", wantIndex: 2},
		"all with name":                   {raw: "all.wolf", wantName: "wolf", wantAll: true},
		"bare all":                        {raw: "all", wantAll: true},
		"all case insensitive":            {raw: "ALL.sword", wantName: "sword", wantAll: true},
		"non-numeric prefix stays as name": {raw: "mr.smith", wantName: "mr.smith", wantIndex: 1},
		"zero index treated as name":      {raw: "0.wolf", wantName: "0.wolf", wantIndex: 1},
		"negative index treated as name":  {raw: "-1.wolf", wantName: "-1.wolf", wantIndex: 1},
		"large index":                     {raw: "99.wolf", wantName: "wolf", wantIndex: 99},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gotName, gotIndex, gotAll := parseTargetPrefix(tc.raw)
			if gotName != tc.wantName {
				t.Errorf("name = %q, want %q", gotName, tc.wantName)
			}
			if gotIndex != tc.wantIndex {
				t.Errorf("index = %d, want %d", gotIndex, tc.wantIndex)
			}
			if gotAll != tc.wantAll {
				t.Errorf("all = %v, want %v", gotAll, tc.wantAll)
			}
		})
	}
}

func TestFindTarget_IndexedPrefix(t *testing.T) {
	mob1 := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("wolf-a", &assets.Mobile{Aliases: []string{"wolf"}, ShortDesc: "a grey wolf"}),
		ActorInstance: game.ActorInstance{InstanceId: "wolf-1"},
	}
	mob2 := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("wolf-b", &assets.Mobile{Aliases: []string{"wolf"}, ShortDesc: "a black wolf"}),
		ActorInstance: game.ActorInstance{InstanceId: "wolf-2"},
	}
	finder := &mockFinder{
		mobs: map[string]*game.MobileInstance{"wolf-1": mob1, "wolf-2": mob2},
	}
	spaces := []SearchSpace{{Finder: finder}}

	t.Run("bare name returns one", func(t *testing.T) {
		results, err := FindTarget("wolf", targetTypeMobile, spaces)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("2.wolf returns one", func(t *testing.T) {
		results, err := FindTarget("2.wolf", targetTypeMobile, spaces)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("3.wolf not found", func(t *testing.T) {
		_, err := FindTarget("3.wolf", targetTypeMobile, spaces)
		if err == nil {
			t.Fatal("expected error for 3.wolf with only 2 wolves")
		}
	})
}

func TestFindTarget_AllPrefix(t *testing.T) {
	mob1 := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("wolf-a", &assets.Mobile{Aliases: []string{"wolf"}, ShortDesc: "a grey wolf"}),
		ActorInstance: game.ActorInstance{InstanceId: "wolf-1"},
	}
	mob2 := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("wolf-b", &assets.Mobile{Aliases: []string{"wolf"}, ShortDesc: "a black wolf"}),
		ActorInstance: game.ActorInstance{InstanceId: "wolf-2"},
	}
	goblin := &game.MobileInstance{
		Mobile:        storage.NewResolvedSmartIdentifier("goblin-a", &assets.Mobile{Aliases: []string{"goblin"}, ShortDesc: "a goblin"}),
		ActorInstance: game.ActorInstance{InstanceId: "goblin-1"},
	}
	finder := &mockFinder{
		mobs: map[string]*game.MobileInstance{"wolf-1": mob1, "wolf-2": mob2, "goblin-1": goblin},
	}
	spaces := []SearchSpace{{Finder: finder}}

	t.Run("all.wolf returns both wolves", func(t *testing.T) {
		results, err := FindTarget("all.wolf", targetTypeMobile, spaces)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("bare all returns everything", func(t *testing.T) {
		results, err := FindTarget("all", targetTypeMobile, spaces)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
	})

	t.Run("all.dragon not found", func(t *testing.T) {
		_, err := FindTarget("all.dragon", targetTypeMobile, spaces)
		if err == nil {
			t.Fatal("expected error for all.dragon with no dragons")
		}
	})
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
