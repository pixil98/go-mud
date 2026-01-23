package commands

import (
	"errors"
	"testing"
)

func TestHandler_parseValue(t *testing.T) {
	h := &Handler{}

	tests := map[string]struct {
		paramType ParamType
		raw       string
		exp       any
		expErr    string
	}{
		"string type": {
			paramType: ParamTypeString,
			raw:       "hello world",
			exp:       "hello world",
		},
		"number type valid": {
			paramType: ParamTypeNumber,
			raw:       "42",
			exp:       42,
		},
		"number type negative": {
			paramType: ParamTypeNumber,
			raw:       "-10",
			exp:       -10,
		},
		"number type invalid": {
			paramType: ParamTypeNumber,
			raw:       "abc",
			expErr:    `"abc" is not a valid number`,
		},
		"number type float rejected": {
			paramType: ParamTypeNumber,
			raw:       "3.14",
			expErr:    `"3.14" is not a valid number`,
		},
		"unknown type": {
			paramType: ParamType("bogus"),
			raw:       "test",
			expErr:    `unknown parameter type "bogus"`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := h.parseValue(tt.paramType, tt.raw)

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

func TestHandler_parseArgs(t *testing.T) {
	h := &Handler{}

	tests := map[string]struct {
		specs   []ParamSpec
		rawArgs []string
		exp     []ParsedArg
		expErr  string
	}{
		"no params no args": {
			specs:   nil,
			rawArgs: nil,
			exp:     []ParsedArg{},
		},
		"no params with args rejected": {
			specs:   nil,
			rawArgs: []string{"extra"},
			expErr:  "Expected at most 0 argument(s), got 1",
		},
		"required param missing": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: true},
			},
			rawArgs: nil,
			expErr:  "Expected at least 1 argument(s), got 0",
		},
		"required param provided": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: true},
			},
			rawArgs: []string{"5"},
			exp: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "count", Type: ParamTypeNumber, Required: true},
					Raw:   "5",
					Value: 5,
				},
			},
		},
		"optional param omitted": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: false},
			},
			rawArgs: nil,
			exp:     []ParsedArg{},
		},
		"optional param provided": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: false},
			},
			rawArgs: []string{"5"},
			exp: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "count", Type: ParamTypeNumber, Required: false},
					Raw:   "5",
					Value: 5,
				},
			},
		},
		"rest param captures remaining": {
			specs: []ParamSpec{
				{Name: "text", Type: ParamTypeString, Required: true, Rest: true},
			},
			rawArgs: []string{"hello", "world", "foo"},
			exp: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "text", Type: ParamTypeString, Required: true, Rest: true},
					Raw:   "hello world foo",
					Value: "hello world foo",
				},
			},
		},
		"mixed params with rest": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: true},
				{Name: "message", Type: ParamTypeString, Required: true, Rest: true},
			},
			rawArgs: []string{"3", "hello", "there", "friend"},
			exp: []ParsedArg{
				{
					Spec:  &ParamSpec{Name: "count", Type: ParamTypeNumber, Required: true},
					Raw:   "3",
					Value: 3,
				},
				{
					Spec:  &ParamSpec{Name: "message", Type: ParamTypeString, Required: true, Rest: true},
					Raw:   "hello there friend",
					Value: "hello there friend",
				},
			},
		},
		"too many args without rest": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: true},
			},
			rawArgs: []string{"5", "extra", "args"},
			expErr:  "Expected at most 1 argument(s), got 3",
		},
		"number parse error": {
			specs: []ParamSpec{
				{Name: "count", Type: ParamTypeNumber, Required: true},
			},
			rawArgs: []string{"notanumber"},
			expErr:  `"notanumber" is not a valid number`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := h.parseArgs(tt.specs, tt.rawArgs)

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
				t.Errorf("returned %d args, expected %d", len(got), len(tt.exp))
				return
			}

			for i, arg := range got {
				expected := tt.exp[i]
				if arg.Raw != expected.Raw {
					t.Errorf("arg[%d].Raw = %q, expected %q", i, arg.Raw, expected.Raw)
				}
				if arg.Value != expected.Value {
					t.Errorf("arg[%d].Value = %v, expected %v", i, arg.Value, expected.Value)
				}
				if arg.Spec.Name != expected.Spec.Name {
					t.Errorf("arg[%d].Spec.Name = %q, expected %q", i, arg.Spec.Name, expected.Spec.Name)
				}
			}
		})
	}
}

func TestHandler_RegisterFactory(t *testing.T) {
	dummyFactory := func(cmd *Command) (CommandFunc, error) {
		return nil, nil
	}

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
