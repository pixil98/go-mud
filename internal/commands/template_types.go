package commands

import (
	"github.com/pixil98/go-mud/internal/game"
)

// Stable template-facing types
// These types decouple templates from internal game structs.
// Templates access .Target.Name instead of .Target.Character.Name

// PlayerRef is the template-facing view of a resolved player.
type PlayerRef struct {
	Name        string
	Description string
}

// PlayerRefFrom creates a PlayerRef from a game.Character.
func PlayerRefFrom(char *game.Character) *PlayerRef {
	if char == nil {
		return nil
	}
	return &PlayerRef{
		Name:        char.Name,
		Description: char.Entity.DetailedDesc,
	}
}

// MobRef is the template-facing view of a resolved mob.
type MobRef struct {
	Name        string
	Description string
}

// ItemRef is the template-facing view of a resolved item.
type ItemRef struct {
	Name        string
	Description string
}

// TargetRef is a polymorphic target reference that could be a player, mob, or item.
type TargetRef struct {
	Type   string     // "player", "mob", or "item"
	Player *PlayerRef // Non-nil if Type == "player"
	Mob    *MobRef    // Non-nil if Type == "mob"
	Item   *ItemRef   // Non-nil if Type == "item"
	Name   string     // Always set - the display name for templates
}

// InputContext is used for Pass 1 expansion (config templates that reference inputs).
type InputContext struct {
	Inputs map[string]any // Parsed input values keyed by input name
}

// RuntimeContext is used for Pass 2 expansion (message templates with full context).
type RuntimeContext struct {
	Actor  *PlayerRef
	Target *PlayerRef
	Text   string
}
