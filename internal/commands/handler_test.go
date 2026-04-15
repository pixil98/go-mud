package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/gametest"
)

func TestHandler_parseValue(t *testing.T) {
	tests := map[string]struct {
		inputType string
		raw       string
		exp       any
		expErr    string
	}{
		"string type": {
			inputType: assets.InputTypeString,
			raw:       "hello world",
			exp:       "hello world",
		},
		"number type valid": {
			inputType: assets.InputTypeNumber,
			raw:       "42",
			exp:       42,
		},
		"number type negative": {
			inputType: assets.InputTypeNumber,
			raw:       "-10",
			exp:       -10,
		},
		"number type invalid": {
			inputType: assets.InputTypeNumber,
			raw:       "abc",
			expErr:    `"abc" is not a valid number.`,
		},
		"number type float rejected": {
			inputType: assets.InputTypeNumber,
			raw:       "3.14",
			expErr:    `"3.14" is not a valid number.`,
		},
		"unknown type": {
			inputType: "bogus",
			raw:       "test",
			expErr:    `unknown parameter type "bogus"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseValue(tt.inputType, tt.raw)

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

	tests := map[string]struct {
		specs   []assets.InputSpec
		rawArgs []string
		exp     map[string]any
		expErr  string
	}{
		"no inputs no args": {
			specs:   nil,
			rawArgs: nil,
			exp:     map[string]any{},
		},
		"no inputs with args rejected": {
			specs:   nil,
			rawArgs: []string{"extra"},
			expErr:  "Expected at most 0 argument(s), got 1.",
		},
		"required input missing": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: true},
			},
			rawArgs: nil,
			expErr:  "Expected at least 1 argument(s), got 0.",
		},
		"required input provided": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: true},
			},
			rawArgs: []string{"5"},
			exp:     map[string]any{"count": 5},
		},
		"optional input omitted": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: false},
			},
			rawArgs: nil,
			exp:     map[string]any{"count": ""},
		},
		"optional input provided": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: false},
			},
			rawArgs: []string{"5"},
			exp:     map[string]any{"count": 5},
		},
		"rest input captures remaining": {
			specs: []assets.InputSpec{
				{Name: "text", Type: assets.InputTypeString, Required: true, Rest: true},
			},
			rawArgs: []string{"hello", "world", "foo"},
			exp:     map[string]any{"text": "hello world foo"},
		},
		"mixed inputs with rest": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: true},
				{Name: "message", Type: assets.InputTypeString, Required: true, Rest: true},
			},
			rawArgs: []string{"3", "hello", "there", "friend"},
			exp:     map[string]any{"count": 3, "message": "hello there friend"},
		},
		"too many args without rest": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: true},
			},
			rawArgs: []string{"5", "extra", "args"},
			expErr:  "Expected at most 1 argument(s), got 3.",
		},
		"number parse error": {
			specs: []assets.InputSpec{
				{Name: "count", Type: assets.InputTypeNumber, Required: true},
			},
			rawArgs: []string{"notanumber"},
			expErr:  `"notanumber" is not a valid number.`,
		},
		"required input missing with custom message": {
			specs: []assets.InputSpec{
				{Name: "item", Type: assets.InputTypeString, Required: true, Missing: "Get what?"},
			},
			rawArgs: nil,
			expErr:  "Get what?",
		},
		"required input missing custom message second arg": {
			specs: []assets.InputSpec{
				{Name: "item", Type: assets.InputTypeString, Required: true},
				{Name: "recipient", Type: assets.InputTypeString, Required: true, Missing: "Give to whom?"},
			},
			rawArgs: []string{"sword"},
			expErr:  "Give to whom?",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseInputs(tt.specs, tt.rawArgs)

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

			for key, expVal := range tt.exp {
				gotVal, ok := got[key]
				if !ok {
					t.Errorf("missing key %q in result", key)
					continue
				}
				if gotVal != expVal {
					t.Errorf("input[%q] = %v, expected %v", key, gotVal, expVal)
				}
			}
		})
	}
}

func TestHandler_resolve(t *testing.T) {
	mkCmd := func(priority int) *compiledCommand {
		return &compiledCommand{cmd: &assets.Command{Priority: priority}}
	}

	// delta has an alias "dd" — both keys share the same compiledCommand.
	deltaCmd := mkCmd(0)

	h := &Handler{
		compiled: map[string]*compiledCommand{
			"alpha": mkCmd(10),
			"apple": mkCmd(0),
			"beta":  mkCmd(5),
			"bat":   mkCmd(5),
			"gamma": mkCmd(0),
			"delta": deltaCmd,
			"dd":    deltaCmd,
		},
	}

	tests := map[string]struct {
		input  string
		expCmd *compiledCommand
		expErr string
	}{
		"exact match": {
			input:  "gamma",
			expCmd: h.compiled["gamma"],
		},
		"exact match case insensitive": {
			input:  "GAMMA",
			expCmd: h.compiled["gamma"],
		},
		"exact match wins over higher priority prefix": {
			input:  "apple",
			expCmd: h.compiled["apple"],
		},
		"prefix single match": {
			input:  "g",
			expCmd: h.compiled["gamma"],
		},
		"prefix with priority tiebreak": {
			input:  "a",
			expCmd: h.compiled["alpha"],
		},
		"prefix ambiguous same priority": {
			input:  "b",
			expErr: "Did you mean: bat, beta?",
		},
		"no match": {
			input:  "zzz",
			expErr: `Command "zzz" is unknown.`,
		},
		"alias exact match": {
			input:  "dd",
			expCmd: deltaCmd,
		},
		"alias case insensitive": {
			input:  "DD",
			expCmd: deltaCmd,
		},
		"primary name still works with alias": {
			input:  "delta",
			expCmd: deltaCmd,
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

			if got != tt.expCmd {
				t.Errorf("resolved to wrong command")
			}
		})
	}
}

type mockHandlerFactory struct{}

func (f *mockHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *mockHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

func (f *mockHandlerFactory) Create() (CommandFunc, error) {
	return nil, nil
}

func TestHandler_compile(t *testing.T) {
	factory := &mockHandlerFactory{}

	tests := map[string]struct {
		preCompile []struct {
			id  string
			cmd *assets.Command
		}
		id     string
		cmd    *assets.Command
		expErr string
		expIds []string // IDs expected in compiled map after success
	}{
		"basic command": {
			id:     "alpha",
			cmd:    &assets.Command{Handler: "mock"},
			expIds: []string{"alpha"},
		},
		"command with aliases": {
			id:     "northwest",
			cmd:    &assets.Command{Handler: "mock", Aliases: []string{"nw"}},
			expIds: []string{"northwest", "nw"},
		},
		"alias conflicts with existing command": {
			preCompile: []struct {
				id  string
				cmd *assets.Command
			}{
				{id: "aa", cmd: &assets.Command{Handler: "mock"}},
			},
			id:     "alpha",
			cmd:    &assets.Command{Handler: "mock", Aliases: []string{"aa"}},
			expErr: `alias "aa" conflicts`,
		},
		"alias conflicts with earlier alias": {
			preCompile: []struct {
				id  string
				cmd *assets.Command
			}{
				{id: "alpha", cmd: &assets.Command{Handler: "mock", Aliases: []string{"aa"}}},
			},
			id:     "beta",
			cmd:    &assets.Command{Handler: "mock", Aliases: []string{"aa"}},
			expErr: `alias "aa" conflicts`,
		},
		"command name conflicts with earlier alias": {
			preCompile: []struct {
				id  string
				cmd *assets.Command
			}{
				{id: "alpha", cmd: &assets.Command{Handler: "mock", Aliases: []string{"beta"}}},
			},
			id:     "beta",
			cmd:    &assets.Command{Handler: "mock"},
			expErr: `command "beta" conflicts`,
		},
		"unknown handler": {
			id:     "alpha",
			cmd:    &assets.Command{Handler: "nonexistent"},
			expErr: `unknown handler "nonexistent"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := &Handler{
				factories: map[string]HandlerFactory{"mock": factory},
				compiled:  make(map[string]*compiledCommand),
			}

			for _, pre := range tt.preCompile {
				if err := h.compile(pre.id, pre.cmd); err != nil {
					t.Fatalf("pre-compile %q failed: %v", pre.id, err)
				}
			}

			err := h.compile(tt.id, tt.cmd)

			if tt.expErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expErr)
				}
				if !strings.Contains(err.Error(), tt.expErr) {
					t.Errorf("error = %q, expected to contain %q", err.Error(), tt.expErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, expId := range tt.expIds {
				if _, ok := h.compiled[expId]; !ok {
					t.Errorf("expected %q in compiled map", expId)
				}
			}
		})
	}
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
		config    map[string]string
		actor     *assets.Character
		targets   map[string][]*TargetRef
		inputs    map[string]any
		expConfig map[string]string
		expErr    string
	}{
		"simple input substitution": {
			config: map[string]string{
				"message": "{{ .Actor.Name }} says, \"{{ .Inputs.text }}\"",
			},
			actor:   &assets.Character{Name: "Alice"},
			targets: map[string][]*TargetRef{},
			inputs: map[string]any{
				"text": "hello world",
			},
			expConfig: map[string]string{
				"message": `Alice says, "hello world"`,
			},
		},
		"target used in template": {
			config: map[string]string{
				"channel": "player-{{ .Targets.target.Actor.Name | lower }}",
				"message": "Hello {{ .Targets.target.Actor.Name }}!",
			},
			actor: &assets.Character{Name: "Alice"},
			targets: map[string][]*TargetRef{
				"target": {{Type: targetTypeActor, Actor: &ActorRef{Name: "Bob"}}},
			},
			inputs: map[string]any{},
			expConfig: map[string]string{
				"channel": "player-bob",
				"message": "Hello Bob!",
			},
		},
		"actor and target combined": {
			config: map[string]string{
				"message": "{{ .Actor.Name }} tells {{ .Targets.target.Actor.Name }}, \"{{ .Inputs.text }}\"",
			},
			actor: &assets.Character{Name: "Alice"},
			targets: map[string][]*TargetRef{
				"target": {{Type: targetTypeActor, Actor: &ActorRef{Name: "Bob"}}},
			},
			inputs: map[string]any{
				"text": "hello there",
			},
			expConfig: map[string]string{
				"message": `Alice tells Bob, "hello there"`,
			},
		},
		"static config value": {
			config: map[string]string{
				"direction": "north",
			},
			actor:   &assets.Character{Name: "Alice"},
			targets: map[string][]*TargetRef{},
			inputs:  map[string]any{},
			expConfig: map[string]string{
				"direction": "north",
			},
		},
		"color in template": {
			config: map[string]string{
				"message": "{{ .Color.Red }}hello{{ .Color.Reset }}",
			},
			actor:   &assets.Character{},
			targets: map[string][]*TargetRef{},
			inputs:  map[string]any{},
			expConfig: map[string]string{
				"message": "\033[31mhello\033[0m",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			h := &Handler{}

			actor := &gametest.BaseActor{ActorId: "alice", ActorName: tt.actor.Name}

			expandedConfig, err := h.expandConfig(tt.config, actor, tt.targets, tt.inputs)

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
		cmd    *assets.Command
		spec   *HandlerSpec
		expErr string
	}{
		"nil spec passes": {
			cmd:    &assets.Command{Handler: "test"},
			spec:   nil,
			expErr: "",
		},
		"empty spec passes": {
			cmd:    &assets.Command{Handler: "test"},
			spec:   &HandlerSpec{},
			expErr: "",
		},
		"missing required target": {
			cmd: &assets.Command{
				Handler: "test",
				Targets: []assets.TargetSpec{},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: targetTypeObject, Required: true},
				},
			},
			expErr: `missing required target "target"`,
		},
		"optional target missing is ok": {
			cmd: &assets.Command{
				Handler: "test",
				Targets: []assets.TargetSpec{},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: targetTypeObject, Required: false},
				},
			},
			expErr: "",
		},
		"wrong target type": {
			cmd: &assets.Command{
				Handler: "test",
				Targets: []assets.TargetSpec{
					{Name: "target", Types: []string{"player"}, Input: "target"},
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: targetTypeObject, Required: true},
				},
			},
			expErr: `target "target": expected type object, got player`,
		},
		"target type subset of spec is ok": {
			cmd: &assets.Command{
				Handler: "test",
				Targets: []assets.TargetSpec{
					{Name: "target", Types: []string{"object"}, Input: "target"},
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: targetTypePlayer | targetTypeMobile | targetTypeObject, Required: true},
				},
			},
			expErr: "",
		},
		"extra target not in spec": {
			cmd: &assets.Command{
				Handler: "test",
				Targets: []assets.TargetSpec{
					{Name: "target", Types: []string{"object"}, Input: "target"},
					{Name: "extra", Types: []string{"object"}, Input: "extra"},
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "target", Type: targetTypeObject, Required: true},
				},
			},
			expErr: `unknown target "extra"`,
		},
		"missing required config": {
			cmd: &assets.Command{
				Handler: "test",
				Config:  map[string]string{},
			},
			spec: &HandlerSpec{
				Config: []ConfigRequirement{
					{Name: "direction", Required: true},
				},
			},
			expErr: `missing required config key "direction"`,
		},
		"optional config missing is ok": {
			cmd: &assets.Command{
				Handler: "test",
				Config:  map[string]string{},
			},
			spec: &HandlerSpec{
				Config: []ConfigRequirement{
					{Name: "optional_key", Required: false},
				},
			},
			expErr: "",
		},
		"extra config not in spec": {
			cmd: &assets.Command{
				Handler: "test",
				Config: map[string]string{
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
			cmd: &assets.Command{
				Handler: "test",
				Targets: []assets.TargetSpec{
					{Name: "item", Types: []string{"object"}, Input: "item"},
					{Name: "recipient", Types: []string{"player"}, Input: "recipient"},
				},
				Config: map[string]string{
					"message": "hello",
				},
			},
			spec: &HandlerSpec{
				Targets: []TargetRequirement{
					{Name: "item", Type: targetTypeObject, Required: true},
					{Name: "recipient", Type: targetTypePlayer, Required: true},
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
