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
			expErr:    `"abc" is not a valid number.`,
		},
		"number type float rejected": {
			inputType: InputTypeNumber,
			raw:       "3.14",
			expErr:    `"3.14" is not a valid number.`,
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
			expErr:  "Expected at most 0 argument(s), got 1.",
		},
		"required input missing": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
			},
			rawArgs: nil,
			expErr:  "Expected at least 1 argument(s), got 0.",
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
			expErr:  "Expected at most 1 argument(s), got 3.",
		},
		"number parse error": {
			specs: []InputSpec{
				{Name: "count", Type: InputTypeNumber, Required: true},
			},
			rawArgs: []string{"notanumber"},
			expErr:  `"notanumber" is not a valid number.`,
		},
		"required input missing with custom message": {
			specs: []InputSpec{
				{Name: "item", Type: InputTypeString, Required: true, Missing: "Get what?"},
			},
			rawArgs: nil,
			expErr:  "Get what?",
		},
		"required input missing custom message second arg": {
			specs: []InputSpec{
				{Name: "item", Type: InputTypeString, Required: true},
				{Name: "recipient", Type: InputTypeString, Required: true, Missing: "Give to whom?"},
			},
			rawArgs: []string{"sword"},
			expErr:  "Give to whom?",
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

func TestHandler_resolve(t *testing.T) {
	mkCmd := func(priority int) *compiledCommand {
		return &compiledCommand{cmd: &Command{Priority: priority}}
	}

	h := &Handler{
		compiled: map[storage.Identifier]*compiledCommand{
			"alpha": mkCmd(10),
			"apple": mkCmd(0),
			"beta":  mkCmd(5),
			"bat":   mkCmd(5),
			"gamma": mkCmd(0),
		},
	}

	tests := map[string]struct {
		input  string
		expId  storage.Identifier
		expErr string
	}{
		"exact match": {
			input: "gamma",
			expId: "gamma",
		},
		"exact match case insensitive": {
			input: "GAMMA",
			expId: "gamma",
		},
		"exact match wins over higher priority prefix": {
			input: "apple",
			expId: "apple",
		},
		"prefix single match": {
			input: "g",
			expId: "gamma",
		},
		"prefix with priority tiebreak": {
			input: "a",
			expId: "alpha",
		},
		"prefix ambiguous same priority": {
			input:  "b",
			expErr: "Did you mean: bat, beta?",
		},
		"no match": {
			input:  "zzz",
			expErr: `Command "zzz" is unknown.`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := h.resolve(tt.input)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expErr)
				}
				var userErr *UserError
				if errors.As(err, &userErr) {
					if userErr.Message != tt.expErr {
						t.Errorf("error = %q, expected %q", userErr.Message, tt.expErr)
					}
				} else {
					t.Errorf("expected UserError, got %T: %v", err, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Find which ID maps to the returned compiled command.
			var foundId storage.Identifier
			for id, cc := range h.compiled {
				if cc == got {
					foundId = id
					break
				}
			}
			if foundId != tt.expId {
				t.Errorf("resolved to %q, expected %q", foundId, tt.expId)
			}
		})
	}
}

type mockHandlerFactory struct{}

func (f *mockHandlerFactory) Spec() *HandlerSpec {
	return nil
}

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
				"channel": "player-{{ .Targets.target.Player.Name | lower }}",
				"message": "Hello {{ .Targets.target.Player.Name }}!",
			},
			actor: &game.Character{Name: "Alice"},
			targets: map[string]*TargetRef{
				"target": {Type: TargetTypePlayer, Player: &PlayerRef{Name: "Bob"}},
			},
			inputs: map[string]any{},
			expConfig: map[string]string{
				"channel": "player-bob",
				"message": "Hello Bob!",
			},
		},
		"actor and target combined": {
			config: map[string]any{
				"message": "{{ .Actor.Name }} tells {{ .Targets.target.Player.Name }}, \"{{ .Inputs.text }}\"",
			},
			actor: &game.Character{Name: "Alice"},
			targets: map[string]*TargetRef{
				"target": {Type: TargetTypePlayer, Player: &PlayerRef{Name: "Bob"}},
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

			session := &game.PlayerState{CharId: "alice", Character: tt.actor}

			expandedConfig, err := h.expandConfig(tt.config, tt.actor, session, tt.targets, tt.inputs)

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

func TestHandler_validateSpec(t *testing.T) {
	tests := map[string]struct {
		cmd    *Command
		spec   *HandlerSpec
		expErr string
	}{
		"nil spec passes": {
			cmd:    &Command{Handler: "test"},
			spec:   nil,
			expErr: "",
		},
		"empty spec passes": {
			cmd:    &Command{Handler: "test"},
			spec:   &HandlerSpec{},
			expErr: "",
		},
		"missing required target": {
			cmd: &Command{
				Handler: "test",
				Targets: []TargetSpec{},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: TargetTypeObject, Required: true},
				},
			},
			expErr: `missing required target "target"`,
		},
		"optional target missing is ok": {
			cmd: &Command{
				Handler: "test",
				Targets: []TargetSpec{},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: TargetTypeObject, Required: false},
				},
			},
			expErr: "",
		},
		"wrong target type": {
			cmd: &Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "target", Types: []string{"player"}, Input: "target"},
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: TargetTypeObject, Required: true},
				},
			},
			expErr: `target "target": expected type object, got player`,
		},
		"target type subset of spec is ok": {
			cmd: &Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "target", Types: []string{"object"}, Input: "target"},
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: TargetTypePlayer | TargetTypeMobile | TargetTypeObject, Required: true},
				},
			},
			expErr: "",
		},
		"extra target not in spec": {
			cmd: &Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "target", Types: []string{"object"}, Input: "target"},
					{Name: "extra", Types: []string{"object"}, Input: "extra"},
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: TargetTypeObject, Required: true},
				},
			},
			expErr: `unknown target "extra"`,
		},
		"missing required config": {
			cmd: &Command{
				Handler: "test",
				Config:  map[string]any{},
			},
			spec: &HandlerSpec{
				Config: []ConfigRequirement{
					{Name: "direction", Required: true},
				},
			},
			expErr: `missing required config key "direction"`,
		},
		"optional config missing is ok": {
			cmd: &Command{
				Handler: "test",
				Config:  map[string]any{},
			},
			spec: &HandlerSpec{
				Config: []ConfigRequirement{
					{Name: "optional_key", Required: false},
				},
			},
			expErr: "",
		},
		"extra config not in spec": {
			cmd: &Command{
				Handler: "test",
				Config: map[string]any{
					"direction": "north",
					"typo_key":  "value",
				},
			},
			spec: &HandlerSpec{
				Config: []ConfigRequirement{
					{Name: "direction", Required: true},
				},
			},
			expErr: `unknown config key "typo_key"`,
		},
		"valid targets and config": {
			cmd: &Command{
				Handler: "test",
				Targets: []TargetSpec{
					{Name: "item", Types: []string{"object"}, Input: "item"},
					{Name: "recipient", Types: []string{"player"}, Input: "recipient"},
				},
				Config: map[string]any{
					"message": "hello",
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "item", Type: TargetTypeObject, Required: true},
					{Name: "recipient", Type: TargetTypePlayer, Required: true},
				},
				Config: []ConfigRequirement{
					{Name: "message", Required: true},
				},
			},
			expErr: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := &Handler{}

			// Skip nil spec
			if tt.spec == nil {
				return
			}

			err := h.validateSpec(tt.cmd, tt.spec)

			if tt.expErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.expErr)
				return
			}

			if err.Error() != tt.expErr {
				t.Errorf("error = %q, expected %q", err.Error(), tt.expErr)
			}
		})
	}
}
