package assets

import (
	"strings"
	"testing"
)

func TestAbility_Validate(t *testing.T) {
	tests := map[string]struct {
		ability Ability
		expErr  string
	}{
		"valid spell": {
			ability: Ability{
				Name:    "test-spell",
				Type:    AbilityTypeSpell,
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString, Required: true}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Scopes: []string{ScopeRoom}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
		},
		"valid skill": {
			ability: Ability{
				Name:    "test-skill",
				Type:    AbilityTypeSkill,
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString, Required: true}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Scopes: []string{ScopeRoom}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
		},
		"valid spell with resource and ap cost": {
			ability: Ability{
				Name:         "test-spell",
				Type:         AbilityTypeSpell,
				Handler:      "test-handler",
				Resource:     "test-resource",
				ResourceCost: 10,
				APCost:       2,
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
				Config: map[string]any{
					"test-key": "test-value",
				},
				Messages: AbilityMessages{
					Actor:  "test-actor-message {{ .Target.Name }}",
					Target: "test-target-message {{ .Actor.Name }}",
					Room:   "test-room-message {{ .Actor.Name }}",
				},
			},
		},
		"missing name": {
			ability: Ability{
				Type:    AbilityTypeSpell,
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "name is required",
		},
		"missing type": {
			ability: Ability{
				Name:    "test-ability",
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "type is required",
		},
		"invalid type": {
			ability: Ability{
				Name:    "test-ability",
				Type:    "bogus",
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: `type must be "spell" or "skill"`,
		},
		"missing handler": {
			ability: Ability{
				Name: "test-spell",
				Type: AbilityTypeSpell,
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "handler is required",
		},
		"cost without resource": {
			ability: Ability{
				Name:         "test-spell",
				Type:         AbilityTypeSpell,
				Handler:      "test-handler",
				ResourceCost: 10,
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "resource_cost requires resource",
		},
		"negative resource cost": {
			ability: Ability{
				Name:         "test-spell",
				Type:         AbilityTypeSpell,
				Handler:      "test-handler",
				Resource:     "test-resource",
				ResourceCost: -5,
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "resource_cost must not be negative",
		},
		"negative ap cost": {
			ability: Ability{
				Name:    "test-spell",
				Type:    AbilityTypeSpell,
				Handler: "test-handler",
				APCost:  -1,
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "ap_cost must not be negative",
		},
		"no messages": {
			ability: Ability{
				Name:    "test-spell",
				Type:    AbilityTypeSpell,
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
			},
			expErr: "at least one message is required",
		},
		"invalid message template": {
			ability: Ability{
				Name:    "test-spell",
				Type:    AbilityTypeSpell,
				Handler: "test-handler",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "{{ .Bad }"},
			},
			expErr: "messages.actor: invalid template:",
		},
		"command validation propagates": {
			ability: Ability{
				Name:    "test-skill",
				Type:    AbilityTypeSkill,
				Handler: "test-handler",
				Command: Command{
					Inputs: []InputSpec{{Type: InputTypeString}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
			expErr: "command: input 0: name is required",
		},
		"resource without cost is valid": {
			ability: Ability{
				Name:     "test-spell",
				Type:     AbilityTypeSpell,
				Handler:  "test-handler",
				Resource: "test-resource",
				Command: Command{
					Inputs:  []InputSpec{{Name: "test-target", Type: InputTypeString}},
					Targets: []TargetSpec{{Name: "test-target", Types: []string{TargetMobile}, Input: "test-target"}},
				},
				Messages: AbilityMessages{Actor: "test-message"},
			},
		},
		"command with no inputs or targets is valid": {
			ability: Ability{
				Name:     "test-skill",
				Type:     AbilityTypeSkill,
				Handler:  "test-handler",
				Messages: AbilityMessages{Actor: "test-message"},
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
