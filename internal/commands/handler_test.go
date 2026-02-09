package commands

import (
	"errors"
	"testing"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestHandler_parseValue(t *testing.T) {
	h := &Handler{}

	tests := map[string]struct {
		inputType InputType
		raw       string
		exp       any
		expErr    string
	}{
		"string type": {
			inputType: InputTypeString,
			raw:       "hello world",
			exp:       "hello world",
		},
		"number type valid": {
			inputType: InputTypeNumber,
			raw:       "42",
			exp:       42,
		},
		"number type negative": {
			inputType: InputTypeNumber,
			raw:       "-10",
			exp:       -10,
		},
		"number type invalid": {
			inputType: InputTypeNumber,
			raw:       "abc",
			expErr:    `"abc" is not a valid number`,
		},
		"number type float rejected": {
			inputType: InputTypeNumber,
			raw:       "3.14",
			expErr:    `"3.14" is not a valid number`,
		},
		"unknown type": {
			inputType: InputType("bogus"),
			raw:       "test",
			expErr:    `unknown parameter type "bogus"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := h.parseValue(tt.inputType, tt.raw)

			if tt.expErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expErr)
					return
				}
				if err.Error() != tt.expErr {
					t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.exp {
				t.Errorf("got %v, expected %v", got, tt.exp)
			}
		})
	}
}

func TestHandler_parseInputs(t *testing.T) {
	h := &Handler{}

	tests := map[string]struct {
		specs   []InputSpec
		rawArgs []string
		exp     []ParsedInput
		expErr  string
	}{
		"no inputs no args": {
			specs:   nil,
			rawArgs: nil,
			exp:     []ParsedInput{},
		},
		"no inputs with args rejected": {
			specs:   nil,
			rawArgs: []string{"extra"},
			expErr:  "Expected at most 0 argument(s), got 1",
		},
		"required input missing": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
			},
			rawArgs: nil,
			expErr:  "Expected at least 1 argument(s), got 0",
		},
		"required input provided": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
			},
			rawArgs: []string{"5"},
			exp: []ParsedInput{
				{
					Spec:  &InputSpec{Name: "count", Type: InputTypeNumber, Required: true},
					Raw:   "5",
					Value: 5,
				},
			},
		},
		"optional input omitted": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: false},
			},
			rawArgs: nil,
			exp:     []ParsedInput{},
		},
		"optional input provided": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: false},
			},
			rawArgs: []string{"5"},
			exp: []ParsedInput{
				{
					Spec:  &InputSpec{Name: "count", Type: InputTypeNumber, Required: false},
					Raw:   "5",
					Value: 5,
				},
			},
		},
		"rest input captures remaining": {
			specs: []InputSpec{
				{Name: "text", Type: InputTypeString, Required: true, Rest: true},
			},
			rawArgs: []string{"hello", "world", "foo"},
			exp: []ParsedInput{
				{
					Spec:  &InputSpec{Name: "text", Type: InputTypeString, Required: true, Rest: true},
					Raw:   "hello world foo",
					Value: "hello world foo",
				},
			},
		},
		"mixed inputs with rest": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
				{Name: "message", Type: InputTypeString, Required: true, Rest: true},
			},
			rawArgs: []string{"3", "hello", "there", "friend"},
			exp: []ParsedInput{
				{
					Spec:  &InputSpec{Name: "count", Type: InputTypeNumber, Required: true},
					Raw:   "3",
					Value: 3,
				},
				{
					Spec:  &InputSpec{Name: "message", Type: InputTypeString, Required: true, Rest: true},
					Raw:   "hello there friend",
					Value: "hello there friend",
				},
			},
		},
		"too many args without rest": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
			},
			rawArgs: []string{"5", "extra", "args"},
			expErr:  "Expected at most 1 argument(s), got 3",
		},
		"number parse error": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
			},
			rawArgs: []string{"notanumber"},
			expErr:  `"notanumber" is not a valid number`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := h.parseInputs(tt.specs, tt.rawArgs)

			if tt.expErr != "" {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expErr)
					return
				}
				var userErr *UserError
				if errors.As(err, &userErr) {
					if userErr.Message != tt.expErr {
						t.Errorf("error = %q, expected %q", userErr.Message, tt.expErr)
					}
				} else if err.Error() != tt.expErr {
					t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(got) != len(tt.exp) {
				t.Errorf("returned %d inputs, expected %d", len(got), len(tt.exp))
				return
			}

			for i, input := range got {
				expected := tt.exp[i]
				if input.Raw != expected.Raw {
					t.Errorf("input[%d].Raw = %q, expected %q", i, input.Raw, expected.Raw)
				}
				if input.Value != expected.Value {
					t.Errorf("input[%d].Value = %v, expected %v", i, input.Value, expected.Value)
				}
				if input.Spec.Name != expected.Spec.Name {
					t.Errorf("input[%d].Spec.Name = %q, expected %q", i, input.Spec.Name, expected.Spec.Name)
				}
			}
		})
	}
}

type mockHandlerFactory struct{}

func (f *mockHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *mockHandlerFactory) Create() (CommandFunc, error) {
	return nil, nil
}

func TestHandler_RegisterFactory(t *testing.T) {
	dummyFactory := &mockHandlerFactory{}

	tests := map[string]struct {
		factoryFn  HandlerFactory
		regName    string
		preRegName string
		expErr     string
	}{
		"empty name": {
			factoryFn: dummyFactory,
			regName:   "",
			expErr:    "handler name cannot be empty",
		},
		"nil factory": {
			factoryFn: nil,
			regName:   "test",
			expErr:    "handler factory cannot be nil",
		},
		"duplicate registration": {
			factoryFn:  dummyFactory,
			regName:    "test",
			preRegName: "test",
			expErr:     `handler factory "test" already registered`,
		},
		"valid registration": {
			factoryFn: dummyFactory,
			regName:   "newhandler",
			expErr:    "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := &Handler{
				factories: make(map[string]HandlerFactory),
			}

			if tt.preRegName != "" {
				h.factories[tt.preRegName] = dummyFactory
			}

			err := h.RegisterFactory(tt.regName, tt.factoryFn)

			if tt.expErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error %q, got nil", tt.expErr)
				return
			}

			if err.Error() != tt.expErr {
				t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
			}
		})
	}
}

func TestHandler_resolveTargets(t *testing.T) {
	tests := map[string]struct {
		chars         map[string]*game.Character
		onlinePlayers map[string]struct{ zone, room storage.Identifier }
		actorZone     storage.Identifier
		actorRoom     storage.Identifier
		targetSpecs   []TargetSpec
		inputs        map[string]any
		expTargets    map[string]string // name -> expected resolved name
		expErr        string
	}{
		"resolve player in world scope": {
			chars: map[string]*game.Character{
				"bob": {Name: "Bob", Title: "the Great"},
			},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{
				"bob": {"zone1", "room1"},
			},
			actorZone: "zone2",
			actorRoom: "room2",
			targetSpecs: []TargetSpec{
				{Name: "target", Type: "player", Scopes: []string{"world"}, Input: "target"},
			},
			inputs: map[string]any{
				"target": "bob",
			},
			expTargets: map[string]string{
				"target": "Bob",
			},
		},
		"optional target with missing input": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "zone1",
			actorRoom:     "room1",
			targetSpecs: []TargetSpec{
				{Name: "target", Type: "target", Scopes: []string{"room"}, Input: "target", Optional: true},
			},
			inputs:     map[string]any{},
			expTargets: map[string]string{}, // target should be nil
		},
		"required target with missing input fails": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "zone1",
			actorRoom:     "room1",
			targetSpecs: []TargetSpec{
				{Name: "target", Type: "player", Scopes: []string{"world"}, Input: "target"},
			},
			inputs: map[string]any{},
			expErr: "Missing required input: target",
		},
		"player not found": {
			chars:         map[string]*game.Character{},
			onlinePlayers: map[string]struct{ zone, room storage.Identifier }{},
			actorZone:     "zone1",
			actorRoom:     "room1",
			targetSpecs: []TargetSpec{
				{Name: "target", Type: "player", Scopes: []string{"world"}, Input: "target"},
			},
			inputs: map[string]any{
				"target": "nobody",
			},
			expErr: "Player 'nobody' not found",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := &Handler{}

			charStore := &mockCharStore{chars: tt.chars}
			world := game.NewWorldState(
				nil,
				charStore,
				&mockZoneStore{zones: map[string]*game.Zone{}},
				&mockRoomStore{rooms: map[string]*game.Room{}},
				&mockMobileStore{mobiles: map[string]*game.Mobile{}},
				&mockObjectStore{objects: map[string]*game.Object{}},
			)

			// Add actor
			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("alice", actorChan, tt.actorZone, tt.actorRoom)

			// Add online players
			for charId, loc := range tt.onlinePlayers {
				ch := make(chan []byte, 1)
				_ = world.AddPlayer(storage.Identifier(charId), ch, loc.zone, loc.room)
			}

			targets, err := h.resolveTargets(tt.targetSpecs, tt.inputs, "alice", world)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				var userErr *UserError
				if errors.As(err, &userErr) {
					if userErr.Message != tt.expErr {
						t.Errorf("error = %q, expected %q", userErr.Message, tt.expErr)
					}
				} else if err.Error() != tt.expErr {
					t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveTargets failed: %v", err)
			}

			// Verify expected targets
			for name, expName := range tt.expTargets {
				target := targets[name]
				if target == nil {
					t.Errorf("target[%q] is nil, expected %q", name, expName)
					continue
				}
				if target.Name != expName {
					t.Errorf("target[%q].Name = %q, expected %q", name, target.Name, expName)
				}
			}

			// Check that optional missing targets are nil
			for _, spec := range tt.targetSpecs {
				if spec.Optional {
					if _, hasInput := tt.inputs[spec.Input]; !hasInput {
						if targets[spec.Name] != nil {
							t.Errorf("target[%q] should be nil for optional missing input", spec.Name)
						}
					}
				}
			}
		})
	}
}

func TestHandler_expandConfig(t *testing.T) {
	tests := map[string]struct {
		config    map[string]any
		actor     *game.Character
		targets   map[string]*TargetRef
		inputs    map[string]any
		expConfig map[string]string
		expErr    string
	}{
		"simple input substitution": {
			config: map[string]any{
				"message": "{{ .Actor.Name }} says, \"{{ .Inputs.text }}\"",
			},
			actor:   &game.Character{Name: "Alice"},
			targets: map[string]*TargetRef{},
			inputs: map[string]any{
				"text": "hello world",
			},
			expConfig: map[string]string{
				"message": `Alice says, "hello world"`,
			},
		},
		"target used in template": {
			config: map[string]any{
				"channel": "player-{{ .Targets.target.Name | lower }}",
				"message": "Hello {{ .Targets.target.Name }}!",
			},
			actor: &game.Character{Name: "Alice"},
			targets: map[string]*TargetRef{
				"target": {Type: "player", Name: "Bob", Player: &PlayerRef{Name: "Bob"}},
			},
			inputs: map[string]any{},
			expConfig: map[string]string{
				"channel": "player-bob",
				"message": "Hello Bob!",
			},
		},
		"actor and target combined": {
			config: map[string]any{
				"message": "{{ .Actor.Name }} tells {{ .Targets.target.Name }}, \"{{ .Inputs.text }}\"",
			},
			actor: &game.Character{Name: "Alice"},
			targets: map[string]*TargetRef{
				"target": {Type: "player", Name: "Bob", Player: &PlayerRef{Name: "Bob"}},
			},
			inputs: map[string]any{
				"text": "hello there",
			},
			expConfig: map[string]string{
				"message": `Alice tells Bob, "hello there"`,
			},
		},
		"static config value": {
			config: map[string]any{
				"direction": "north",
			},
			actor:   &game.Character{Name: "Alice"},
			targets: map[string]*TargetRef{},
			inputs:  map[string]any{},
			expConfig: map[string]string{
				"direction": "north",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := &Handler{}

			world := game.NewWorldState(
				nil,
				&mockCharStore{chars: map[string]*game.Character{}},
				&mockZoneStore{zones: map[string]*game.Zone{}},
				&mockRoomStore{rooms: map[string]*game.Room{}},
				&mockMobileStore{mobiles: map[string]*game.Mobile{}},
				&mockObjectStore{objects: map[string]*game.Object{}},
			)

			actorChan := make(chan []byte, 1)
			_ = world.AddPlayer("alice", actorChan, "zone1", "room1")
			actorState := world.GetPlayer("alice")

			expandedConfig, err := h.expandConfig(tt.config, tt.actor, actorState, tt.targets, tt.inputs)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if err.Error() != tt.expErr {
					t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("expandConfig failed: %v", err)
			}

			// Verify expected config values
			for key, expValue := range tt.expConfig {
				gotValue := expandedConfig[key]
				if gotValue != expValue {
					t.Errorf("config[%q] = %q, expected %q", key, gotValue, expValue)
				}
			}
		})
	}
}
