package commands

import (
	"fmt"
	"strings"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// ResolvedPlayer represents a resolved player target.
type ResolvedPlayer struct {
	Character *game.Character   // Target's character data
	State     *game.PlayerState // Target's current state
	Raw       string            // Original input string
}

// ResolvedMob is a placeholder for future mob resolution.
type ResolvedMob struct {
	Raw string
}

// ResolvedItem is a placeholder for future item resolution.
type ResolvedItem struct {
	Raw string
}

// TargetResolver resolves target strings to game entities.
type TargetResolver interface {
	ResolvePlayer(world *game.WorldState, input string) (*ResolvedPlayer, error)
	ResolveMob(world *game.WorldState, input string) (*ResolvedMob, error)
	ResolveItem(world *game.WorldState, input string) (*ResolvedItem, error)
	ResolveTarget(world *game.WorldState, input string) (any, error)
}

// DefaultTargetResolver is the standard implementation of TargetResolver.
type DefaultTargetResolver struct{}

// ResolvePlayer resolves a player target by name (case-insensitive exact match).
// Returns an error if the player doesn't exist or isn't online.
func (r *DefaultTargetResolver) ResolvePlayer(world *game.WorldState, input string) (*ResolvedPlayer, error) {
	inputLower := strings.ToLower(input)

	// Find character with matching name (case-insensitive)
	var matchedChar *game.Character
	var matchedCharId storage.Identifier

	for charId, char := range world.Characters().GetAll() {
		if strings.ToLower(char.Name()) == inputLower {
			matchedChar = char
			matchedCharId = charId
			break
		}
	}

	if matchedChar == nil {
		return nil, NewUserError(fmt.Sprintf("Player '%s' not found", input))
	}

	state := world.GetPlayer(matchedCharId)
	if state == nil {
		return nil, NewUserError(fmt.Sprintf("Player '%s' not found", input))
	}

	return &ResolvedPlayer{
		Character: matchedChar,
		State:     state,
		Raw:       input,
	}, nil
}

// ResolveMob is a stub for future mob resolution.
func (r *DefaultTargetResolver) ResolveMob(world *game.WorldState, input string) (*ResolvedMob, error) {
	return &ResolvedMob{Raw: input}, nil
}

// ResolveItem is a stub for future item resolution.
func (r *DefaultTargetResolver) ResolveItem(world *game.WorldState, input string) (*ResolvedItem, error) {

	// Future: Check inventory
	// Future: Check current room

	return &ResolvedItem{Raw: input}, nil
}

// ResolveTarget resolves a generic target by trying all entity types.
func (r *DefaultTargetResolver) ResolveTarget(world *game.WorldState, input string) (any, error) {
	// Try player
	if resolved, err := r.ResolvePlayer(world, input); err == nil {
		return resolved, nil
	}

	// Future: Try mob
	// Future: Try item

	return nil, NewUserError(fmt.Sprintf("Target '%s' not found", input))
}
