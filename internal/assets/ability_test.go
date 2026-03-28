package assets

import (
	"strings"
	"testing"
)

func TestAbility_Help(t *testing.T) {
	tests := map[string]struct {
		ability  Ability
		name     string
		contains []string
	}{
		"with ap and mana cost": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect"}},
				Command: Command{
					Description: "test description",
					Config: map[string]string{
						"ap_cost":       "2",
						"resource":      "mana",
						"resource_cost": "15",
					},
					Inputs: []InputSpec{{Name: "target", Type: InputTypeString, Required: true}},
				},
			},
			name: "test-ability",
			contains: []string{
				"TEST-ABILITY - test description",
				"Usage: test-ability <target>",
				"Cost: 2 AP, 15 mana",
			},
		},
		"ap only": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect"}},
				Command: Command{
					Description: "test description",
					Config:      map[string]string{"ap_cost": "1"},
				},
			},
			name: "test-ability",
			contains: []string{
				"Cost: 1 AP",
			},
		},
		"no cost": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect"}},
				Command: Command{
					Description: "test description",
					Config:      map[string]string{},
				},
			},
			name: "test-ability",
			contains: []string{
				"TEST-ABILITY - test description",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.ability.Help(tc.name)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("Help() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestAbility_Validate(t *testing.T) {
	tests := map[string]struct {
		ability Ability
		expErr  string
	}{
		"valid ability with targets": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect"}},
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString, Required: true}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Scopes: []string{ScopeRoom}, Input: "test-target"}},
				},
			},
		},
		"valid ability with effect config": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect", Config: map[string]string{"test-key": "test-value"}}},
				Command: Command{
					Inputs: []InputSpec{{Name: "test-target", Type: InputTypeString, Required: true}},
					Targets: []TargetSpec{{
						Name:     "test-target",
						Types:    []string{TargetMobile, TargetPlayer},
						Scopes:   []string{ScopeRoom},
						Input:    "test-target",
						NotFound: "You don't see '{{ .Inputs.target }}' here.",
					}},
				},
			},
		},
		"missing effects": {
			ability: Ability{
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
			},
			expErr: "at least one effect is required",
		},
		"effect missing type": {
			ability: Ability{
				Effects: []EffectSpec{{}},
				Command: Command{},
			},
			expErr: "effect 0: type is required",
		},
		"command validation propagates": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect"}},
				Command: Command{
					Inputs: []InputSpec{{Type: InputTypeString}},
				},
			},
			expErr: "command: input 0: name is required",
		},
		"command with no inputs or targets is valid": {
			ability: Ability{
				Effects: []EffectSpec{{Type: "test-effect"}},
				Command: Command{},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.ability.Validate()

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

			if !strings.Contains(err.Error(), tt.expErr) {
				t.Errorf("error = %q, expected to contain %q", err.Error(), tt.expErr)
			}
		})
	}
}
