package game

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// MobileInstance represents a single spawned instance of a Mobile definition.
// Location is tracked by the containing structure (room map).
type MobileInstance struct {
	InstanceId string
	Mobile     storage.SmartIdentifier[*assets.Mobile]
	InCombat   bool

	ActorInstance
}

func (mi *MobileInstance) Flags() []string {
	var flags []string
	if mi.InCombat {
		flags = append(flags, "fighting")
	}
	return flags
}

// StatSections returns the mobile's stat display sections.
func (mi *MobileInstance) StatSections() []StatSection {
	mob := mi.Mobile.Get()
	return []StatSection{
		{Lines: []StatLine{
			{Value: mob.ShortDesc, Center: true},
			{Value: fmt.Sprintf("Level %d", mob.Level), Center: true},
		}},
	}
}
